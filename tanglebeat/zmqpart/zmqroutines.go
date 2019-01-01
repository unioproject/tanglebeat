package zmqpart

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/bufferwe"
	"github.com/lunfardo314/tanglebeat/lib/nanomsg"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/hashcache"
	"github.com/lunfardo314/tanglebeat/tanglebeat/inreaders"
	"math"
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
	tlTXCacheSegmentDurationSec  = 10
	tlSNCacheSegmentDurationSec  = 60
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
	uri               string
	txCount           uint64
	ctxCount          uint64
	obsoleteSnCount   uint64
	tsLastTX3min      *bufferwe.BufferWE
	tsLastSN3min      *bufferwe.BufferWE
	last100TXBehindMs *utils.RingArray
	last100SNBehindMs *utils.RingArray
}

func createZmqRoutine(uri string) {
	ret := &zmqRoutine{
		InputReaderBase: *inreaders.NewInputReaderBase(),
		uri:             uri,
		tsLastTX3min: bufferwe.NewBufferWE(
			uri+"--tsLastTX3min", false, tlTXCacheSegmentDurationSec, 3*60),
		tsLastSN3min: bufferwe.NewBufferWE(
			uri+"--tsLastSN3min", false, tlSNCacheSegmentDurationSec, 3*60),
		last100TXBehindMs: utils.NewRingArray(100),
		last100SNBehindMs: utils.NewRingArray(100),
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

		//updateVecMetrics(msgSplit[0], r.Uri)

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
		errorf("%v: Message %v is invalid", r.GetUri(), string(msgData))
		return
	}
	seen = txcache.SeenHash(msgSplit[1], nil, &entry)

	if seen {
		behind = utils.SinceUnixMs(entry.FirstSeen)
	} else {
		behind = 0
	}
	r.Lock()
	r.txCount++
	r.tsLastTX3min.Push(utils.UnixMs(time.Now()))
	r.last100TXBehindMs.Push(behind)
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
		errorf("%v: Message %v is invalid", r.GetUri(), string(msgData))
		return
	}
	index, err = strconv.Atoi(msgSplit[1])
	seen = false
	if err != nil {
		errorf("expected index, found %v", msgSplit[1])
		return
	}
	obsolete, _ := sncache.checkCurrentMilestoneIndex(index)
	if obsolete {
		// if index of the current confirmetion message is less than the latest seen,
		// confirmation is ignored.
		// Reason: if it is not too old, it must had been seen from other sources
		r.Lock()
		r.obsoleteSnCount++
		r.Unlock()
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
	r.ctxCount++
	r.tsLastSN3min.Push(utils.UnixMs(time.Now()))
	r.last100SNBehindMs.Push(behind)
	r.Unlock()

	toOutput(msgData, msgSplit, entry.Visits)
}

func (r *zmqRoutine) clearCounters() {
	r.Lock()
	defer r.Unlock()
	r.tsLastTX3min.Reset()
	r.last100TXBehindMs.Reset()
	r.tsLastSN3min.Reset()
	r.last100SNBehindMs.Reset()
}

type ZmqRoutineStats struct {
	inreaders.InputReaderBaseStats
	Uri                  string  `json:"uri"`
	TxCount              uint64  `json:"txCount"`
	CtxCount             uint64  `json:"ctxCount"`
	ObsoleteConfirmCount uint64  `json:"obsoleteSNCount"`
	Tps                  float64 `json:"tps"`
	Ctps                 float64 `json:"ctps"`
	Confrate             uint64  `json:"confrate"`
	BehindTX             uint64  `json:"behindTX"`
	BehindSN             uint64  `json:"behindSN"`
}

func (r *zmqRoutine) getStats() *ZmqRoutineStats {
	r.Lock()
	defer r.Unlock()

	tps := float64(r.tsLastTX3min.CountAll()) / (5 * 60)
	tps = math.Round(100*tps) / 100

	ctps := float64(r.tsLastSN3min.CountAll()) / (5 * 60)
	ctps = math.Round(100*ctps) / 100

	confrate := uint64(0)
	if tps != 0 {
		confrate = uint64(100 * ctps / tps)
	}

	behindTX := r.last100TXBehindMs.AvgGT(0)
	behindSN := r.last100SNBehindMs.AvgGT(0)

	ret := &ZmqRoutineStats{
		InputReaderBaseStats: *r.GetReaderBaseStats__(),
		Uri:                  r.uri,
		Tps:                  tps,
		TxCount:              r.txCount,
		CtxCount:             r.ctxCount,
		ObsoleteConfirmCount: r.obsoleteSnCount,
		Ctps:                 ctps,
		Confrate:             confrate,
		BehindTX:             behindTX,
		BehindSN:             behindSN,
	}
	return ret
}

func GetRoutineStats() map[string]*ZmqRoutineStats {
	ret := make(map[string]*ZmqRoutineStats)
	zmqRoutines.ForEach(func(name string, ir inreaders.InputReader) {
		ret[name] = ir.(*zmqRoutine).getStats()
	})
	return ret
}
