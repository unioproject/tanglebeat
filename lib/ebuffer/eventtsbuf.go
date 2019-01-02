package ebuffer

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
)

//--------------------------------------------
type EventTSExpiringSegment struct {
	ExpiringSegmentBase
	eventTs []uint64
}

type EventTsExpiringBuffer struct {
	ExpiringBuffer
}

const defaulCapacityEventTSExpiringSegment = 50

func NewEventTsExpiringBuffer(segDurationSec, retentionPeriodSec int) *EventTsExpiringBuffer {
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
		ret.SetPrev_(prev)
		return ret
	}
	return &EventTsExpiringBuffer{
		ExpiringBuffer: *NewExpiringBuffer(segDurationSec, retentionPeriodSec, constructor),
	}
}

func (buf *EventTsExpiringBuffer) ForEachEntryRO(callback func(ts uint64) bool) {
	buf.RLock()
	defer buf.RUnlock()
	earliest := utils.UnixMsNow() - buf.retentionPeriodMs
	for s := buf.top; s != nil; s = s.GetPrev_() {
		if s.IsExpired(buf.retentionPeriodMs) {
			return
		}
		seg := s.(*EventTSExpiringSegment)
		seg.ForEachEntryRO(callback, earliest)
	}
}

func (buf *EventTsExpiringBuffer) CountAll() int {
	if buf == nil {
		return 0
	}
	var ret int
	buf.ForEachEntryRO(func(ts uint64) bool {
		ret++
		return true
	})
	return ret
}

func (buf *EventTsExpiringBuffer) RecordTS() {
	buf.NewEntry(utils.UnixMsNow())
}

func NewEventTSExpiringSegment(capacity int) *EventTSExpiringSegment {
	if capacity == 0 {
		capacity = defaulCapacityEventTSExpiringSegment
	}
	return &EventTSExpiringSegment{
		ExpiringSegmentBase: *NewExpiringSegmentBase(),
		eventTs:             make([]uint64, 0, capacity),
	}
}

func (seg *EventTSExpiringSegment) Put(args ...interface{}) {
	seg.Lock()
	defer seg.Unlock()
	seg.eventTs = append(seg.eventTs, utils.UnixMsNow())
}

func (seg *EventTSExpiringSegment) Size() int {
	seg.RLock()
	defer seg.RUnlock()
	return len(seg.eventTs)
}

func (seg *EventTSExpiringSegment) ForEachEntryRO(callback func(ts uint64) bool, earliest uint64) {
	seg.RLock()
	defer seg.RUnlock()
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
