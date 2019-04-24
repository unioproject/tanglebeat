package inputpart

import (
	"github.com/unioproject/tanglebeat/tanglebeat/hashcache"
	"time"
)

type bundleCache struct {
	hashcache.HashCacheBase
}

type transferBundleData struct {
	hash            string
	fakeTransfer    bool
	nonZeroEntries  map[string]int64
	posted          bool
	postedValue     uint64
	valueOfReminder uint64 // assumed last in the bundle
	confirmed       bool
	numUpdate       int
}

func newBundleCache(hashLen int, segmentDurationSec int, retentionPeriodSec int) *bundleCache {
	ret := &bundleCache{
		HashCacheBase: *hashcache.NewHashCacheBase("bundlecache", hashLen, segmentDurationSec, retentionPeriodSec),
	}
	return ret
}

func (cache *bundleCache) updateBundleData(bundleHash, addr string, value int64, lastInBundle bool) {
	cache.Lock()
	defer cache.Unlock()

	var entry hashcache.CacheEntry
	var data *transferBundleData

	shash := cache.ShortHash(bundleHash)
	seen := cache.FindNolock(shash, &entry, true)

	if seen {
		data = entry.Data.(*transferBundleData)
		debugf("Bundle '%v' updating with value = %v # = %v", bundleHash, value, data.numUpdate)
		if data.confirmed {
			debugf("+++++++++++++++ Bundle '%v' already confirmed. # = %v", bundleHash, data.numUpdate)
		}
		val, ok := data.nonZeroEntries[addr]
		if ok {
			if value > 0 && val < 0 || value < 0 && val > 0 {
				data.fakeTransfer = true
				debugf("Bundle '%v' marked FAKE", bundleHash)
				return
			}
			data.nonZeroEntries[addr] += value
		} else {
			data.nonZeroEntries[addr] = value
		}
	} else {
		debugf("Bundle '%v' creating new entry with value = %v", bundleHash, value)
		data = &transferBundleData{
			hash:           shash,
			nonZeroEntries: make(map[string]int64),
		}
		data.nonZeroEntries[addr] = value
		cache.InsertNewNolock(shash, 0, data)
	}
	if lastInBundle && value > 0 {
		data.valueOfReminder = uint64(value)
		debugf("Bundle '%v' lastInBundle, value = %v", bundleHash, data.valueOfReminder)
	}
	var s int64
	for _, v := range data.nonZeroEntries {
		s += v
	}
	debugf("Bundle '%v' balance = %v", bundleHash, s)
	data.posted = false
	data.numUpdate++
}

func (cache *bundleCache) markConfirmed(bundleHash string) {
	var entry hashcache.CacheEntry
	var data *transferBundleData

	seen := transferBundleCache.Find(bundleHash, &entry)
	if !seen {
		return
	}
	data = entry.Data.(*transferBundleData)
	if !data.confirmed {
		data.confirmed = true
		debugf("Bundle %v... marked CONFIRMED", data.hash)
	}
}

func updateBundleMetricsLoop() {
	debugf("Started 'updateBundleMetricsLoop'")
	for {
		time.Sleep(3 * time.Second)
		var data *transferBundleData
		var value uint64
		transferBundleCache.ForEachEntry(func(entry *hashcache.CacheEntry) {
			var positSum uint64
			var sum int64
			data = entry.Data.(*transferBundleData)
			debugf("++++++ Checking bundle %v... fake=%v confirmed=%v posted=%v",
				data.hash, data.fakeTransfer, data.confirmed, data.posted)

			if data.fakeTransfer || !data.confirmed || data.posted {
				return
			}
			for _, v := range data.nonZeroEntries {
				if v > 0 {
					positSum += uint64(v)
				}
				sum += v
			}
			debugf("++++++ Checking bundle %v... sum = %v ", data.hash, sum)
			if sum == 0 {
				// if balanced
				value = positSum - data.valueOfReminder - data.postedValue
				prevPosted := data.postedValue
				data.postedValue = value
				data.posted = true
				debugf("Bundle '%v' metrics to be updated with value = %v, already posted = %v reminder = %v",
					data.hash, value, prevPosted, data.valueOfReminder)
				//updateTransferVolume(value)
			}
		}, 0, true)
	}
}
