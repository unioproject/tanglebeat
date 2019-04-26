package inputpart

import (
	"github.com/unioproject/tanglebeat/lib/utils"
	"github.com/unioproject/tanglebeat/tanglebeat/cfg"
	"github.com/unioproject/tanglebeat/tanglebeat/hashcache"
	"strconv"
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
	postedValue  int64
	posted       bool
	confirmed    bool
	numUpdate    int
}

var transferBundleCache *bundleCache

const segmentDurationBundleCacheSec = 10 * 60

func initValueTx() {
	transferBundleCache = newBundleCache(
		useFirstHashTrytes, segmentDurationBundleCacheSec, cfg.Config.RetentionPeriodMin*60)

	go updateBundleMetricsLoop()
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
	data.numUpdate++
	data.posted = false
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

// returns: isBalanced, valueMoved

func sumBundle(data *transferBundleData) int64 {
	var sum int64
	var sumPos int64

	if data.posted {
		return data.postedValue
	}

	for i := range data.entries {
		sum += data.entries[i].value
		if data.entries[i].value > 0 {
			sumPos += data.entries[i].value
		}
	}
	if sum != 0 {
		// only makes sense if all tx balanced to zero
		return 0
	}
	// collect net sums by address
	sumMap := make(map[string]int64)
	for _, e := range data.entries {
		if _, ok := sumMap[e.addr]; !ok {
			sumMap[e.addr] = 0
		}
		sumMap[e.addr] += e.value
	}
	var valueMoved int64
	for _, e := range data.entries {
		m, _ := sumMap[e.addr]
		if m > 0 && e.value > 0 {
			// if address balance moved then sum up positive value
			// thus excluded addresses which doesn't move balances
			valueMoved += e.value
		}
	}
	if valueMoved == 0 {
		return 0 // bundle doesn't move balances, it is fake transfer
	}
	//  assume:
	//  - reminder = last in the bundle if value > 0
	//  - corresponding addr balance moved
	//  - reminder < value moved
	//  - otherwise it is = 0
	reminder := data.entries[len(data.entries)-1].value
	if reminder > 0 {
		mv, _ := sumMap[data.entries[len(data.entries)-1].addr]
		if mv == 0 {
			reminder = 0
		}
	} else {
		reminder = 0
	}
	if valueMoved <= reminder {
		return 0
	}
	return valueMoved - reminder
}

func updateBundleMetricsLoop() {
	debugf("Started 'updateBundleMetricsLoop'")
	var data *transferBundleData
	var valueMoved, deltaValue, totalNewConfirmedValue int64
	var newConfirmedBundles int

	for {
		time.Sleep(4 * time.Second)

		totalNewConfirmedValue = 0
		newConfirmedBundles = 0

		transferBundleCache.ForEachEntry(func(entry *hashcache.CacheEntry) {
			data = entry.Data.(*transferBundleData)
			if !data.confirmed {
				return
			}
			valueMoved = sumBundle(data)
			deltaValue = valueMoved - data.postedValue
			if deltaValue == 0 {
				return
			}
			debugf("++++++ Bundle %v...: deltaValueMoved = %v", data.hash, deltaValue)

			if !data.counted {
				data.counted = true
				newConfirmedBundles++
				debugf("++++++ Bundle %v...: counting new", data.hash)
			}
			data.postedValue = valueMoved
			data.posted = true

			totalNewConfirmedValue += deltaValue
		}, 0, true)

		if totalNewConfirmedValue > 0 {
			infof("Updating newly confirmed transfer value: %v", totalNewConfirmedValue)
			updateTransferVolumeMetrics(uint64(totalNewConfirmedValue))
		}
		if newConfirmedBundles > 0 {
			infof("Updating counter of newly confirmed transfers: %v", newConfirmedBundles)
			updateTransferCounter(newConfirmedBundles)
		}
	}
}

func processValueTxMsg(msgSplit []string) {
	var idx, idxLast int

	switch msgSplit[0] {
	case "tx":
		if len(msgSplit) < 9 {
			errorf("toOutput: expected at least 9 fields in TX message")
			return
		}
		value, err := strconv.Atoi(msgSplit[3])
		if err != nil {
			errorf("toOutput: expected integer in value field")
			return
		}
		if value != 0 {
			idx, _ = strconv.Atoi(msgSplit[6])
			idxLast, _ = strconv.Atoi(msgSplit[7])

			transferBundleCache.updateBundleData(msgSplit[8], msgSplit[2], int64(value), idx, idxLast)
		}
	case "sn":
		if len(msgSplit) < 7 {
			errorf("toOutput: expected at least 7 fields in SN message")
			return
		}
		transferBundleCache.markConfirmed(msgSplit[6])
	}
}

// return num confirmed bundles, total value without last in bundle
func getValueConfirmationStats(msecBack uint64) (int, int64) {
	var confBundles int
	var totalValue int64

	earliest := utils.UnixMsNow() - msecBack
	if msecBack == 0 {
		earliest = 0
	}
	var data *transferBundleData
	transferBundleCache.ForEachEntry(func(entry *hashcache.CacheEntry) {
		data = entry.Data.(*transferBundleData)
		if data.counted {
			confBundles++
		}
		totalValue += data.postedValue
	}, earliest, true)
	return confBundles, totalValue
}
