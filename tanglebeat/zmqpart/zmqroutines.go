package zmqpart

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/ebuffer"
	"github.com/lunfardo314/tanglebeat/lib/nanomsg"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/inreaders"
	"math"
	"sort"
	"strings"
)

const (
	tlTXCacheSegmentDurationSec = 10
	tlSNCacheSegmentDurationSec = 60
	routineBufferRetention10Min = 10
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
		"tsLastTX10Min: "+uri, tlTXCacheSegmentDurationSec, routineBufferRetention10Min*60)
	r.tsLastSN10Min = ebuffer.NewEventTsExpiringBuffer(
		"tsLastSN10Min: "+uri, tlSNCacheSegmentDurationSec, routineBufferRetention10Min*60)
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
}

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
	TxCount              uint64  `json:"txCount"`
	CtxCount             uint64  `json:"ctxCount"`
	ObsoleteConfirmCount uint64  `json:"obsoleteSNCount"`
	Tps                  float64 `json:"tps"`
	Ctps                 float64 `json:"ctps"`
	Confrate             uint64  `json:"confrate"`
	BehindTX             uint64  `json:"behindTX"`
	BehindSN             uint64  `json:"behindSN"`
	LmiCount             int     `json:"lmiCount"`
	LastLmi              int     `json:"lastLmi"`
	NotPropagatedRate    int     `json:"notPropagatedRate"`
}

func (r *zmqRoutine) getStats() *ZmqRoutineStats {
	r.Lock()
	defer r.Unlock()

	numLastTX10Min := r.tsLastTX10Min.CountAll()
	tps := float64(numLastTX10Min) / (routineBufferRetention10Min * 60)
	tps = math.Round(100*tps) / 100

	nopRate := getNotPropagatedById10Min(r.GetId__())
	if numLastTX10Min != 0 {
		nopRate = (100 * nopRate) / numLastTX10Min
	}

	ctps := float64(r.tsLastSN10Min.CountAll()) / (routineBufferRetention10Min * 60)
	ctps = math.Round(100*ctps) / 100

	confrate := uint64(0)
	if tps != 0 {
		confrate = uint64(100 * ctps / tps)
	}

	behindTX := r.last100TXBehindMs.AvgGT(0)
	behindSN := r.last100SNBehindMs.AvgGT(0)

	ret := &ZmqRoutineStats{
		Uri:                  r.uri,
		InputReaderBaseStats: *r.GetReaderBaseStats__(),
		Tps:                  tps,
		TxCount:              r.txCount,
		CtxCount:             r.ctxCount,
		ObsoleteConfirmCount: r.obsoleteSnCount,
		Ctps:                 ctps,
		Confrate:             confrate,
		BehindTX:             behindTX,
		BehindSN:             behindSN,
		LmiCount:             r.lmiCount,
		LastLmi:              r.lastLmi,
		NotPropagatedRate:    nopRate,
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
