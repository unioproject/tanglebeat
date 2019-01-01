package bufferwe

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"sync"
	"time"
)

// BufferWE = Time Limited cache
type bufferweSegment struct {
	created uint64
	ts      []uint64
	data    []interface{}
	next    *bufferweSegment
}

type BufferWE struct {
	name              string
	withData          bool
	segmentDurationMs uint64
	retentionPeriodMs uint64
	top               *bufferweSegment
	mutex             *sync.Mutex
}

func NewBufferWE(name string, withData bool, segmentDurationSec int, retentionPeriodSec int) *BufferWE {
	ret := &BufferWE{
		name:              name,
		withData:          withData,
		segmentDurationMs: uint64(segmentDurationSec * 1000),
		retentionPeriodMs: uint64(retentionPeriodSec * 1000),
		mutex:             &sync.Mutex{},
	}
	return ret
}

func (buf *BufferWE) Reset() {
	if buf != nil {
		buf.mutex.Lock()
		buf.top = nil
		buf.mutex.Unlock()
	}
}

// returns false if empty
func (buf *BufferWE) purge() bool {
	if buf == nil {
		return false
	}
	buf.mutex.Lock()
	defer buf.mutex.Unlock()

	if buf.top == nil {
		return false
	}
	earliest := utils.UnixMsNow() - uint64(buf.retentionPeriodMs)
	if buf.top.created < earliest {
		buf.top = nil
		return false
	}
	for s := buf.top; s != nil; s = s.next {
		if s.next != nil && s.next.created < earliest {
			s.next = nil
		}
	}
	return true
}

// loop exits when buf becomes empty
func (buf *BufferWE) purgeLoop() {
	for buf.purge() {
		time.Sleep(10 * time.Second)
	}
}

func (buf *BufferWE) Push(data interface{}) {
	if buf == nil {
		return
	}
	buf.mutex.Lock()
	defer buf.mutex.Unlock()

	nowis := utils.UnixMsNow()
	if buf.top == nil || nowis-buf.top.created > buf.segmentDurationMs {
		capacity := 100
		if buf.top != nil {
			capacity = len(buf.top.ts)
			capacity += capacity / 20 // 5% more than the last
		}
		var dataArray []interface{}
		if buf.withData {
			dataArray = make([]interface{}, 0, capacity)
		}
		startPurge := buf.top == nil
		buf.top = &bufferweSegment{
			created: nowis,
			ts:      make([]uint64, 0, capacity),
			data:    dataArray,
			next:    buf.top,
		}
		if startPurge {
			// the purge goroutine will be started upon first push and will exit when purged uo to empty buffer
			go buf.purgeLoop()
		}
	}
	buf.top.ts = append(buf.top.ts, nowis)
	if buf.withData {
		buf.top.data = append(buf.top.data, data)
	}
}

func (buf *BufferWE) Last() interface{} {
	if buf == nil {
		return nil
	}
	buf.mutex.Lock()
	defer buf.mutex.Unlock()

	if !buf.withData || buf.top == nil {
		return nil
	}
	// ts array must be same length as data array and not empty
	earliest := utils.UnixMsNow() - buf.retentionPeriodMs
	idx := len(buf.top.ts) - 1 // must be > 0
	if buf.top.ts[idx] < earliest {
		// last one is out of retention period
		return nil
	}
	return buf.top.data[idx]
}

func (buf *BufferWE) TouchLast() {
	if buf == nil {
		return
	}
	buf.mutex.Lock()
	defer buf.mutex.Unlock()

	if buf.top == nil || len(buf.top.ts) == 0 {
		return
	}
	buf.top.ts[len(buf.top.ts)-1] = utils.UnixMsNow()
}

func (buf *BufferWE) ForEach(callback func(data interface{})) {
	if buf == nil {
		return
	}
	buf.mutex.Lock()
	defer buf.mutex.Unlock()

	earliest := utils.UnixMsNow() - uint64(buf.retentionPeriodMs)
	for s := buf.top; s != nil; s = s.next {
		if s.created < earliest {
			return
		}
		for idx := range s.ts {
			if s.ts[idx] >= earliest {
				if buf.withData {
					callback(s.data[idx])
				} else {
					callback(nil)
				}
			}
		}
	}
}

func (buf *BufferWE) CountAll() int {
	var ret int
	buf.ForEach(func(data interface{}) {
		ret++
	})
	return ret
}
