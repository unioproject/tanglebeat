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
	GetPrev() ExpiringSegment
	SetPrev(ExpiringSegment)
	Put(data ...interface{})
	Touch()
	Size() int
}

//-------------------------------
// ExpiringBuffer is abstract type, mus be used for concrete thread safe implementation
// It is linked list of ExpiringSegments and purge loop in the background
// Provides low level functions for implementations.
// Not thread safe, but has locking primitives to make it therad safe in implementations
// Purge routine in the background is synchronized with locking

type ExpiringBuffer struct {
	segDurationMs      uint64
	retentionPeriodMs  uint64
	segmentConstructor func(prev ExpiringSegment) ExpiringSegment
	top                ExpiringSegment
	mutex              *sync.Mutex
}

// Thread safe through the lock of the whole buffer
func NewExpiringBuffer(segDurationSec, retentionPeriodSec int, constructor func(prev ExpiringSegment) ExpiringSegment) *ExpiringBuffer {
	return &ExpiringBuffer{
		segDurationMs:      uint64(segDurationSec * 1000),
		retentionPeriodMs:  uint64(retentionPeriodSec * 1000),
		segmentConstructor: constructor,
		mutex:              &sync.Mutex{},
	}
}

func (buf *ExpiringBuffer) Lock() {
	buf.mutex.Lock()
}

func (buf *ExpiringBuffer) Unlock() {
	buf.mutex.Unlock()
}

func (buf *ExpiringBuffer) isEmpty() bool {
	return buf.top == nil
}

const purgeLoopSleepSec = 5

// ---------------------- THREAD SAFE
// purge is protected by locking
func (buf *ExpiringBuffer) purge() bool {
	buf.Lock()
	defer buf.Unlock()
	if buf.isEmpty() {
		return false
	}
	if buf.top.IsExpired(buf.retentionPeriodMs) {
		tracef("Expiring Buffer purge routine: purged top segment with size = %v", buf.top.Size())
		buf.top = nil
		return false
	}
	for s := buf.top; ; {
		if s == nil {
			break
		}
		prev := s.GetPrev()
		if prev == nil {
			break
		}
		if prev.IsExpired(buf.retentionPeriodMs) {
			tracef("Expiring Buffer purge routine: purged segment of size = %v", prev.Size())
			s.SetPrev(nil)
			break
		}
		s = prev
	}
	return true
}

func (buf *ExpiringBuffer) purgeLoop() {
	tracef("Expiring Buffer purge routine: loop started")
	defer tracef("Expiring Buffer purge routine: loop finished")

	for buf.purge() {
		time.Sleep(time.Duration(purgeLoopSleepSec) * time.Second)
	}
}

func (buf *ExpiringBuffer) Size() (int, int) {
	buf.Lock()
	defer buf.Unlock()
	var numseg, numentries int
	for s := buf.top; s != nil; s = s.GetPrev() {
		numseg++
		numentries += s.Size()
	}
	return numseg, numentries
}

//------------------ NOT THREAD SAFE
func (buf *ExpiringBuffer) NewEntry(data ...interface{}) {
	empty := buf.isEmpty()
	if empty || !buf.top.IsOpen(buf.segDurationMs) {
		buf.top = buf.segmentConstructor(buf.top)
		if empty {
			go buf.purgeLoop()
		}
	}
	buf.top.Put(data...)
	buf.top.Touch()
}

func (buf *ExpiringBuffer) ForEachSegment(callback func(seg ExpiringSegment)) {
	for s := buf.top; s != nil; s = s.GetPrev() {
		if !s.IsExpired(buf.retentionPeriodMs) {
			callback(s)
		} else {
			return
		}
	}
}

//---------------------------------
type ExpiringSegmentBase struct {
	created   uint64
	lastTouch uint64
	prev      ExpiringSegment
}

func NewExpiringSegmentBase() *ExpiringSegmentBase {
	nowis := utils.UnixMsNow()
	return &ExpiringSegmentBase{
		created:   nowis,
		lastTouch: nowis,
	}
}

func (seg *ExpiringSegmentBase) IsExpired(retentionPeriodMs uint64) bool {
	return utils.UnixMsNow()-seg.lastTouch >= retentionPeriodMs
}

func (seg *ExpiringSegmentBase) IsOpen(segDurationMs uint64) bool {
	return utils.UnixMsNow()-seg.created < segDurationMs
}

func (seg *ExpiringSegmentBase) GetPrev() ExpiringSegment {
	return seg.prev
}

func (seg *ExpiringSegmentBase) SetPrev(prev ExpiringSegment) {
	seg.prev = prev
}

func (seg *ExpiringSegmentBase) Touch() {
	seg.lastTouch = utils.UnixMsNow()
}

func (seg *ExpiringSegmentBase) IsOpen_(segDurationMs uint64) bool {
	return utils.UnixMsNow()-seg.created < segDurationMs
}
