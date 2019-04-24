package inputpart

import (
	"github.com/unioproject/tanglebeat/tanglebeat/hashcache"
	"time"
)

type bundleCache struct {
	hashcache.HashCacheBase
}

type transferBundleData struct {
	fakeTransfer    bool
	nonZeroEntries  map[string]int64
	posted          bool
	postedValue     uint64
	valueOfReminder uint64 // assumed last in the bundle
	confirmed       bool
}

func newBundleCache(hashLen int, segmentDurationSec int, retentionPeriodSec int) *bundleCache {
	ret := &bundleCache{
		HashCacheBase: *hashcache.NewHashCacheBase("bundlecache", hashLen, segmentDurationSec, retentionPeriodSec),
	}
	return ret
}

func (cache *bundleCache) updateValueBundleData(bundleHash, addr string, value int64, reminder bool) {
	cache.Lock()
	defer cache.Unlock()

	var entry hashcache.CacheEntry
	var data *transferBundleData

	shash := cache.ShortHash(bundleHash)
	seen := cache.FindNolock(shash, &entry, true)
	if seen {
		data = entry.Data.(*transferBundleData)
		val, ok := data.nonZeroEntries[addr]
		if ok {
			if value > 0 && val < 0 || value < 0 && val > 0 {
				data.fakeTransfer = true
				return
			}
			data.nonZeroEntries[addr] += value
		} else {
			data.nonZeroEntries[addr] = value
		}
	} else {
		data := &transferBundleData{
			nonZeroEntries: make(map[string]int64),
		}
		data.nonZeroEntries[addr] = value
		cache.InsertNewNolock(shash, 0, data)
	}
	if reminder && value > 0 {
		data.valueOfReminder = uint64(value)
	}
	data.posted = false
}

func (cache *bundleCache) markConfirmed(bundleHash string) {
	var entry hashcache.CacheEntry
	var data *valueBundleData

	seen := valueBundleCache.Find(bundleHash, &entry)
	if !seen {
		return
	}
	data = entry.Data.(*valueBundleData)
	data.confirmed = true
}

func updateValueMetricsLoop() {
	for {
		time.Sleep(3 * time.Second)
		var data *transferBundleData
		var value uint64
		valueBundleCache.ForEachEntry(func(entry *hashcache.CacheEntry) {
			var positSum uint64
			var sum int64
			data = entry.Data.(*transferBundleData)
			if !data.confirmed || data.posted {
				return
			}
			for _, v := range data.nonZeroEntries {
				if v > 0 {
					positSum += uint64(v)
				}
				sum += v
			}
			if sum == 0 {
				// if balanced
				value = positSum - data.postedValue - data.valueOfReminder
				//updateTransferVolume(value)
				data.postedValue = value
				data.posted = true
			}
		}, 0, true)
	}
}
