package main

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"sync"
	"time"
)

type cacheEntry struct {
	firstSeen uint64
	lastSeen  uint64
	visits    int // testing
	data      interface{}
}

type cacheSegment struct {
	themap  map[string]cacheEntry
	created uint64
	latest  uint64
	next    *cacheSegment
}

type hashCacheBase struct {
	mutex             *sync.Mutex
	hashLen           int
	segmentDurationMs uint64
	retentionPeriodMs uint64
	top               *cacheSegment
}

//type hashCache interface {
//	shortHash(hash string) string
//	__insertNew(shorthash string, data interface{})
//	__find(shorthash string) (uint64, interface{}, bool)
//	seenHash(hash string, data interface{}) (uint64, interface{}, bool)
//	__findWithDelete(shorthash string) (uint64, interface{}, bool)
//	seenAtWithDelete(hash string, data interface{}) (uint64, interface{}, bool)
//	stats() (int, int)
//	__stats() (int, int)
//	startPurge()
//}

func newHashCacheBase(hashLen int, segmentDurationMs uint64, retentionPeriodMs uint64) *hashCacheBase {
	ret := &hashCacheBase{
		hashLen:           hashLen,
		segmentDurationMs: segmentDurationMs,
		retentionPeriodMs: retentionPeriodMs,
		mutex:             &sync.Mutex{},
	}
	ret.startPurge()
	return ret
}

func (cache *hashCacheBase) shortHash(hash string) string {
	if cache.hashLen == 0 {
		return hash
	}
	ret := make([]byte, cache.hashLen)
	copy(ret, hash[:cache.hashLen])
	return string(ret)
}

func (cache *hashCacheBase) __insertNew(shorthash string, data interface{}) {
	nowis := utils.UnixMs(time.Now())
	if cache.top == nil || (len(cache.top.themap) != 0 && (nowis-cache.top.created > cache.segmentDurationMs)) {
		cache.top = &cacheSegment{
			themap:  make(map[string]cacheEntry),
			next:    cache.top,
			created: nowis,
		}
	}
	cache.top.themap[shorthash] = cacheEntry{
		firstSeen: nowis,
		lastSeen:  nowis,
		visits:    1,
		data:      data,
	}
	cache.top.latest = nowis
}

// finds entry and increases visit counter if found
func (cache *hashCacheBase) __find(shorthash string, ret *cacheEntry) bool {
	for seg := cache.top; seg != nil; seg = seg.next {
		if entry, ok := seg.themap[shorthash]; ok {
			seg.themap[shorthash] = cacheEntry{
				firstSeen: entry.firstSeen,
				lastSeen:  utils.UnixMsNow(),
				visits:    entry.visits + 1,
				data:      entry.data,
			}
			if ret != nil {
				*ret = entry
			}
			return true
		}
	}
	return false
}

func (cache *hashCacheBase) find(hash string, ret *cacheEntry) bool {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	return cache.__find(cache.shortHash(hash), ret)
}

func (cache *hashCacheBase) __findWithDelete(shorthash string, ret *cacheEntry) bool {
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
func (cache *hashCacheBase) findWithDelete(hash string, ret *cacheEntry) bool {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	shash := cache.shortHash(hash)
	return cache.__findWithDelete(shash, ret)
}

func (cache *hashCacheBase) seenHash(hash string, data interface{}, ret *cacheEntry) bool {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	shash := cache.shortHash(hash)
	if seen := cache.__find(shash, ret); seen {
		return true
	}
	cache.__insertNew(shash, data)
	return false
}

func (cache *hashCacheBase) startPurge() {
	if cache.segmentDurationMs > cache.retentionPeriodMs {
		return
	}
	go func() {
		for {
			time.Sleep(1 * time.Minute)

			cache.mutex.Lock()
			nowis := utils.UnixMs(time.Now())
			for top := cache.top; top != nil; top = top.next {
				if top.next != nil && (nowis-top.next.latest > cache.retentionPeriodMs) {
					top.next = nil // cut the tail
				}
			}
			cache.mutex.Unlock()
		}
	}()
}

type hashcacheStats struct {
	Numseg         int     `json:"numseg"`
	Numtx          int     `json:"numtx"`
	NumNoVisit     int     `json:"numNoVisit"`
	NumNoVisitPerc int     `json:"numNoVisitPerc"`
	LatencySecMax  float64 `json:"latencyMsMax"`
	LatencySecAvg  float64 `json:"latencyMsAvg"`
}

func (cache *hashCacheBase) stats(msecBack uint64) *hashcacheStats {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	earliest := utils.UnixMsNow() - msecBack
	if msecBack == 0 {
		earliest = 0 // count all of it
	}
	ret := &hashcacheStats{}
	var numVisited int
	var lat float64
	for seg := cache.top; seg != nil; seg = seg.next {
		ret.Numseg += 1
		if seg.latest < earliest {
			continue
		}
		for _, entry := range seg.themap {
			ret.Numtx++
			if entry.lastSeen >= earliest {
				if entry.visits > 1 {
					numVisited++
					lat = float64(entry.lastSeen-entry.firstSeen) / 1000
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

func (cache *hashCacheBase) forEachEntry(callback func(entry *cacheEntry)) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	for seg := cache.top; seg != nil; seg = seg.next {
		for _, entry := range seg.themap {
			callback(&entry)
		}
	}
}
