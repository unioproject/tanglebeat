package hashcache

import (
	"github.com/unioproject/tanglebeat/lib/ebuffer"
	"github.com/unioproject/tanglebeat/lib/utils"
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

func (cache *HashCacheBase) InsertNewNolock(shorthash string, id byte, data interface{}) {
	cache.NewEntry(shorthash, id, data)
}

// finds entry and increases visit counter if found
func (cache *HashCacheBase) FindNolock(shorthash string, ret *CacheEntry, touch bool) bool {
	var found bool
	if touch {
		cache.ForEachSegment__(func(seg ebuffer.ExpiringSegment) bool {
			found = seg.(*cacheSegment).Find(shorthash, ret)
			return !found // stop traversing when found
		})
	} else {
		cache.ForEachSegment__(func(seg ebuffer.ExpiringSegment) bool {
			found = seg.(*cacheSegment).FindNoTouch(shorthash, ret)
			return !found // stop traversing when found
		})
	}
	return found
}

func (cache *HashCacheBase) Find(hash string, ret *CacheEntry) bool {
	cache.Lock()
	defer cache.Unlock()
	return cache.FindNolock(cache.ShortHash(hash), ret, true)
}

func (cache *HashCacheBase) FindNoTouch(hash string, ret *CacheEntry) bool {
	cache.Lock()
	defer cache.Unlock()
	return cache.FindNolock(cache.ShortHash(hash), ret, false)
}

func (cache *HashCacheBase) FindNoTouch__(hash string, ret *CacheEntry) bool {
	return cache.FindNolock(cache.ShortHash(hash), ret, false)
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

// TODO if same message is coming several times from same source it is passed.
//  It is incorrect but probably not important practically

func (cache *HashCacheBase) SeenHashBy(hash string, id byte, data interface{}, ret *CacheEntry) bool {
	cache.Lock()
	defer cache.Unlock()

	shash := cache.ShortHash(hash)
	if seen := cache.FindNolock(shash, ret, true); seen {
		return true
	}
	cache.InsertNewNolock(shash, id, data)
	// if new entry, ret is not touched
	// CacheEntry is mock
	if ret != nil {
		nowis := utils.UnixMsNow()
		ret.Visits = 1
		ret.LastSeen = nowis
		ret.FirstSeen = nowis
		ret.Data = data
		ret.FirstVisitId = id
	}
	return false
}

type hashcacheStats struct {
	TxCount          int
	TxCountOlder1Min int
	TxCountPassed    int
	SeenOnce         int
	LatencySecAvg    float64
	EarliestSeen     uint64
	SeenOnceRateById map[byte]int
}

func (cache *HashCacheBase) Stats(msecBack uint64, quorumTx int) *hashcacheStats {
	nowis := utils.UnixMsNow()
	earliest := nowis - msecBack
	ago1min := nowis - 10*60*1000

	if msecBack == 0 {
		earliest = 0 // count all of it
	}
	ret := &hashcacheStats{
		EarliestSeen:     utils.UnixMsNow(),
		SeenOnceRateById: make(map[byte]int),
	}
	totalCount5to1MinById := make(map[byte]int)
	var lat float64
	var ok bool
	cache.ForEachEntry(func(entry *CacheEntry) {
		ret.TxCount++
		// counting only those seenOnce, which are older than 1 min
		if entry.FirstSeen <= ago1min {
			ret.TxCountOlder1Min++

			if _, ok = totalCount5to1MinById[entry.FirstVisitId]; !ok {
				totalCount5to1MinById[entry.FirstVisitId] = 0
			}
			totalCount5to1MinById[entry.FirstVisitId] += 1
			if entry.Visits == 1 {
				ret.SeenOnce++
				if _, ok = ret.SeenOnceRateById[entry.FirstVisitId]; !ok {
					ret.SeenOnceRateById[entry.FirstVisitId] = 0
				}
				ret.SeenOnceRateById[entry.FirstVisitId] += 1
			}
		}
		if int(entry.Visits) >= quorumTx {
			lat = float64(entry.LastSeen-entry.FirstSeen) / 1000
			ret.LatencySecAvg += lat
			ret.TxCountPassed++
		}
		if entry.LastSeen < ret.EarliestSeen {
			ret.EarliestSeen = entry.LastSeen
		}
	}, earliest, true)

	for id := range ret.SeenOnceRateById {
		ret.SeenOnceRateById[id] = (ret.SeenOnceRateById[id] * 100) / totalCount5to1MinById[id]
	}

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
