package zmqpart

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/hashcache"
)

type hashCacheSN struct {
	hashcache.HashCacheBase
	largestIndex int
	indexChanged uint64
}

func newHashCacheSN(hashLen int, segmentDurationMs uint64, retentionPeriodMs uint64) *hashCacheSN {
	ret := &hashCacheSN{
		HashCacheBase: *hashcache.NewHashCacheBase(hashLen, segmentDurationMs, retentionPeriodMs),
	}
	ret.StartPurge()
	return ret
}

func (cache *hashCacheSN) checkCurrentMilestoneIndex(index int) (bool, uint64) {
	cache.Lock()
	defer cache.Unlock()

	if index < cache.largestIndex {
		return true, cache.indexChanged
	}
	if index == cache.largestIndex {
		return false, cache.indexChanged
	}
	debugf("------ milestone index changed %v --> %v ", cache.largestIndex, index)
	cache.largestIndex = index
	cache.indexChanged = utils.UnixMsNow()
	return false, cache.indexChanged
}