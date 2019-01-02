package ebuffer

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"sync"
	"time"
)

//--------------------------------------------
type ExpiringSegment interface {
	IsExpired(retentionPeriodMs uint64) bool
	IsOpen(segDurationMs uint64) bool
	GetPrev_() ExpiringSegment
	SetPrev_(ExpiringSegment)
	Put(data ...interface{})
	Touch_()
	Size() int
}

//-------------------------------
// ExpiringBuffer is linked list of ExpiringSegments and purge loop in the background
type ExpiringBuffer struct {
	segDurationMs     uint64
	retentionPeriodMs uint64
	constructor       func(prev ExpiringSegment) ExpiringSegment
	top               ExpiringSegment
	mutex             *sync.RWMutex
}

func NewExpiringBuffer(segDurationSec, retentionPeriodSec int, constructor func(prev ExpiringSegment) ExpiringSegment) *ExpiringBuffer {
	return &ExpiringBuffer{
		segDurationMs:     uint64(segDurationSec * 1000),
		retentionPeriodMs: uint64(retentionPeriodSec * 1000),
		constructor:       constructor,
		mutex:             &sync.RWMutex{},
	}
}

func (buf *ExpiringBuffer) isEmpty() bool {
	buf.mutex.RLock()
	defer buf.mutex.RUnlock()
	return buf.top == nil
}

const purgeLoopSleepSec = 5

func (buf *ExpiringBuffer) purgeLoop() {
	tracef("Purge loop started")
	defer tracef("Purge loop stopped")

	for {
		if buf.isEmpty() {
			return
		}
		if buf.top.IsExpired(buf.retentionPeriodMs) {
			tracef("purged top segment size = %v", buf.top.Size())
			buf.top = nil
			return
		}
		buf.mutex.Lock()
		for s := buf.top; ; {
			if s == nil {
				break
			}
			prev := s.GetPrev_()
			if prev == nil {
				break
			}
			if prev.IsExpired(buf.retentionPeriodMs) {
				tracef("purged size = %v", prev.Size())
				s.SetPrev_(nil)
				break
			}
			s = prev
		}
		buf.mutex.Unlock()
		time.Sleep(time.Duration(purgeLoopSleepSec) * time.Second)
	}
}

func (buf *ExpiringBuffer) NewEntry(data ...interface{}) {
	empty := buf.isEmpty()
	if empty || !buf.top.IsOpen(buf.segDurationMs) {
		buf.mutex.Lock()
		buf.top = buf.constructor(buf.top)
		buf.mutex.Unlock()
		if empty {
			go buf.purgeLoop()
		}
	}
	buf.top.Put(data...)
	buf.top.Touch_()
}

func (buf *ExpiringBuffer) Size() (int, int) {
	buf.mutex.RLock()
	defer buf.mutex.RUnlock()

	var numseg, numentries int
	for s := buf.top; s != nil; s = s.GetPrev_() {
		numseg++
		numentries += s.Size()
	}
	return numseg, numentries
}

//---------------------------------
type ExpiringSegmentBase struct {
	created   uint64
	lastTouch uint64
	prev      ExpiringSegment
	mutex     *sync.RWMutex
}

func NewExpiringSegmentBase() *ExpiringSegmentBase {
	nowis := utils.UnixMsNow()
	return &ExpiringSegmentBase{
		created:   nowis,
		lastTouch: nowis,
		mutex:     &sync.RWMutex{},
	}
}

func (seg *ExpiringSegmentBase) IsExpired(retentionPeriodMs uint64) bool {
	seg.RLock()
	defer seg.RUnlock()
	return utils.UnixMsNow()-seg.lastTouch >= retentionPeriodMs
}

func (seg *ExpiringSegmentBase) IsOpen(segDurationMs uint64) bool {
	seg.RLock()
	defer seg.RUnlock()
	return utils.UnixMsNow()-seg.created < segDurationMs
}

func (seg *ExpiringSegmentBase) GetPrev_() ExpiringSegment {
	return seg.prev
}

func (seg *ExpiringSegmentBase) SetPrev_(prev ExpiringSegment) {
	seg.prev = prev
}

func (seg *ExpiringSegmentBase) Lock() {
	seg.mutex.Lock()
}

func (seg *ExpiringSegmentBase) Unlock() {
	seg.mutex.Unlock()
}

func (seg *ExpiringSegmentBase) RLock() {
	seg.mutex.RLock()
}

func (seg *ExpiringSegmentBase) RUnlock() {
	seg.mutex.RUnlock()
}

func (seg *ExpiringSegmentBase) Touch_() {
	seg.lastTouch = utils.UnixMsNow()
}

func (seg *ExpiringSegmentBase) IsOpen_(segDurationMs uint64) bool {
	return utils.UnixMsNow()-seg.created < segDurationMs
}
