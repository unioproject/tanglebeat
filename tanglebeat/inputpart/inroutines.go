package inputpart

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/ebuffer"
	"github.com/lunfardo314/tanglebeat/lib/nanomsg"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/inreaders"
	"math"
	"sort"
	"time"
)

const (
	tlTXCacheSegmentDurationSec = 10
	tlSNCacheSegmentDurationSec = 60
	routineBufferRetentionMin   = 5
)

const (
	inputStreamZMQ     = 0
	inputStreamNanomsg = 1
)

type inputRoutine struct {
	inreaders.InputReaderBase
	initialized            bool
	inputStreamType        int
	uri                    string
	txCount                uint64
	ctxCount               uint64
	lmiCount               int
	lastLmi                int
	obsoleteSnCount        uint64
	lastSeenOnceRate       uint64
	lastSeenSomeMinSNCount uint64
	tsLastTXSomeMin        *ebuffer.EventTsExpiringBuffer
	tsLastSNSomeMin        *ebuffer.EventTsExpiringBuffer
}

func createInputRoutine(uri string, inputStreamType int) {
	ret := &inputRoutine{
		InputReaderBase: *inreaders.NewInputReaderBase(),
		inputStreamType: inputStreamType,
		uri:             uri,
	}
	inputRoutines.AddInputReader(uri, ret)
}

var (
	inputRoutines        *inreaders.InputReaderSet
	compoundOutPublisher *nanomsg.Publisher
)

func MustInitInputRoutines(outEnabled bool, outPort int, inputsZMQ []string, inputsNanomsg []string) {
	initMsgFilter()
	inputRoutines = inreaders.NewInputReaderSet("inreader set")
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

	for _, uri := range inputsZMQ {
		createInputRoutine(uri, inputStreamZMQ)
	}
	for _, uri := range inputsNanomsg {
		createInputRoutine(uri, inputStreamNanomsg)
	}
	startOutValveRoutine()
	startEchoLatencyRoutine()
}

func (r *inputRoutine) GetUri() string {
	r.RLock()
	defer r.RUnlock()
	return r.uri
}

func (r *inputRoutine) checkOnHoldCondition() inreaders.ReasonNotRunning {
	if inputRoutines.NumRunning() < 10 {
		return inreaders.REASON_NORUN_NONE
	}
	r.RLock()
	defer r.RUnlock()
	ret := inreaders.REASON_NORUN_NONE
	// do not put on hold first 5 minutes of run and in case less than 10 readesr left
	if time.Since(r.ReadingSince) > 5*time.Minute {
		if r.lastSeenSomeMinSNCount == 0 {
			// put on hold for 15 min if last 5 min no sn tx came
			infof("Last 5 min no SN message came. Put on hold 15 min: %v", r.uri)
			ret = inreaders.REASON_NORUN_ONHOLD_15MIN
		}
	}
	return ret
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

func (r *inputRoutine) init() {
	uri := r.GetUri()
	tracef("++++++++++++ INIT inputRoutine uri = '%v'", uri)
	r.Lock()
	defer r.Unlock()
	r.tsLastTXSomeMin = ebuffer.NewEventTsExpiringBuffer(
		"tsLastTXSomeMin: "+uri, tlTXCacheSegmentDurationSec, routineBufferRetentionMin*60)
	r.tsLastSNSomeMin = ebuffer.NewEventTsExpiringBuffer(
		"tsLastSNSomeMin: "+uri, tlSNCacheSegmentDurationSec, routineBufferRetentionMin*60)
	r.initialized = true
}

func (r *inputRoutine) uninit() {
	tracef("++++++++++++ UNINIT inputRoutine uri = '%v'", r.GetUri())
	r.Lock()
	defer r.Unlock()
	r.txCount = 0
	r.ctxCount = 0
	r.obsoleteSnCount = 0
	r.tsLastTXSomeMin = nil
	r.tsLastSNSomeMin = nil
	r.initialized = false
}

// TODO dynamically / upon user action add, delete, disable, enable input streams
// TODO nanomsg routines, inputRoutine code reuse

func (r *inputRoutine) Run(name string) inreaders.ReasonNotRunning {
	r.init()
	defer r.uninit()

	uri := r.GetUri()
	var socket inSocket
	var err error

	switch r.inputStreamType {
	case inputStreamZMQ:
		socket, err = NewZmqSocket(uri, topics)
	case inputStreamNanomsg:
		socket, err = NewNanomsgSocket(uri, topics)
	default:
		panic("wrong input stream type")
	}

	if err != nil {
		errorf("Error while starting input channel from %v", uri)
		r.SetLastErr(fmt.Sprintf("%v", err))
		return inreaders.REASON_NORUN_ERROR
	}
	defer socket.Close()

	r.SetReading(true)

	infof("Successfully started input routine for %v", uri)
	for {
		msg, msgSplit, err := socket.RecvMsg()

		if err != nil {
			errorf("%v", err)
			r.SetLastErr(fmt.Sprintf("%v", err))
			return inreaders.REASON_NORUN_ERROR
		}
		r.SetLastHeartbeatNow()

		// send to filter's channel
		if expectedTopic(msgSplit[0]) {
			toFilter(r, msg, msgSplit)
		}
	}
}

func (r *inputRoutine) accountTx() {
	r.Lock()
	defer r.Unlock()
	if !r.initialized {
		return
	}
	r.txCount++
	r.tsLastTXSomeMin.RecordTS()
}

func (r *inputRoutine) accountSn() {
	r.Lock()
	defer r.Unlock()
	if !r.initialized {
		return
	}
	r.ctxCount++
	r.tsLastSNSomeMin.RecordTS()
}

func (r *inputRoutine) accountLmi(index int) {
	r.Lock()
	defer r.Unlock()
	if !r.initialized {
		return
	}
	r.lmiCount++
	r.lastLmi = index
}

func (r *inputRoutine) incObsoleteCount() {
	r.Lock()
	defer r.Unlock()
	if !r.initialized {
		return
	}
	r.obsoleteSnCount++
}

type ZmqRoutineStats struct {
	Uri      string `json:"uri"`
	Id       uint64 `json:"id"`
	Protocol string `json:"protocol"`
	inreaders.InputReaderBaseStats
	TxCount              uint64 `json:"txCount"`
	CtxCount             uint64 `json:"ctxCount"`
	SomeMin              uint64 `json:"someMin"`
	TxCountSomeMin       uint64 `json:"txCountSomeMin"`
	CtxCountSomeMin      uint64 `json:"ctxCountSomeMin"`
	timeIntervalSec10min uint64
	ObsoleteConfirmCount uint64  `json:"obsoleteSNCount"`
	Tps                  float64 `json:"tps"`
	Ctps                 float64 `json:"ctps"`
	Confrate             uint64  `json:"confrate"`
	LmiCount             int     `json:"lmiCount"`
	LastLmi              int     `json:"lastLmi"`
	SeenOnceRate         uint64  `json:"seenOnceRate"`
	State                string  `json:"state"`
	routine              *inputRoutine
}

func (r *inputRoutine) getStats() *ZmqRoutineStats {
	// lock for writing due to seenOnceRate update
	r.Lock()
	defer r.Unlock()

	numLastTX5Min, earliestTx := r.tsLastTXSomeMin.CountAll()
	numLastSN5Min, earliestSn := r.tsLastSNSomeMin.CountAll()
	earliest5Min := earliestTx
	if earliestSn < earliest5Min {
		earliest5Min = earliestSn
	}

	timeIntervalSec := (utils.UnixMsNow() - earliest5Min) / 1000

	var tps, ctps float64
	if timeIntervalSec != 0 {
		tps = float64(numLastTX5Min) / float64(timeIntervalSec)
		tps = math.Round(100*tps) / 100
		ctps = float64(numLastSN5Min) / float64(timeIntervalSec)
		ctps = math.Round(100*ctps) / 100
	}

	confrate := uint64(0)
	if tps != 0 {
		confrate = uint64(100 * ctps / tps)
	}

	r.lastSeenOnceRate = uint64(getSeenOnceRate5to1Min(r.GetId__()))
	r.lastSeenSomeMinSNCount = uint64(numLastSN5Min)

	typ := "zmq"
	if r.inputStreamType == inputStreamNanomsg {
		typ = "nanomsg"
	}
	ret := &ZmqRoutineStats{
		Uri:                  r.uri,
		Id:                   uint64(r.GetId__()),
		Protocol:             typ,
		InputReaderBaseStats: *r.GetReaderBaseStats__(),
		Tps:                  tps,
		TxCount:              r.txCount,
		CtxCount:             r.ctxCount,
		SomeMin:              routineBufferRetentionMin,
		TxCountSomeMin:       uint64(numLastTX5Min),
		CtxCountSomeMin:      uint64(numLastSN5Min),
		timeIntervalSec10min: timeIntervalSec,
		ObsoleteConfirmCount: r.obsoleteSnCount,
		Ctps:                 ctps,
		Confrate:             confrate,
		LmiCount:             r.lmiCount,
		LastLmi:              r.lastLmi,
		SeenOnceRate:         r.lastSeenOnceRate,
	}
	if ret.Running {
		lastHBSec := utils.SinceUnixMs(ret.LastHeartbeatTs) / 1000
		switch {
		case lastHBSec < 60:
			if sncache.firstMilestoneArrived() {
				ret.State = "running"
			} else {
				ret.State = "wait_milestone"
			}
		case lastHBSec < 300:
			ret.State = "slow"
		default:
			ret.State = "inactive"
		}
	} else {
		ret.State = string(r.GetOnHoldInfo__())
	}
	ret.routine = r
	return ret
}

func GetInputStats() []*ZmqRoutineStats {
	ret := make([]*ZmqRoutineStats, 0, 10)
	inputRoutines.ForEach(func(name string, ir inreaders.InputReader) {
		ret = append(ret, ir.(*inputRoutine).getStats())
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
	return a[i].Id < a[j].Id
}
