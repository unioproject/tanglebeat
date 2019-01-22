package hashcache

import (
	"github.com/lunfardo314/tanglebeat/lib/ebuffer"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/cfg"
)

type CacheEntry struct {
	FirstSeen    uint64
	LastSeen     uint64
	Visits       byte
	FirstVisitId byte
	Data         interface{}
}

type cacheSegment struct {
	ebuffer.ExpiringSegmentBase
	themap map[string]CacheEntry
}

type HashCacheBase struct {
	ebuffer.ExpiringBuffer
	hashLen               int
	segmentDurationMsCopy uint64
	retentionPeriodMsCopy uint64
}

var segmentConstructor = func(prev ebuffer.ExpiringSegment) ebuffer.ExpiringSegment {
	ret := &cacheSegment{
		ExpiringSegmentBase: *ebuffer.NewExpiringSegmentBase(),
		themap:              make(map[string]CacheEntry),
	}
	ret.SetPrev(prev)
	return ebuffer.ExpiringSegment(ret)
}

func NewHashCacheBase(id string, hashLen int, segmentDurationSec int, retentionPeriodSec int) *HashCacheBase {
	return &HashCacheBase{
		ExpiringBuffer:        *ebuffer.NewExpiringBuffer(id, segmentDurationSec, retentionPeriodSec, segmentConstructor),
		hashLen:               hashLen,
		segmentDurationMsCopy: uint64(segmentDurationSec * 1000),
		retentionPeriodMsCopy: uint64(retentionPeriodSec * 1000),
	}
}

func (seg *cacheSegment) Put(args ...interface{}) {
	shorthash := args[0].(string)
	nowis := utils.UnixMsNow()
	seg.themap[shorthash] = CacheEntry{
		FirstSeen:    nowis,
		LastSeen:     nowis,
		Visits:       1,
		FirstVisitId: args[1].(byte),
		Data:         args[2],
	}
}

func (seg *cacheSegment) Size() int {
	return len(seg.themap)
}

func (seg *cacheSegment) Find(shorthash string, ret *CacheEntry) bool {
	return seg.findIntern(shorthash, ret, true)
}

func (seg *cacheSegment) FindNoTouch(shorthash string, ret *CacheEntry) bool {
	return seg.findIntern(shorthash, ret, false)
}

// searches for the hash, marks if found
func (seg *cacheSegment) findIntern(shorthash string, ret *CacheEntry, touch bool) bool {
	entry, ok := seg.themap[shorthash]
	if !ok {
		return false
	}
	if touch {
		seg.themap[shorthash] = CacheEntry{
			FirstSeen: entry.FirstSeen,
			LastSeen:  utils.UnixMsNow(),
			Visits:    entry.Visits + 1,
			Data:      entry.Data,
		}
	}
	if ret != nil {
		*ret = seg.themap[shorthash]
	}
	return true
}

func (seg *cacheSegment) FindWithDelete(shorthash string, ret *CacheEntry) bool {
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

// returns prefix of the hash with a given length, a parameter of the HashCacheBase buffer
func (cache *HashCacheBase) ShortHash(hash string) string {
	if cache.hashLen == 0 || len(hash) <= cache.hashLen {
		return hash
	}
	ret := make([]byte, cache.hashLen)
	copy(ret, hash[:cache.hashLen])
	return string(ret)
}

func (cache *HashCacheBase) __insertNew(shorthash string, id byte, data interface{}) {
	cache.NewEntry(shorthash, id, data)
}

// finds entry and increases visit counter if found
func (cache *HashCacheBase) __find(shorthash string, ret *CacheEntry, touch bool) bool {
	var found bool
	cache.ForEachSegment__(func(seg ebuffer.ExpiringSegment) bool {
		if touch {
			found = seg.(*cacheSegment).Find(shorthash, ret)
		} else {
			found = seg.(*cacheSegment).FindNoTouch(shorthash, ret)
		}
		return !found // stop traversing, when found
	})
	return found
}

func (cache *HashCacheBase) Find(hash string, ret *CacheEntry) bool {
	cache.Lock()
	defer cache.Unlock()
	return cache.__find(cache.ShortHash(hash), ret, true)
}

func (cache *HashCacheBase) FindNoTouch(hash string, ret *CacheEntry) bool {
	cache.Lock()
	defer cache.Unlock()
	return cache.__find(cache.ShortHash(hash), ret, false)
}

func (cache *HashCacheBase) FindNoTouch__(hash string, ret *CacheEntry) bool {
	return cache.__find(cache.ShortHash(hash), ret, false)
}

func (cache *HashCacheBase) __findWithDelete(shorthash string, ret *CacheEntry) bool {
	var found bool
	cache.ForEachSegment__(func(seg ebuffer.ExpiringSegment) bool {
		if seg.(*cacheSegment).FindWithDelete(shorthash, ret) {
			found = true
			return false // stop traversing, it was found
		}
		return true
	})
	return found
}

// if seen, return entry and deletes it
func (cache *HashCacheBase) FindWithDelete(hash string, ret *CacheEntry) bool {
	cache.Lock()
	defer cache.Unlock()

	shash := cache.ShortHash(hash)
	return cache.__findWithDelete(shash, ret)
}

// TODO if same message is coming several times from same source it is passed. It is incorrect
func (cache *HashCacheBase) SeenHashBy(hash string, id byte, data interface{}, ret *CacheEntry) bool {
	cache.Lock()
	defer cache.Unlock()

	shash := cache.ShortHash(hash)
	if seen := cache.__find(shash, ret, true); seen {
		return true
	}
	cache.__insertNew(shash, id, data)
	return false
}

type hashcacheStats struct {
	TxCount           int
	TxCountPassed     int
	SeenOnce          int
	LatencySecAvg     float64
	EarliestSeen      uint64
	SeenOnceCountById map[byte]int
}

func (cache *HashCacheBase) Stats(msecBack uint64) *hashcacheStats {
	earliest := utils.UnixMsNow() - msecBack
	if msecBack == 0 {
		earliest = 0 // count all of it
	}
	ret := &hashcacheStats{
		EarliestSeen:      utils.UnixMsNow(),
		SeenOnceCountById: make(map[byte]int),
	}
	var lat float64
	var ok bool
	cache.ForEachEntry(func(entry *CacheEntry) {
		ret.TxCount++
		if entry.Visits == 1 {
			ret.SeenOnce++
			if _, ok = ret.SeenOnceCountById[entry.FirstVisitId]; !ok {
				ret.SeenOnceCountById[entry.FirstVisitId] = 0
			}
			ret.SeenOnceCountById[entry.FirstVisitId] += 1
		}
		if int(entry.Visits) >= cfg.Config.RepeatToAccept {
			lat = float64(entry.LastSeen-entry.FirstSeen) / 1000
			ret.LatencySecAvg += lat
			ret.TxCountPassed++
		}
		if entry.LastSeen < ret.EarliestSeen {
			ret.EarliestSeen = entry.LastSeen
		}
	}, earliest, true)

	if ret.TxCountPassed != 0 {
		ret.LatencySecAvg = ret.LatencySecAvg / float64(ret.TxCountPassed)
	} else {
		ret.LatencySecAvg = 0
	}
	return ret
}

func (cache *HashCacheBase) ForEachEntry(callback func(entry *CacheEntry), earliest uint64, lock bool) uint64 {
	if lock {
		cache.Lock()
		defer cache.Unlock()
	}
	retEarliest := utils.UnixMsNow()
	cache.ForEachSegment__(func(s ebuffer.ExpiringSegment) bool {
		seg := s.(*cacheSegment)
		for _, entry := range seg.themap {
			if entry.LastSeen >= earliest {
				callback(&entry)
				if entry.LastSeen < retEarliest {
					retEarliest = entry.LastSeen
				}
			}
		}
		return true // always continue
	})
	return retEarliest
}
