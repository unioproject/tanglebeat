package zmqpart

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/nanomsg"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/hashcache"
	"github.com/lunfardo314/tanglebeat/tanglebeat/inreaders"
	"strconv"
	"strings"
	"time"
)

const (
	useFirstHashTrytes           = 18 // first positions of the hash will only be used in hash table. To spare memory
	segmentDurationTXMs          = uint64((1 * time.Minute) / time.Millisecond)
	segmentDurationValueTXMs     = uint64((10 * time.Minute) / time.Millisecond)
	segmentDurationValueBundleMs = uint64((10 * time.Minute) / time.Millisecond)
	segmentDurationSNMs          = uint64((1 * time.Minute) / time.Millisecond)
	retentionPeriodMs            = uint64((1 * time.Hour) / time.Millisecond)
)

var (
	txcache          *hashcache.HashCacheBase
	sncache          *hashCacheSN
	valueTxCache     *hashcache.HashCacheBase
	valueBundleCache *hashcache.HashCacheBase
)

func init() {
	txcache = hashcache.NewHashCacheBase(useFirstHashTrytes, segmentDurationTXMs, retentionPeriodMs)
	sncache = newHashCacheSN(useFirstHashTrytes, segmentDurationSNMs, retentionPeriodMs)
	valueTxCache = hashcache.NewHashCacheBase(useFirstHashTrytes, segmentDurationValueTXMs, retentionPeriodMs)
	valueBundleCache = hashcache.NewHashCacheBase(useFirstHashTrytes, segmentDurationValueBundleMs, retentionPeriodMs)
	startCollectingLatencyMetrics(txcache, sncache)
}

type zmqRoutine struct {
	inreaders.InputReaderBase
	uri      string
	lastTX   time.Time
	lastSN   time.Time
	behindTX *utils.RingArray
	behindSN *utils.RingArray
	totalTX  uint64
	totalSN  uint64
}

const ringArrayLen = 100

func createZmqRoutine(uri string) {
	ret := &zmqRoutine{
		InputReaderBase: *inreaders.NewInputReaderBase(),
		uri:             uri,
		behindTX:        utils.NewRingArray(ringArrayLen),
		behindSN:        utils.NewRingArray(ringArrayLen),
	}
	zmqRoutines.AddInputReader("ZMQ--"+uri, ret)
}

var (
	zmqRoutines          *inreaders.InputReaderSet
	compoundOutPublisher *nanomsg.Publisher
)

func MustInitZmqRoutines(outPort int, inputs []string) {
	zmqRoutines = inreaders.NewInputReaderSet("zmq routine set")
	var err error
	compoundOutPublisher, err = nanomsg.NewPublisher(outPort, 0, nil)
	if err != nil {
		errorf("Failed to create publishing channel. Publisher is disabled: %v", err)
		panic(err)
	}
	infof("Publisher for zmq compound out stream initialized successfully on port %v", outPort)

	for _, uri := range inputs {
		createZmqRoutine(uri)
	}
}

func (r *zmqRoutine) GetUri() string {
	r.Lock()
	defer r.Unlock()
	return r.uri
}

var topics = []string{"tx", "sn"}

func (r *zmqRoutine) Run(name string) {
	uri := r.GetUri()
	infof("Starting zmq routine '%v' for %v", name, uri)
	defer errorf("Leaving zmq routine '%v' for %v", name, uri)

	r.clearCounters()
	socket, err := utils.OpenSocketAndSubscribe(uri, topics)
	if err != nil {
		errorf("Error while starting zmq channel for %v", uri)
		r.SetLastErr(fmt.Sprintf("%v", err))
		return
	}
	r.SetReading(true)

	infof("Successfully started zmq routine and channel for %v", uri)
	for {
		msg, err := socket.Recv()
		if err != nil {
			errorf("reading ZMQ socket for '%v': socket.Recv() returned %v", uri, err)
			r.SetLastErr(fmt.Sprintf("%v", err))
			return // exit routine
		}
		if len(msg.Frames) == 0 {
			errorf("+++++++++ empty zmq msgSplit for '%v': %+v", uri, msg)
			r.SetLastErr(fmt.Sprintf("empty msgSplit from zmq"))
			return // exit routine
		}
		r.SetLastHeartbeatNow()
		msgSplit := strings.Split(string(msg.Frames[0]), " ")

		//updateVecMetrics(msgSplit[0], r.uri)

		switch msgSplit[0] {
		case "tx":
			r.processTXMsg(msg.Frames[0], msgSplit)
		case "sn":
			r.processSNMsg(msg.Frames[0], msgSplit)
		}
	}
}

func (r *zmqRoutine) processTXMsg(msgData []byte, msgSplit []string) {
	var seen bool
	var behind uint64
	var entry hashcache.CacheEntry

	if len(msgSplit) < 2 {
		errorf("%v: Message %v is invalid", r.uri, string(msgData))
		return
	}
	seen = txcache.SeenHash(msgSplit[1], nil, &entry)

	if seen {
		behind = utils.SinceUnixMs(entry.FirstSeen)
	} else {
		behind = 0
	}
	r.Lock()
	r.lastTX = time.Now()
	r.behindTX.Push(behind)
	r.totalTX++
	r.Unlock()

	toOutput(msgData, msgSplit, entry.Visits)
}

func (r *zmqRoutine) processSNMsg(msgData []byte, msgSplit []string) {
	var hash string
	var seen bool
	var behind uint64
	var index int
	var err error
	var entry hashcache.CacheEntry

	if len(msgSplit) < 3 {
		errorf("%v: Message %v is invalid", r.uri, string(msgData))
		return
	}
	index, err = strconv.Atoi(msgSplit[1])
	seen = false
	if err != nil {
		errorf("expected index, found %v", msgSplit[1])
		return
	}
	obsolete, whenSeen := sncache.obsoleteIndex(index)
	if obsolete {
		debugf("%v: obsolete 'sn' message: %v. Last index since %v sec ago",
			r.uri, string(msgData), utils.SinceUnixMs(whenSeen)/1000)
		return
	}
	hash = msgSplit[2]

	seen = sncache.SeenHash(hash, nil, &entry)
	if seen {
		behind = utils.SinceUnixMs(entry.FirstSeen)
	} else {
		behind = 0
	}

	r.Lock()
	r.lastSN = time.Now()
	r.behindSN.Push(behind)
	r.totalSN++
	r.Unlock()

	toOutput(msgData, msgSplit, entry.Visits)
}

func (r *zmqRoutine) clearCounters() {
	r.Lock()
	defer r.Unlock()
	r.lastTX = time.Time{}
	r.lastSN = time.Time{}
	r.totalTX = 0
	r.totalSN = 0
}

type ZmqRoutineStats struct {
	Uri               string  `json:"uri"`
	Running           bool    `json:"running"`
	TotalTX           uint64  `json:"totalTX"`
	TotalSN           uint64  `json:"totalSN"`
	SNbyTxPerc        float32 `json:"SNbyTX"`
	AvgBehindTXSec    float32 `json:"avgBehindTXSec"`
	AvgBehindSNSec    float32 `json:"avgBehindSNSec"`
	LeaderTXPerc      uint64  `json:"leaderTXPerc"`
	LeaderSNPerc      uint64  `json:"leaderSNPerc"`
	LastTXMsecAgo     uint64  `json:"lastTXMsecAgo"`
	LastSNMsecAgo     uint64  `json:"lastSNMsecAgo"`
	RunningAlreadyMin float32 `json:"runningAlreadyMin"`
	LastErr           string  `json:"lastErr"`
}

func (r *zmqRoutine) getStats() *ZmqRoutineStats {
	r.Lock()
	defer r.Unlock()

	var errs string
	running, reading, readingSince := r.GetState__()
	if running && reading {
		errs = ""
	} else {
		errs = r.GetLastErr__()
	}
	var tmpVal float32
	if r.totalTX != 0 {
		tmpVal = 100 * float32(r.totalSN) / float32(r.totalTX)
	} else {
		tmpVal = -1
	}
	return &ZmqRoutineStats{
		Uri:               r.uri,
		Running:           running && reading,
		TotalTX:           r.totalTX,
		TotalSN:           r.totalSN,
		SNbyTxPerc:        tmpVal,
		AvgBehindTXSec:    float32(r.behindTX.Sum()) / (1000 * float32(r.behindTX.Len())),
		AvgBehindSNSec:    float32(r.behindSN.Sum()) / (1000 * float32(r.behindSN.Len())),
		LeaderTXPerc:      uint64(float32(r.behindTX.Numzeros()) * 100 / float32(r.behindTX.Len())),
		LeaderSNPerc:      uint64(float32(r.behindSN.Numzeros()) * 100 / float32(r.behindSN.Len())),
		LastTXMsecAgo:     utils.SinceUnixMs(utils.UnixMs(r.lastTX)),
		LastSNMsecAgo:     utils.SinceUnixMs(utils.UnixMs(r.lastSN)),
		RunningAlreadyMin: float32(utils.SinceUnixMs(utils.UnixMs(readingSince))) / 60000,
		LastErr:           errs,
	}
}

func GetRoutineStats() map[string]*ZmqRoutineStats {
	ret := make(map[string]*ZmqRoutineStats)
	zmqRoutines.ForEach(func(name string, ir inreaders.InputReader) {
		ret[name] = ir.(*zmqRoutine).getStats()
	})
	return ret
}
