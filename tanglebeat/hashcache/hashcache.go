package hashcache

import (
	"github.com/lunfardo314/tanglebeat/lib/bufferwe"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"sync"
	"time"
)

type CacheEntry struct {
	FirstSeen uint64
	LastSeen  uint64
	Visits    int // testing
	Data      interface{}
}

type cacheSegment struct {
	themap map[string]CacheEntry
	mutex  *sync.Mutex
}

type HashCacheBase struct {
	bufferwe.BufferWithExpiration
	hashLen               int
	segmentDurationMsCopy uint64
	retentionPeriodMsCopy uint64
}

func NewHashCacheBase(hashLen int, segmentDurationSec int, retentionPeriodSec int) *HashCacheBase {
	return &HashCacheBase{
		BufferWithExpiration:  *bufferwe.NewBufferWE(true, segmentDurationSec, retentionPeriodSec, "HashCacheBase"),
		hashLen:               hashLen,
		segmentDurationMsCopy: uint64(segmentDurationSec * 1000),
		retentionPeriodMsCopy: uint64(retentionPeriodSec * 1000),
	}
}

func (cache *HashCacheBase) shortHash(hash string) string {
	if cache.hashLen == 0 {
		return hash
	}
	ret := make([]byte, cache.hashLen)
	copy(ret, hash[:cache.hashLen])
	return string(ret)
}

func (cache *HashCacheBase) __insertNew(shorthash string, data interface{}) {
	created, segtmp := cache.Last__()
	var seg *cacheSegment
	if segtmp != nil {
		seg = segtmp.(*cacheSegment)
	}

	nowis := utils.UnixMs(time.Now())
	if seg == nil || nowis-created > cache.segmentDurationMsCopy {
		newseg := &cacheSegment{
			themap: make(map[string]CacheEntry),
			mutex:  &sync.Mutex{},
		}
		newseg.themap[shorthash] = CacheEntry{
			FirstSeen: nowis,
			LastSeen:  nowis,
			Visits:    1,
			Data:      data,
		}
		cache.Push__(newseg)
	} else {
		seg.themap[shorthash] = CacheEntry{
			FirstSeen: nowis,
			LastSeen:  nowis,
			Visits:    1,
			Data:      data,
		}
		cache.TouchLast__()
	}
}

// finds entry and increases visit counter if found
func (cache *HashCacheBase) __find(shorthash string, ret *CacheEntry) bool {
	var found bool
	cache.ForEach__(func(data interface{}) bool {
		seg, _ := data.(*cacheSegment)
		seg.mutex.Lock()
		defer seg.mutex.Unlock()
		entry, ok := seg.themap[shorthash]
		if ok {
			found = true
			seg.themap[shorthash] = CacheEntry{
				FirstSeen: entry.FirstSeen,
				LastSeen:  utils.UnixMsNow(),
				Visits:    entry.Visits + 1,
				Data:      entry.Data,
			}
			if ret != nil {
				*ret = seg.themap[shorthash]
			}
			return true
		}
		return false
	})
	return found
}

func (cache *HashCacheBase) Find(hash string, ret *CacheEntry) bool {
	cache.Lock()
	defer cache.Unlock()
	return cache.__find(cache.shortHash(hash), ret)
}

func (cache *HashCacheBase) __findWithDelete(shorthash string, ret *CacheEntry) bool {
	var found bool
	cache.ForEach__(func(data interface{}) bool {
		seg, _ := data.(*cacheSegment)
		seg.mutex.Lock()
		defer seg.mutex.Unlock()
		entry, ok := seg.themap[shorthash]
		if ok {
			found = true
			if ret != nil {
				*ret = entry
			}
			delete(seg.themap, shorthash)
			return true
		}
		return false
	})
	return found
}

// if seen, return entry and deletes it
func (cache *HashCacheBase) FindWithDelete(hash string, ret *CacheEntry) bool {
	cache.Lock()
	defer cache.Unlock()

	shash := cache.shortHash(hash)
	return cache.__findWithDelete(shash, ret)
}

func (cache *HashCacheBase) SeenHash(hash string, data interface{}, ret *CacheEntry) bool {
	cache.Lock()
	defer cache.Unlock()

	shash := cache.shortHash(hash)
	if seen := cache.__find(shash, ret); seen {
		return true
	}
	cache.__insertNew(shash, data)
	return false
}

type hashcacheStats struct {
	Numseg         int     `json:"numseg"`
	Numtx          int     `json:"numtx"`
	NumNoVisit     int     `json:"numNoVisit"`
	NumNoVisitPerc int     `json:"numNoVisitPerc"`
	LatencySecMax  float64 `json:"latencyMsMax"`
	LatencySecAvg  float64 `json:"latencyMsAvg"`
}

func (cache *HashCacheBase) Stats(msecBack uint64) *hashcacheStats {
	earliest := utils.UnixMsNow() - msecBack
	if msecBack == 0 {
		earliest = 0 // count all of it
	}
	ret := &hashcacheStats{}
	var numVisited int
	var lat float64

	cache.ForEachEntry(func(entry *CacheEntry) {
		ret.Numtx++
		if entry.LastSeen >= earliest {
			if entry.Visits > 1 {
				numVisited++
				lat = float64(entry.LastSeen-entry.FirstSeen) / 1000
				ret.LatencySecAvg += lat
				if lat > ret.LatencySecMax {
					ret.LatencySecMax = lat
				}
			} else {
				ret.NumNoVisit++
			}
		}
	})
	if ret.Numtx > 0 {
		ret.NumNoVisitPerc = (100 * ret.NumNoVisit) / ret.Numtx
	}
	if numVisited == 0 {
		numVisited = 1
	}
	ret.LatencySecAvg = ret.LatencySecAvg / float64(numVisited)
	return ret
}

func (cache *HashCacheBase) ForEachEntry(callback func(entry *CacheEntry)) {
	earliest := utils.UnixMsNow() - cache.retentionPeriodMsCopy
	cache.ForEach(func(data interface{}) bool {
		seg, _ := data.(*cacheSegment)
		seg.mutex.Lock()
		defer seg.mutex.Unlock()
		for _, entry := range seg.themap {
			if entry.LastSeen >= earliest {
				callback(&entry)
			}
		}
		return true
	})
}
