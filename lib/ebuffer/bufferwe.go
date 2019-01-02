package ebuffer

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"sync"
	"time"
)

// BufferWithExpiration = Buffer With Expiration
type bufferweSegment struct {
	created      uint64
	lastModified uint64
	ts           []uint64
	data         []interface{}
	next         *bufferweSegment
}

type BufferWithExpiration struct {
	id                string
	withData          bool
	segmentDurationMs uint64
	retentionPeriodMs uint64
	top               *bufferweSegment
	mutex             *sync.Mutex
}

func (buf *BufferWithExpiration) Lock() {
	buf.mutex.Lock()
}

func (buf *BufferWithExpiration) Unlock() {
	buf.mutex.Unlock()
}

func NewBufferWE(withData bool, segmentDurationSec int, retentionPeriodSec int, id string) *BufferWithExpiration {
	ret := &BufferWithExpiration{
		id:                id,
		withData:          withData,
		segmentDurationMs: uint64(segmentDurationSec * 1000),
		retentionPeriodMs: uint64(retentionPeriodSec * 1000),
		mutex:             &sync.Mutex{},
	}
	return ret
}

func (buf *BufferWithExpiration) Reset() {
	if buf != nil {
		buf.Lock()
		buf.top = nil
		buf.Unlock()
	}
}

// returns false if empty
func (buf *BufferWithExpiration) purge() bool {
	if buf == nil {
		return false
	}
	buf.Lock()
	defer buf.Unlock()

	if buf.top == nil {
		return false
	}
	earliest := utils.UnixMsNow() - uint64(buf.retentionPeriodMs)
	if buf.top.lastModified < earliest {
		tracef("++++++++++ purge segment in %v. len = %v", buf.id, len(buf.top.ts))
		buf.top = nil
		return false
	}
	for s := buf.top; s != nil; s = s.next {
		if s.next != nil && s.next.lastModified < earliest {
			tracef("++++++++++ purge segment in %v. len = %v", buf.id, len(s.next.ts))
			s.next = nil
		}
	}
	return true
}

// loop exits when buf becomes empty
func (buf *BufferWithExpiration) purgeLoop() {
	tracef("++++++++++ Start purge loop for BufferWithExpiration id='%v'", buf.id)
	defer tracef("++++++++++ Finish purge loop for BufferWithExpiration id='%v'", buf.id)

	for buf.purge() {
		time.Sleep(10 * time.Second)
	}
}

func (buf *BufferWithExpiration) Push(data interface{}) {
	if buf == nil {
		return
	}
	buf.Lock()
	defer buf.Unlock()
	buf.Push__(data)
}

func (buf *BufferWithExpiration) Push__(data interface{}) {
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
			created:      nowis,
			lastModified: nowis,
			ts:           make([]uint64, 0, capacity),
			data:         dataArray,
			next:         buf.top,
		}
		if startPurge {
			// the purge goroutine will be started upon first push and will exit when purged uo to empty buffer
			go buf.purgeLoop()
		}
	}
	buf.top.ts = append(buf.top.ts, nowis)
	buf.top.lastModified = nowis
	if buf.withData {
		buf.top.data = append(buf.top.data, data)
	}
}

func (buf *BufferWithExpiration) Last() (uint64, interface{}) {
	if buf == nil {
		return 0, nil
	}
	buf.Lock()
	defer buf.Unlock()
	return buf.Last__()
}

func (buf *BufferWithExpiration) Last__() (uint64, interface{}) {
	if buf == nil {
		return 0, nil
	}
	if buf.top == nil {
		return 0, nil
	}
	earliest := utils.UnixMsNow() - buf.retentionPeriodMs
	idx := len(buf.top.ts) - 1 // must be > 0, because buf.top != nil
	retts := buf.top.ts[idx]
	if retts < earliest {
		// last one is out of retention period
		return 0, nil
	}
	if buf.withData {
		return retts, buf.top.data[idx]
	}
	return retts, nil
}

func (buf *BufferWithExpiration) TouchLast__() {
	if buf.top == nil || len(buf.top.ts) == 0 {
		return
	}
	buf.top.lastModified = utils.UnixMsNow()
}

func (buf *BufferWithExpiration) ForEach(callback func(data interface{}) bool) {
	if buf == nil {
		return
	}
	buf.Lock()
	defer buf.Unlock()
	buf.ForEach__(callback)
}

func (buf *BufferWithExpiration) ForEach__(callback func(data interface{}) bool) {
	if buf == nil {
		return
	}
	earliest := utils.UnixMsNow() - uint64(buf.retentionPeriodMs)
	for s := buf.top; s != nil; s = s.next {
		if s.created < earliest {
			return
		}
		var exit bool
		for idx := range s.ts {
			if s.ts[idx] >= earliest {
				if buf.withData {
					exit = callback(s.data[idx])
				} else {
					exit = callback(nil)
				}
				if exit {
					return
				}
			}
		}
	}
}

func (buf *BufferWithExpiration) CountAll() int {
	var ret int
	buf.ForEach(func(data interface{}) bool {
		ret++
		return false
	})
	return ret
}
