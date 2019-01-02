package hashcache

import (
	"github.com/lunfardo314/tanglebeat/lib/ebuffer"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"sync"
	"time"
)

type CacheEntry2 struct {
	FirstSeen uint64
	LastSeen  uint64
	Visits    int
	Data      interface{}
}

type cacheSegment2 struct {
	ebuffer.ExpiringSegmentBase
	themap map[string]CacheEntry2
}

type HashCacheBase2 struct {
	ebuffer.ExpiringBuffer
	hashLen               int
	segmentDurationMsCopy uint64
	retentionPeriodMsCopy uint64
}

var constructor = func(prev ebuffer.ExpiringSegment) ebuffer.ExpiringSegment {
	ret := &cacheSegment2{
		ExpiringSegmentBase: *ebuffer.NewExpiringSegmentBase(),
		themap:              make(map[string]CacheEntry2),
	}
	return ebuffer.ExpiringSegment(ret)
}

func NewHashCacheBase2(hashLen int, segmentDurationSec int, retentionPeriodSec int) *HashCacheBase2 {
	return &HashCacheBase2{
		ExpiringBuffer:        *ebuffer.NewExpiringBuffer(segmentDurationSec, retentionPeriodSec, nil),
		hashLen:               hashLen,
		segmentDurationMsCopy: uint64(segmentDurationSec * 1000),
		retentionPeriodMsCopy: uint64(retentionPeriodSec * 1000),
	}
}

func (seg *cacheSegment2) Put(args ...interface{}) {
	seg.Lock()
	defer seg.Unlock()
	shorthash := args[0].(string)
	nowis := utils.UnixMsNow()
	seg.themap[shorthash] = CacheEntry2{
		FirstSeen: nowis,
		LastSeen:  nowis,
		Visits:    1,
		Data:      args[1],
	}
}

func (seg *cacheSegment2) Size() int {
	seg.RLock()
	defer seg.RUnlock()
	return len(seg.themap)
}

func (seg *cacheSegment2) Find(shorthash string, ret *CacheEntry2) bool {
	seg.Lock()
	defer seg.Unlock()
	entry, ok := seg.themap[shorthash]
	if !ok {
		return false
	}
	seg.themap[shorthash] = CacheEntry2{
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

func (seg *cacheSegment2) FindWithDelete(shorthash string, ret *CacheEntry2) bool {
	seg.Lock()
	defer seg.Unlock()
	entry, ok := seg.themap[shorthash]
	if !ok {
		return false
	}
	if ret != nil {
		*ret = entry
	}
	delete(seg.themap, shorthash)
	return true
}

func (cache *HashCacheBase2) shortHash(hash string) string {
	if cache.hashLen == 0 {
		return hash
	}
	ret := make([]byte, cache.hashLen)
	copy(ret, hash[:cache.hashLen])
	return string(ret)
}

func (cache *HashCacheBase2) __insertNew(shorthash string, data interface{}) {
	// TODO blogai -- dvigubas blokavimas
	cache.NewEntry(shorthash, data)
}

// finds entry and increases visit counter if found
func (cache *HashCacheBase2) __find(shorthash string, ret *CacheEntry2) bool {
	var found bool
	cache.ForEachSegment_(func(seg interface{}) bool {
		hcseg := seg.(*cacheSegment2)
		if hcseg.Find(shorthash, ret) {
			found = true
			return false
		}
		return true
	})
	return found
}

func (cache *HashCacheBase2) Find(hash string, ret *CacheEntry2) bool {
	cache.RLock()
	defer cache.RUnlock()
	return cache.__find(cache.shortHash(hash), ret)
}

func (cache *HashCacheBase2) __findWithDelete(shorthash string, ret *CacheEntry2) bool {
	var found bool
	cache.ForEachSegment_(func(seg interface{}) bool {
		hcseg := seg.(*cacheSegment2)
		if hcseg.FindWithDelete(shorthash, ret) {
			found = true
			return false
		}
		return true
	})
	return found
}

// if seen, return entry and deletes it
func (cache *HashCacheBase2) FindWithDelete(hash string, ret *CacheEntry2) bool {
	cache.Lock()
	defer cache.Unlock()

	shash := cache.shortHash(hash)
	return cache.__findWithDelete(shash, ret)
}

func (cache *HashCacheBase2) SeenHash(hash string, data interface{}, ret *CacheEntry2) bool {
	cache.RLock()
	defer cache.RUnlock()

	shash := cache.shortHash(hash)
	if seen := cache.__find(shash, ret); seen {
		return true
	}
	cache.__insertNew(shash, data)
	return false
}

type hashcacheStats2 struct {
	Numseg         int     `json:"numseg"`
	Numtx          int     `json:"numtx"`
	NumNoVisit     int     `json:"numNoVisit"`
	NumNoVisitPerc int     `json:"numNoVisitPerc"`
	LatencySecMax  float64 `json:"latencyMsMax"`
	LatencySecAvg  float64 `json:"latencyMsAvg"`
}

func (cache *HashCacheBase2) Stats(msecBack uint64) *hashcacheStats2 {
	earliest := utils.UnixMsNow() - msecBack
	if msecBack == 0 {
		earliest = 0 // count all of it
	}
	ret := &hashcacheStats2{}
	var numVisited int
	var lat float64

	cache.ForEachEntry(func(entry *CacheEntry2) {
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

func (cache *HashCacheBase2) ForEachEntry(callback func(entry *CacheEntry2)) {
	earliest := utils.UnixMsNow() - cache.retentionPeriodMsCopy
	cache.ForEach(func(data interface{}) bool {
		seg, _ := data.(*cacheSegment2)
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
