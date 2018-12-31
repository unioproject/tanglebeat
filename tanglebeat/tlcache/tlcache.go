package tlcache

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"sync"
	"time"
)

// TLCache = Time Limited cache
type tlCacheEntry struct {
	Ts   uint64
	Data interface{}
}

type tlCacheSegment struct {
	last    uint64
	entries []tlCacheEntry
	next    *tlCacheSegment
}

type TLCache struct {
	name              string
	segmentDurationMs uint64
	retentionPeriodMs uint64
	top               *tlCacheSegment
	mutex             *sync.Mutex
}

func NewTlCache(name string, segmentDurationSec int, retentionPeriodSec int) *TLCache {
	ret := &TLCache{
		name:              name,
		segmentDurationMs: uint64(segmentDurationSec * 1000),
		retentionPeriodMs: uint64(retentionPeriodSec * 1000),
		mutex:             &sync.Mutex{},
	}
	go func() {
		for {
			time.Sleep(10 * time.Second)
			ret.purge()
		}
	}()
	return ret
}

func (cache *TLCache) Reset() {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	cache.top = nil
}

func (cache *TLCache) purge() {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	earliest := utils.UnixMsNow() - uint64(cache.retentionPeriodMs)
	for s := cache.top; s != nil; s = s.next {
		if s.next != nil && s.next.last < earliest {
			s.next = nil
		}
	}
}

func (cache *TLCache) Push(data interface{}) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	nowis := utils.UnixMsNow()
	if cache.top == nil || nowis-cache.top.last > cache.segmentDurationMs {
		capacity := 100
		if cache.top != nil {
			capacity = len(cache.top.entries) + 1
			capacity += capacity / 20 // 5% more than the last
		}
		cache.top = &tlCacheSegment{
			entries: make([]tlCacheEntry, 0, capacity),
			next:    cache.top,
		}
	}
	cache.top.last = nowis
	cache.top.entries = append(cache.top.entries, tlCacheEntry{
		Ts:   nowis,
		Data: data,
	})
}

func (cache *TLCache) ForEach(callback func(ts uint64, data interface{})) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	earliest := utils.UnixMsNow() - uint64(cache.retentionPeriodMs)
	for s := cache.top; s != nil; s = s.next {
		if s.last < earliest {
			return
		}
		for _, entry := range s.entries {
			if entry.Ts >= earliest {
				callback(entry.Ts, entry.Data)
			}
		}
	}
}

func (cache *TLCache) CountAll() int {
	var ret int
	cache.ForEach(func(ts uint64, data interface{}) {
		ret++
	})
	return ret
}
