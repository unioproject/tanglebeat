package ebuffer

import (
	"github.com/unioproject/tanglebeat/lib/utils"
)

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
		ret := ExpiringSegment(newEventTSWithDataExpiringSegment(capacity))
		ret.SetPrev(prev)
		return ret
	}
	return &EventTsWithDataExpiringBuffer{
		ExpiringBuffer: *NewExpiringBuffer(id, segDurationSec, retentionPeriodSec, constructor),
	}
}

func newEventTSWithDataExpiringSegment(capacity int) *eventTSWithDataExpiringSegment {
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
	seg.eventData = append(seg.eventData, args[1])
}

func (seg *eventTSWithDataExpiringSegment) Size() int {
	return len(seg.eventTs)
}

func (buf *EventTsWithDataExpiringBuffer) ForEachEntry(callback func(ts uint64, data interface{}) bool, earliest uint64, lock bool) uint64 {
	if lock {
		buf.Lock()
		defer buf.Unlock()
	}
	var retEarliest = utils.UnixMsNow()
	var t uint64
	buf.ForEachSegment__(func(s ExpiringSegment) bool {
		t = s.(*eventTSWithDataExpiringSegment).forEachEntry(callback, earliest)
		if t < retEarliest {
			retEarliest = t
		}
		return true
	})
	return retEarliest

}

func (seg *eventTSWithDataExpiringSegment) forEachEntry(callback func(ts uint64, data interface{}) bool, earliest uint64) uint64 {
	var retEarliest = utils.UnixMsNow()
	for idx, ts := range seg.eventTs {
		if ts >= earliest {
			if ts < retEarliest {
				retEarliest = ts
			}
			if !callback(ts, seg.eventData[idx]) {
				break
			}
		}
	}
	return retEarliest
}

func (buf *EventTsWithDataExpiringBuffer) RecordTS(data interface{}) {
	buf.Lock()
	defer buf.Unlock()
	buf.NewEntry(utils.UnixMsNow(), data)
}
