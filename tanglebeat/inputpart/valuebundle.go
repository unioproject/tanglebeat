package inputpart

import (
	"github.com/unioproject/tanglebeat/tanglebeat/hashcache"
	"time"
)

type bundleCache struct {
	hashcache.HashCacheBase
}

type bundleEntry struct {
	addr  string // nil mean entry is not filled empty
	value int64
}

type transferBundleData struct {
	hash         string
	entries      []bundleEntry
	inconsistent bool
	counted      bool
	posted       bool
	postedValue  int64
	confirmed    bool
	numUpdate    int
}

func newBundleCache(hashLen int, segmentDurationSec int, retentionPeriodSec int) *bundleCache {
	ret := &bundleCache{
		HashCacheBase: *hashcache.NewHashCacheBase("bundlecache", hashLen, segmentDurationSec, retentionPeriodSec),
	}
	return ret
}

const maxBundleSize = 100

func (cache *bundleCache) updateBundleData(bundleHash, addr string, value int64, idx, lastIdx int) {
	if idx > lastIdx || lastIdx > maxBundleSize || lastIdx < 0 {
		errorf("Bundle '%v' is inconsistent or too big", bundleHash)
		return
	}

	cache.Lock()
	defer cache.Unlock()

	var entry hashcache.CacheEntry
	var data *transferBundleData

	shash := cache.ShortHash(bundleHash)
	seen := cache.FindNolock(shash, &entry, true)

	if seen {
		debugf("Bundle '%v' updating entry. Tx value = %v", bundleHash, value)
		data = entry.Data.(*transferBundleData)
		if idx >= len(data.entries) {
			errorf("Bundle '%v': tx index is out of bounds", bundleHash)
			return
		}
		if data.inconsistent {
			return
		}
		if data.entries[idx].addr == "" {
			data.entries[idx].addr = addr
			data.entries[idx].value = value
		} else {
			if data.entries[idx].addr != addr || data.entries[idx].value != value {
				data.inconsistent = true
				errorf("Bundle '%v' is inconsistent: address or value mismatch", bundleHash)
				return
			}
		}
	} else {
		debugf("Bundle '%v' creating new bundle entry. Tx value = %v", bundleHash, value)
		data = &transferBundleData{
			hash:    shash,
			entries: make([]bundleEntry, lastIdx+1, lastIdx+1),
		}
		data.entries[idx].addr = addr
		data.entries[idx].value = value
		cache.InsertNewNolock(shash, 0, data)
	}
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
	transferBundleCache.Lock()
	defer transferBundleCache.Unlock()

	data = entry.Data.(*transferBundleData)
	if !data.confirmed {
		data.confirmed = true
		debugf("Bundle %v... marked CONFIRMED", data.hash)
	}
}

// returns: isBalanced, isFake, sumPos, lastInBundleValue

func sumBundle(data *transferBundleData) (bool, bool, int64, int64) {
	var sum int64
	var sumPos int64
	for i := range data.entries {
		sum += data.entries[i].value
		if data.entries[i].value > 0 {
			sumPos += data.entries[i].value
		}
	}
	if sum != 0 {
		return false, false, sumPos, data.entries[len(data.entries)-1].value
	}
	// check if funds are moved outside addresses
	sumMap := make(map[string]int64)
	for i := range data.entries {
		if _, ok := sumMap[data.entries[i].addr]; !ok {
			sumMap[data.entries[i].addr] = 0
		}
		sumMap[data.entries[i].addr] += data.entries[i].value
	}
	for _, v := range sumMap {
		if v != 0 {
			return true, false, sumPos, data.entries[len(data.entries)-1].value
		}
	}
	return true, true, sumPos, data.entries[len(data.entries)-1].value
}

func updateBundleMetricsLoop() {
	debugf("Started 'updateBundleMetricsLoop'")
	var isBalanced, isFake bool
	var sumPos int64
	var valueLast, deltaValue int64
	var data *transferBundleData
	var newConfirmedValue int64
	var newConfirmedBundles int
	var reminder int64

	for {
		time.Sleep(3 * time.Second)
		newConfirmedValue = 0
		newConfirmedBundles = 0
		transferBundleCache.ForEachEntry(func(entry *hashcache.CacheEntry) {
			data = entry.Data.(*transferBundleData)
			if !data.confirmed || data.posted {
				return
			}
			isBalanced, isFake, sumPos, valueLast = sumBundle(data)
			debugf("++++++ Bundle %v...: isBalanced=%v isFake=%v sumPos=%v rem=%v",
				data.hash, isBalanced, isFake, sumPos, valueLast)

			if !isBalanced || isFake || sumPos == 0 {
				return
			}
			reminder = 0
			if valueLast > 0 {
				reminder = valueLast
			}
			if !data.counted {
				data.counted = true
				newConfirmedBundles++
				debugf("++++++ Bundle %v...: counting new", data.hash)
			}
			deltaValue = sumPos - reminder - data.postedValue
			debugf("++++++ Bundle %v...: posting value %v posted prev %v",
				data.hash, deltaValue, data.postedValue)

			data.postedValue = deltaValue
			data.posted = true

			newConfirmedValue += deltaValue
		}, 0, true)
		if newConfirmedValue > 0 {
			infof("Updating newly confirmed transfer value: %v", newConfirmedValue)
			updateTransferVolumeMetrics(uint64(newConfirmedValue))
		}
		if newConfirmedBundles > 0 {
			infof("Updating counter of newly confirmed transfers: %v", newConfirmedBundles)
			updateTransferCounter(newConfirmedBundles)
		}
	}
}
