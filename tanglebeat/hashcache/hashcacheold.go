package hashcache

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"sync"
	"time"
)

type CacheEntryOld struct {
	FirstSeen uint64
	LastSeen  uint64
	Visits    int // testing
	Data      interface{}
}

type cacheSegmentOld struct {
	themap  map[string]CacheEntryOld
	created uint64
	latest  uint64
	next    *cacheSegmentOld
}

type HashCacheBaseOld struct {
	mutex             *sync.Mutex
	hashLen           int
	segmentDurationMs uint64
	retentionPeriodMs uint64
	top               *cacheSegmentOld
}

//type hashCache interface {
//	shortHash(hash string) string
//	__insertNew(shorthash string, Data interface{})
//	__find(shorthash string) (uint64, interface{}, bool)
//	SeenHash(hash string, Data interface{}) (uint64, interface{}, bool)
//	__findWithDelete(shorthash string) (uint64, interface{}, bool)
//	seenAtWithDelete(hash string, Data interface{}) (uint64, interface{}, bool)
//	Stats() (int, int)
//	__stats() (int, int)
//	StartPurge()
//}

func NewHashCacheBaseOld(hashLen int, segmentDurationSec uint64, retentionPeriodSec uint64) *HashCacheBaseOld {
	ret := &HashCacheBaseOld{
		hashLen:           hashLen,
		segmentDurationMs: segmentDurationSec * 1000,
		retentionPeriodMs: retentionPeriodSec * 1000,
		mutex:             &sync.Mutex{},
	}
	ret.StartPurge()
	return ret
}

func (cache *HashCacheBaseOld) Lock() {
	cache.mutex.Lock()
}

func (cache *HashCacheBaseOld) Unlock() {
	cache.mutex.Unlock()
}

func (cache *HashCacheBaseOld) shortHash(hash string) string {
	if cache.hashLen == 0 {
		return hash
	}
	ret := make([]byte, cache.hashLen)
	copy(ret, hash[:cache.hashLen])
	return string(ret)
}

func (cache *HashCacheBaseOld) __insertNew(shorthash string, data interface{}) {
	nowis := utils.UnixMs(time.Now())
	if cache.top == nil || (len(cache.top.themap) != 0 && (nowis-cache.top.created > cache.segmentDurationMs)) {
		cache.top = &cacheSegmentOld{
			themap:  make(map[string]CacheEntryOld),
			next:    cache.top,
			created: nowis,
		}
	}
	cache.top.themap[shorthash] = CacheEntryOld{
		FirstSeen: nowis,
		LastSeen:  nowis,
		Visits:    1,
		Data:      data,
	}
	cache.top.latest = nowis
}

// finds entry and increases visit counter if found
func (cache *HashCacheBaseOld) __find(shorthash string, ret *CacheEntryOld) bool {
	for seg := cache.top; seg != nil; seg = seg.next {
		if entry, ok := seg.themap[shorthash]; ok {
			seg.themap[shorthash] = CacheEntryOld{
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
	}
	return false
}

func (cache *HashCacheBaseOld) Find(hash string, ret *CacheEntryOld) bool {
	cache.Lock()
	defer cache.Unlock()

	return cache.__find(cache.shortHash(hash), ret)
}

func (cache *HashCacheBaseOld) __findWithDelete(shorthash string, ret *CacheEntryOld) bool {
	for seg := cache.top; seg != nil; seg = seg.next {
		if entry, ok := seg.themap[shorthash]; ok {
			if ret != nil {
				*ret = entry
			}
			delete(seg.themap, shorthash)
			return true
		}
	}
	return false
}

// if seen, return entry and deletes it
func (cache *HashCacheBaseOld) FindWithDelete(hash string, ret *CacheEntryOld) bool {
	cache.Lock()
	defer cache.Unlock()

	shash := cache.shortHash(hash)
	return cache.__findWithDelete(shash, ret)
}

func (cache *HashCacheBaseOld) SeenHash(hash string, data interface{}, ret *CacheEntryOld) bool {
	cache.Lock()
	defer cache.Unlock()

	shash := cache.shortHash(hash)
	if seen := cache.__find(shash, ret); seen {
		return true
	}
	cache.__insertNew(shash, data)
	return false
}

func (cache *HashCacheBaseOld) StartPurge() {
	if cache.segmentDurationMs > cache.retentionPeriodMs {
		return
	}
	go func() {
		for {
			time.Sleep(1 * time.Minute)

			cache.Lock()
			nowis := utils.UnixMs(time.Now())
			for top := cache.top; top != nil; top = top.next {
				if top.next != nil && (nowis-top.next.latest > cache.retentionPeriodMs) {
					top.next = nil // cut the tail
				}
			}
			cache.Unlock()
		}
	}()
}

type hashcacheStatsOld struct {
	Numseg         int     `json:"numseg"`
	Numtx          int     `json:"numtx"`
	NumNoVisit     int     `json:"numNoVisit"`
	NumNoVisitPerc int     `json:"numNoVisitPerc"`
	LatencySecMax  float64 `json:"latencyMsMax"`
	LatencySecAvg  float64 `json:"latencyMsAvg"`
}

func (cache *HashCacheBaseOld) Stats(msecBack uint64) *hashcacheStatsOld {
	cache.Lock()
	defer cache.Unlock()

	earliest := utils.UnixMsNow() - msecBack
	if msecBack == 0 {
		earliest = 0 // count all of it
	}
	ret := &hashcacheStatsOld{}
	var numVisited int
	var lat float64
	for seg := cache.top; seg != nil; seg = seg.next {
		ret.Numseg += 1
		if seg.latest < earliest {
			continue
		}
		for _, entry := range seg.themap {
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
		}
	}
	if ret.Numtx > 0 {
		ret.NumNoVisitPerc = (100 * ret.NumNoVisit) / ret.Numtx
	}
	if numVisited == 0 {
		numVisited = 1
	}
	ret.LatencySecAvg = ret.LatencySecAvg / float64(numVisited)
	return ret
}

func (cache *HashCacheBaseOld) ForEachEntry(callback func(entry *CacheEntryOld)) {
	cache.Lock()
	defer cache.Unlock()
	for seg := cache.top; seg != nil; seg = seg.next {
		for _, entry := range seg.themap {
			callback(&entry)
		}
	}
}
