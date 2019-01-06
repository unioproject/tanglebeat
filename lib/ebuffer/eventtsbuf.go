package ebuffer

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
)

// Thread safe expiring buffer
//--------------------------------------------

type eventTSExpiringSegment struct {
	ExpiringSegmentBase
	eventTs []uint64
}

type EventTsExpiringBuffer struct {
	ExpiringBuffer
}

const defaulCapacityEventTSExpiringSegment = 50

func NewEventTsExpiringBuffer(id string, segDurationSec, retentionPeriodSec int) *EventTsExpiringBuffer {
	if retentionPeriodSec <= segDurationSec {
		retentionPeriodSec = segDurationSec
		retentionPeriodSec += retentionPeriodSec / 10
	}
	constructor := func(prev ExpiringSegment) ExpiringSegment {
		capacity := defaulCapacityEventTSExpiringSegment
		if prev != nil {
			capacity = prev.Size()
			capacity += capacity / 20 // 5% more
		}
		ret := ExpiringSegment(NewEventTSExpiringSegment(capacity))
		ret.SetPrev(prev)
		return ret
	}
	return &EventTsExpiringBuffer{
		ExpiringBuffer: *NewExpiringBuffer(id, segDurationSec, retentionPeriodSec, constructor),
	}
}

func NewEventTSExpiringSegment(capacity int) *eventTSExpiringSegment {
	if capacity == 0 {
		capacity = defaulCapacityEventTSExpiringSegment
	}

	return &eventTSExpiringSegment{
		ExpiringSegmentBase: *NewExpiringSegmentBase(),
		eventTs:             make([]uint64, 0, capacity),
	}
}

func (seg *eventTSExpiringSegment) Put(args ...interface{}) {
	seg.eventTs = append(seg.eventTs, args[0].(uint64))
}

func (seg *eventTSExpiringSegment) Size() int {
	return len(seg.eventTs)
}

func (buf *EventTsExpiringBuffer) forEachEntry(callback func(ts uint64) bool) {
	earliest := utils.UnixMsNow() - buf.retentionPeriodMs
	buf.ForEachSegment(func(s ExpiringSegment) {
		s.(*eventTSExpiringSegment).forEachEntry(callback, earliest)
	})
}

func (seg *eventTSExpiringSegment) forEachEntry(callback func(ts uint64) bool, earliest uint64) {
	for _, ts := range seg.eventTs {
		if ts < earliest {
			return
		}
		if !callback(ts) {
			return
		}
	}
	return
}

func (buf *EventTsExpiringBuffer) CountAll() int {
	if buf == nil {
		return 0
	}
	buf.Lock()
	defer buf.Unlock()
	var ret int
	buf.forEachEntry(func(ts uint64) bool {
		ret++
		return true
	})
	return ret
}

func (buf *EventTsExpiringBuffer) RecordTS() {
	buf.Lock()
	defer buf.Unlock()
	buf.NewEntry(utils.UnixMsNow())
}
