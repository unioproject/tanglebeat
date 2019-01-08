package ebuffer

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
)

type eventTSWithIntExpiringSegment struct {
	ExpiringSegmentBase
	eventTs  []uint64
	eventInt []int
}

type EventTsWithIntExpiringBuffer struct {
	ExpiringBuffer
}

const defaultCapacityEventTSWithIntExpiringSegment = 50

func NewEventTsWithIntExpiringBuffer(id string, segDurationSec, retentionPeriodSec int) *EventTsWithIntExpiringBuffer {
	if retentionPeriodSec <= segDurationSec {
		retentionPeriodSec = segDurationSec
		retentionPeriodSec += retentionPeriodSec / 10
	}
	constructor := func(prev ExpiringSegment) ExpiringSegment {
		capacity := defaultCapacityEventTSWithIntExpiringSegment
		if prev != nil {
			capacity = prev.Size()
			capacity += capacity / 20 // 5% more
		}
		ret := ExpiringSegment(newEventTSWithIntExpiringSegment(capacity))
		ret.SetPrev(prev)
		return ret
	}
	return &EventTsWithIntExpiringBuffer{
		ExpiringBuffer: *NewExpiringBuffer(id, segDurationSec, retentionPeriodSec, constructor),
	}
}

func newEventTSWithIntExpiringSegment(capacity int) *eventTSWithIntExpiringSegment {
	if capacity == 0 {
		capacity = defaultCapacityEventTSWithDataExpiringSegment
	}
	return &eventTSWithIntExpiringSegment{
		ExpiringSegmentBase: *NewExpiringSegmentBase(),
		eventTs:             make([]uint64, 0, capacity),
		eventInt:            make([]int, 0, capacity),
	}
}

func (seg *eventTSWithIntExpiringSegment) Put(args ...interface{}) {
	seg.eventTs = append(seg.eventTs, args[0].(uint64))
	seg.eventInt = append(seg.eventInt, args[1].(int))
}

func (seg *eventTSWithIntExpiringSegment) Size() int {
	return len(seg.eventTs)
}

func (buf *EventTsWithIntExpiringBuffer) ForEachEntry(callback func(ts uint64, num int) bool, earliest uint64, lock bool) uint64 {
	if lock {
		buf.Lock()
		defer buf.Unlock()
	}
	var retEarliest = utils.UnixMsNow()
	var t uint64
	buf.ForEachSegment__(func(s ExpiringSegment) bool {
		t = s.(*eventTSWithIntExpiringSegment).forEachEntry(callback, earliest)
		if t < retEarliest {
			retEarliest = t
		}
		return true
	})
	return retEarliest
}

func (seg *eventTSWithIntExpiringSegment) forEachEntry(callback func(ts uint64, num int) bool, earliest uint64) uint64 {
	var retEarliest = utils.UnixMsNow()
	for idx, ts := range seg.eventTs {
		if ts >= earliest {
			if ts < retEarliest {
				retEarliest = ts
			}
			if !callback(ts, seg.eventInt[idx]) {
				break
			}
		}
	}
	return retEarliest
}

func (buf *EventTsWithIntExpiringBuffer) RecordInt(num int) {
	buf.Lock()
	defer buf.Unlock()
	buf.NewEntry(utils.UnixMsNow(), num)
}

func (buf *EventTsWithIntExpiringBuffer) ToFloat64(msecAgo uint64) ([]float64, uint64) {
	_, numentries := buf.Size()
	capacity := (int(msecAgo) * numentries) / int(buf.retentionPeriodMs)
	capacity += capacity / 20
	var fBuf = make([]float64, 0, capacity)
	earliest := utils.UnixMsNow() - msecAgo

	retEarliest := buf.ForEachEntry(func(ts uint64, num int) bool {
		fBuf = append(fBuf, float64(num))
		return true
	}, earliest, true)
	return fBuf, retEarliest
}
