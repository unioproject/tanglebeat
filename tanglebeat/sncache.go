package main

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
)

type hashCacheSN struct {
	hashCacheBase
	largestIndex int
	indexChanged uint64
}

func newHashCacheSN(hashLen int, segmentDurationMs uint64, retentionPeriodMs uint64) *hashCacheSN {
	ret := &hashCacheSN{
		hashCacheBase: *newHashCacheBase(hashLen, segmentDurationMs, retentionPeriodMs),
	}
	ret.startPurge()
	return ret
}

func (cache *hashCacheSN) obsoleteIndex(index int) (bool, uint64) {
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
