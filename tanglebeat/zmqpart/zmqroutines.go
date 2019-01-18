package zmqpart

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/ebuffer"
	"github.com/lunfardo314/tanglebeat/lib/nanomsg"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/hashcache"
	"github.com/lunfardo314/tanglebeat/tanglebeat/inreaders"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	tlTXCacheSegmentDurationSec        = 10
	tlSNCacheSegmentDurationSec        = 60
	routineBufferRetentionMin          = 5
	quarantineBufferSegmentDurationSec = 15
	quarantineBufferRetentionMin       = 3
	quarantineHashLen                  = 12
	quarantineSeenOnceRateThreashold   = 50
	unquarantineSeenOnceRateThreashold = 40
)

type zmqRoutine struct {
	inreaders.InputReaderBase
	uri               string
	txCount           uint64
	ctxCount          uint64
	lmiCount          int
	lastLmi           int
	obsoleteSnCount   uint64
	tsLastTX10Min     *ebuffer.EventTsExpiringBuffer
	tsLastSN10Min     *ebuffer.EventTsExpiringBuffer
	last100TXBehindMs *utils.RingArray
	last100SNBehindMs *utils.RingArray
	quarantined       bool
	quarantinedTx     *ebuffer.EventTsWithDataExpiringBuffer
}

func createZmqRoutine(uri string) {
	ret := &zmqRoutine{
		InputReaderBase: *inreaders.NewInputReaderBase(),
		uri:             uri,
	}
	zmqRoutines.AddInputReader(uri, ret)
}

var (
	zmqRoutines          *inreaders.InputReaderSet
	compoundOutPublisher *nanomsg.Publisher
)

func MustInitZmqRoutines(outEnabled bool, outPort int, inputs []string) {
	initMsgFilter()
	zmqRoutines = inreaders.NewInputReaderSet("zmq routine set")
	var err error
	compoundOutPublisher, err = nanomsg.NewPublisher(outEnabled, outPort, 0, localLog)
	if err != nil {
		errorf("Failed to create publishing channel. Publisher is disabled: %v", err)
		panic(err)
	}
	if outEnabled {
		infof("Publisher for zmq compound output stream initialized successfully on port %v", outPort)
	} else {
		infof("Publisher for zmq compound output stream is DISABLED")
	}

	for _, uri := range inputs {
		createZmqRoutine(uri)
	}
}

func (r *zmqRoutine) GetUri() string {
	r.Lock()
	defer r.Unlock()
	return r.uri
}

var topics = []string{"tx", "sn", "lmi"}

func expectedTopic(topic string) bool {
	for _, t := range topics {
		if t == topic {
			return true
		}
	}
	return false
}

func (r *zmqRoutine) init() {
	uri := r.GetUri()
	tracef("++++++++++++ INIT zmqRoutine uri = '%v'", uri)
	r.Lock()
	defer r.Unlock()
	r.tsLastTX10Min = ebuffer.NewEventTsExpiringBuffer(
		"tsLastTX10Min: "+uri, tlTXCacheSegmentDurationSec, routineBufferRetentionMin*60)
	r.tsLastSN10Min = ebuffer.NewEventTsExpiringBuffer(
		"tsLastSN10Min: "+uri, tlSNCacheSegmentDurationSec, routineBufferRetentionMin*60)
	r.last100TXBehindMs = utils.NewRingArray(100)
	r.last100SNBehindMs = utils.NewRingArray(100)
}

func (r *zmqRoutine) uninit() {
	tracef("++++++++++++ UNINIT zmqRoutine uri = '%v'", r.GetUri())
	r.Lock()
	defer r.Unlock()
	r.tsLastTX10Min = nil
	r.last100TXBehindMs = nil
	r.tsLastSN10Min = nil
	r.last100SNBehindMs = nil
	r.quarantinedTx = nil
}

// TODO dynamically / upon user action add, delete, disable, enable input streams

func (r *zmqRoutine) Run(name string) {
	r.init()
	defer r.uninit()

	uri := r.GetUri()

	socket, err := utils.OpenSocketAndSubscribe(uri, topics)
	if err != nil {
		errorf("Error while starting zmq channel for %v", uri)
		r.SetLastErr(fmt.Sprintf("%v", err))
		return
	}
	r.SetReading(true)
	cancelQuarantineRoutine := r.startQuaratineRoutine()
	defer cancelQuarantineRoutine()

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

		// send to filter's channel
		if expectedTopic(msgSplit[0]) {
			toFilter(r, msg.Frames[0], msgSplit)
		}
	}
}

func (r *zmqRoutine) processMsg(msgData []byte, msgSplit []string) {
	if r.isQuarantined() {
		r.toQuarantine(msgData, msgSplit)
	} else {
		toFilter(r, msgData, msgSplit)
	}
}

func (r *zmqRoutine) setQuarantined(quarantined bool) {
	r.Lock()
	defer r.Unlock()
	if r.quarantined == quarantined {
		return // no action
	}
	if quarantined {
		r.quarantinedTx = ebuffer.NewEventTsWithDataExpiringBuffer(
			"quarantineBuffer-"+r.uri, quarantineBufferSegmentDurationSec, quarantineBufferRetentionMin*60)
	} else {
		r.quarantinedTx = nil // release, not needed
	}
	r.quarantined = quarantined
}

func (r *zmqRoutine) startQuaratineRoutine() func() {
	uri := r.GetUri()
	infof("Started quarantine routine for %v", uri)
	chCancel := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-chCancel:
				return
			case <-time.After(30 * time.Second):
			}
			if r.isQuarantined() {
				if r.toBeUnquarantined() {
					r.setQuarantined(false)
					infof("++++++ Unquarantined %v", r.GetUri())
				}
			} else {
				if r.toBeQuarantined() {
					infof("++++++ Quarantined %v", r.GetUri())
					r.setQuarantined(true)
				}
			}
		}
	}()
	return func() {
		close(chCancel)
		wg.Wait()
		infof("Stopped quarantine routine for %v", uri)
	}
}

func (r *zmqRoutine) isQuarantined() bool {
	r.RLock()
	defer r.RUnlock()
	return r.quarantined
}

func (r *zmqRoutine) toBeQuarantined() bool {
	if r.isQuarantined() {
		return false
	}
	stats := r.getStats()
	return stats.timeIntervalSec10min >= 5*60 && stats.SeenOnceRate > quarantineSeenOnceRateThreashold
}

func (r *zmqRoutine) toBeUnquarantined() bool {
	if !r.isQuarantined() {
		return false
	}
	_, numEntries := r.quarantinedTx.Size()
	if numEntries == 0 {
		return true // ?????
	}
	numSeenOnce := r.calcSeenOnceRateQuarantined()
	seenOncePerc := (numSeenOnce * 100) / numEntries
	return seenOncePerc < unquarantineSeenOnceRateThreashold
}

func (r *zmqRoutine) calcSeenOnceRateQuarantined() int {
	if !r.isQuarantined() {
		return 0
	}
	var hash string
	var ret int
	var entry hashcache.CacheEntry
	r.quarantinedTx.ForEachEntry(func(ts uint64, data interface{}) bool {
		hash = data.(string)
		if txcache.FindNoTouch(hash, &entry) {
			ret++
		}
		return true
	}, 0, true)
	return ret
}

func shortHash(hash string) string {
	ret := make([]byte, quarantineHashLen)
	copy(ret, hash[:quarantineHashLen])
	return string(ret)
}

func (r *zmqRoutine) toQuarantine(msgData []byte, msgSplit []string) {
	r.Lock()
	defer r.Unlock()
	if !r.quarantined {
		return
	}
	if msgSplit[0] != "tx" || len(msgSplit) < 2 {
		return
	}
	r.quarantinedTx.RecordTS(shortHash(msgSplit[1]))
}

func (r *zmqRoutine) accountTx(behind uint64) {
	r.Lock()
	defer r.Unlock()
	r.txCount++
	r.tsLastTX10Min.RecordTS()
	r.last100TXBehindMs.Push(behind)
}

func (r *zmqRoutine) accountSn(behind uint64) {
	r.Lock()
	defer r.Unlock()
	r.ctxCount++
	r.tsLastSN10Min.RecordTS()
	r.last100SNBehindMs.Push(behind)
}

func (r *zmqRoutine) accountLmi(index int) {
	r.Lock()
	defer r.Unlock()
	r.lmiCount++
	r.lastLmi = index
}

func (r *zmqRoutine) incObsoleteCount() {
	r.Lock()
	defer r.Unlock()
	r.obsoleteSnCount++
}

type ZmqRoutineStats struct {
	Uri string `json:"uri"`
	inreaders.InputReaderBaseStats
	TxCount              uint64 `json:"txCount"`
	CtxCount             uint64 `json:"ctxCount"`
	TxCount10min         uint64 `json:"txCount10min"`
	CtxCount10min        uint64 `json:"ctxCount10min"`
	timeIntervalSec10min uint64
	ObsoleteConfirmCount uint64  `json:"obsoleteSNCount"`
	Tps                  float64 `json:"tps"`
	Ctps                 float64 `json:"ctps"`
	Confrate             uint64  `json:"confrate"`
	BehindTX             uint64  `json:"behindTX"`
	BehindSN             uint64  `json:"behindSN"`
	LmiCount             int     `json:"lmiCount"`
	LastLmi              int     `json:"lastLmi"`
	SeenOnceCount10Min   int     `json:"seenOnceCount10Min"`
	SeenOnceRate         uint64  `json:"seenOnceRate"`
	Quarantined          bool    `json:"quarantined"`
}

func (r *zmqRoutine) getStats() *ZmqRoutineStats {
	r.RLock()
	defer r.RUnlock()

	numLastTX10Min, earliestTx := r.tsLastTX10Min.CountAll()
	numLastSN10Min, earliestSn := r.tsLastSN10Min.CountAll()
	earliest10Min := earliestTx
	if earliestSn < earliest10Min {
		earliest10Min = earliestSn
	}

	timeIntervalSec := (utils.UnixMsNow() - earliest10Min) / 1000

	var tps, ctps float64
	if timeIntervalSec != 0 {
		tps = float64(numLastTX10Min) / float64(timeIntervalSec)
		tps = math.Round(100*tps) / 100
		ctps = float64(numLastSN10Min) / float64(timeIntervalSec)
		ctps = math.Round(100*ctps) / 100
	}

	confrate := uint64(0)
	if tps != 0 {
		confrate = uint64(100 * ctps / tps)
	}

	behindTX := r.last100TXBehindMs.AvgGT(0)
	behindSN := r.last100SNBehindMs.AvgGT(0)

	sor := uint64(0)
	sc := getSeenOnceCount10Min(r.GetId__())
	if numLastTX10Min != 0 {
		sor = (uint64(sc) * 100) / uint64(numLastTX10Min)
	}

	ret := &ZmqRoutineStats{
		Uri:                  r.uri,
		InputReaderBaseStats: *r.GetReaderBaseStats__(),
		Tps:                  tps,
		TxCount:              r.txCount,
		CtxCount:             r.ctxCount,
		TxCount10min:         uint64(numLastTX10Min),
		CtxCount10min:        uint64(numLastSN10Min),
		timeIntervalSec10min: timeIntervalSec,
		ObsoleteConfirmCount: r.obsoleteSnCount,
		Ctps:                 ctps,
		Confrate:             confrate,
		BehindTX:             behindTX,
		BehindSN:             behindSN,
		LmiCount:             r.lmiCount,
		LastLmi:              r.lastLmi,
		SeenOnceCount10Min:   sc,
		SeenOnceRate:         sor,
		Quarantined:          r.quarantined,
	}
	return ret
}

func GetInputStats() []*ZmqRoutineStats {
	ret := make([]*ZmqRoutineStats, 0, 10)
	zmqRoutines.ForEach(func(name string, ir inreaders.InputReader) {
		ret = append(ret, ir.(*zmqRoutine).getStats())
	})
	sort.Sort(ZmqRoutineStatsSlice(ret))
	return ret
}

type ZmqRoutineStatsSlice []*ZmqRoutineStats

func (a ZmqRoutineStatsSlice) Len() int {
	return len(a)
}

func (a ZmqRoutineStatsSlice) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a ZmqRoutineStatsSlice) Less(i, j int) bool {
	return a[i].Uri < a[j].Uri
}
