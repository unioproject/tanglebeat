package ebuffer

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
)

// Thread safe expiring buffer
//--------------------------------------------

type eventTSWithDataExpiringSegment struct {
	ExpiringSegmentBase
	eventTs   []uint64
	eventData []interface{}
}

type EventTsWithDataExpiringBuffer struct {
	ExpiringBuffer
}

const defaultCapacityEventTSWithDataExpiringSegment = 50

func NewEventTsWithDataExpiringBuffer(id string, segDurationSec, retentionPeriodSec int) *EventTsWithDataExpiringBuffer {
	if retentionPeriodSec <= segDurationSec {
		retentionPeriodSec = segDurationSec
		retentionPeriodSec += retentionPeriodSec / 10
	}
	constructor := func(prev ExpiringSegment) ExpiringSegment {
		capacity := defaultCapacityEventTSWithDataExpiringSegment
		if prev != nil {
			capacity = prev.Size()
			capacity += capacity / 20 // 5% more
		}
		ret := ExpiringSegment(NewEventTSWithDataExpiringSegment(capacity))
		ret.SetPrev(prev)
		return ret
	}
	return &EventTsWithDataExpiringBuffer{
		ExpiringBuffer: *NewExpiringBuffer(id, segDurationSec, retentionPeriodSec, constructor),
	}
}

func NewEventTSWithDataExpiringSegment(capacity int) *eventTSWithDataExpiringSegment {
	if capacity == 0 {
		capacity = defaultCapacityEventTSWithDataExpiringSegment
	}
	return &eventTSWithDataExpiringSegment{
		ExpiringSegmentBase: *NewExpiringSegmentBase(),
		eventTs:             make([]uint64, 0, capacity),
		eventData:           make([]interface{}, 0, capacity),
	}
}

func (seg *eventTSWithDataExpiringSegment) Put(args ...interface{}) {
	seg.eventTs = append(seg.eventTs, args[0].(uint64))
	seg.eventData = append(seg.eventData, args[0])
}

func (seg *eventTSWithDataExpiringSegment) Size() int {
	return len(seg.eventTs)
}

func (buf *EventTsWithDataExpiringBuffer) ForEachEntry(callback func(ts uint64, data interface{}) bool, lock bool) {
	if lock {
		buf.Lock()
		defer buf.Unlock()
	}
	earliest := utils.UnixMsNow() - buf.retentionPeriodMs
	buf.ForEachSegment(func(s ExpiringSegment) {
		s.(*eventTSWithDataExpiringSegment).forEachEntry(callback, earliest)
	})
}

func (seg *eventTSWithDataExpiringSegment) forEachEntry(callback func(ts uint64, data interface{}) bool, earliest uint64) {
	for idx, ts := range seg.eventTs {
		if ts < earliest {
			return
		}
		if !callback(ts, seg.eventData[idx]) {
			return
		}
	}
	return
}

func (buf *EventTsWithDataExpiringBuffer) RecordTS(data interface{}) {
	buf.Lock()
	defer buf.Unlock()
	buf.NewEntry(utils.UnixMsNow(), data)
}
