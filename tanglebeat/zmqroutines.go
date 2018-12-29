package main

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/nanomsg"
	"github.com/lunfardo314/tanglebeat/lib/utils"
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
	txcache          *hashCacheBase
	sncache          *hashCacheSN
	valueTxCache     *hashCacheBase
	valueBundleCache *hashCacheBase
)

func init() {
	txcache = newHashCacheBase(useFirstHashTrytes, segmentDurationTXMs, retentionPeriodMs)
	sncache = newHashCacheSN(useFirstHashTrytes, segmentDurationSNMs, retentionPeriodMs)
	valueTxCache = newHashCacheBase(useFirstHashTrytes, segmentDurationValueTXMs, retentionPeriodMs)
	valueBundleCache = newHashCacheBase(useFirstHashTrytes, segmentDurationValueBundleMs, retentionPeriodMs)
	startCollectingLatencyMetrics(txcache, sncache)
}

type zmqRoutine struct {
	inreaders.InputReaderBase
	uri      string
	lastTX   time.Time
	lastSN   time.Time
	behindTX *ringArray
	behindSN *ringArray
	totalTX  uint64
	totalSN  uint64
}

const ringArrayLen = 100

func createZmqRoutine(uri string) {
	ret := &zmqRoutine{
		InputReaderBase: *inreaders.NewInoutReaderBase("ZMQ--" + uri),
		uri:             uri,
		behindTX:        newRingArray(ringArrayLen),
		behindSN:        newRingArray(ringArrayLen),
	}
	zmqRoutines.AddInputReader(ret)
}

var (
	zmqRoutines          *inreaders.InputReaderSet
	compoundOutPublisher *nanomsg.Publisher
)

func mustInitZmqRoutines() {
	zmqRoutines = inreaders.NewInputReaderSet()
	var err error
	compoundOutPublisher, err = nanomsg.NewPublisher(Config.IriMsgStream.OutputPort, 0, nil)
	if err != nil {
		errorf("Failed to create publishing channel. Publisher is disabled: %v", err)
		panic(err)
	}
	infof("Publisher for zmq compound out stream initialized successfully on port %v",
		Config.IriMsgStream.OutputPort)

	for _, uri := range Config.IriMsgStream.Inputs {
		createZmqRoutine(uri)
	}
}

func (r *zmqRoutine) GetUri() string {
	r.Lock()
	defer r.Unlock()
	return r.uri
}

var topics = []string{"tx", "sn"}

func (r *zmqRoutine) Run() {
	uri := r.GetUri()
	infof("Starting zmq routine for %v", uri)
	defer infof("Leaving zmq routine for %v", uri)

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
	var entry cacheEntry

	if len(msgSplit) < 2 {
		errorf("%v: Message %v is invalid", r.uri, string(msgData))
		return
	}
	seen = txcache.seenHash(msgSplit[1], nil, &entry)

	if seen {
		behind = utils.SinceUnixMs(entry.firstSeen)
	} else {
		behind = 0
	}
	r.Lock()
	r.lastTX = time.Now()
	r.behindTX.push(behind)
	r.totalTX++
	r.Unlock()

	toOutput(msgData, msgSplit, entry.visits)
}

func (r *zmqRoutine) processSNMsg(msgData []byte, msgSplit []string) {
	var hash string
	var seen bool
	var behind uint64
	var index int
	var err error
	var entry cacheEntry

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

	seen = sncache.seenHash(hash, nil, &entry)
	if seen {
		behind = utils.SinceUnixMs(entry.firstSeen)
	} else {
		behind = 0
	}

	r.Lock()
	r.lastSN = time.Now()
	r.behindSN.push(behind)
	r.totalSN++
	r.Unlock()

	toOutput(msgData, msgSplit, entry.visits)
}

func (r *zmqRoutine) clearCounters() {
	r.Lock()
	defer r.Unlock()
	r.lastTX = time.Time{}
	r.lastSN = time.Time{}
	r.totalTX = 0
	r.totalSN = 0
}

type zmqRoutineStats struct {
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

func (r *zmqRoutine) getStats() *zmqRoutineStats {
	r.Lock()
	defer r.Unlock()
	var errs string
	running, reading, readingSince := r.GetState()
	if running && reading {
		errs = ""
	} else {
		errs = r.GetLastErr()
	}
	var tmpVal float32
	if r.totalTX != 0 {
		tmpVal = 100 * float32(r.totalSN) / float32(r.totalTX)
	} else {
		tmpVal = -1
	}
	return &zmqRoutineStats{
		Running:           running && reading,
		TotalTX:           r.totalTX,
		TotalSN:           r.totalSN,
		SNbyTxPerc:        tmpVal,
		AvgBehindTXSec:    float32(r.behindTX.sum()) / (1000 * float32(r.behindTX.len())),
		AvgBehindSNSec:    float32(r.behindSN.sum()) / (1000 * float32(r.behindSN.len())),
		LeaderTXPerc:      uint64(float32(r.behindTX.numzeros()) * 100 / float32(r.behindTX.len())),
		LeaderSNPerc:      uint64(float32(r.behindSN.numzeros()) * 100 / float32(r.behindSN.len())),
		LastTXMsecAgo:     utils.SinceUnixMs(utils.UnixMs(r.lastTX)),
		LastSNMsecAgo:     utils.SinceUnixMs(utils.UnixMs(r.lastSN)),
		RunningAlreadyMin: float32(utils.SinceUnixMs(utils.UnixMs(readingSince))) / 60000,
		LastErr:           errs,
	}
}

func getRoutineStats() map[string]*zmqRoutineStats {
	ret := make(map[string]*zmqRoutineStats)
	zmqRoutines.ForEach(func(name string, ir inreaders.InputReader) {
		ret[name] = ir.(*zmqRoutine).getStats()
	})
	return ret
}
