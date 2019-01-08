package zmqpart

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/hashcache"
	"strconv"
)

type valueTxData struct {
	value        uint64
	tag          string
	lastInBundle bool
}

func processValueTxMsg(msgSplit []string) {
	switch msgSplit[0] {
	case "tx":
		// track hashes of >0 value transaction if 'positiveValueTxCache'.
		if len(msgSplit) >= 9 {
			if value, err := strconv.Atoi(msgSplit[3]); err == nil && value > 0 {
				data := &valueTxData{
					value:        uint64(value),
					tag:          msgSplit[4],
					lastInBundle: msgSplit[6] == msgSplit[7],
				}
				// Store tx hash is seen first time to wait for corresponding 'sn' message
				positiveValueTxCache.SeenHashBy(msgSplit[1], 0, data, nil) // transaction
				conf := false
				// Store bundle hash is seen first time to wait for corresponding 'sn' message (track bundle confirmation)
				valueBundleCache.SeenHashBy(msgSplit[8], 0, &conf, nil) // bundle. data is *bool
			}
		} else {
			errorf("toOutput: expected at least 9 fields in TX message")
		}
	case "sn":
		if len(msgSplit) >= 7 {
			var entry hashcache.CacheEntry
			// confirmed value transaction received
			// checking if it was seen positiveValueTxCache.
			// If so, delete it from there and update corresponding metrics
			// tx is not needed in the cache anymore because another message with the same hash won't come
			seen := positiveValueTxCache.FindWithDelete(msgSplit[2], &entry)
			if seen {
				if entry.Data == nil {
					errorf("ValueTX entry.data == nil")
					panic("ValueTX entry.data == nil")
				}
				if vtd, ok := entry.Data.(*valueTxData); ok {
					confirmedTransfers.RecordTS(vtd.value)
					updateConfirmedValueTxMetrics(vtd.value, vtd.lastInBundle)
					infof("Confirmed value tx %v value = %v tag = %v duration %v min",
						msgSplit[2], vtd.value, vtd.tag, float32(utils.SinceUnixMs(entry.FirstSeen))/60000,
					)
				}
			}
			// confirmed value bundle
			// check if it was seen in 'valueBundleCache'
			// if it was seen first time (!*pconf), update corresponding metrics
			// bundle is not delete from the cache, just market as 'confirmed' and then purged by the
			// background routine after 'retentionPeriod'
			// that is because bundle must be kept in the cache as long as confirmations with that bundle hash are coming
			seen = valueBundleCache.Find(msgSplit[6], &entry)
			if seen {
				pconf := entry.Data.(*bool)
				if !*pconf {
					updateConfirmedValueBundleMetrics()
					infof("Confirmed bundle %v", msgSplit[6])
				}
				*pconf = true
			}
		} else {
			errorf("toOutput: expected at least 7 fields in SN message")
		}
	}
}

func getValueConfirmationStats(msecBack uint64) (int, uint64) {
	var count int
	var value uint64
	earliest := utils.UnixMsNow() - msecBack
	if msecBack == 0 {
		earliest = 0
	}
	confirmedTransfers.ForEachEntry(func(ts uint64, data interface{}) bool {
		count++
		value += data.(uint64)
		return true
	}, earliest, true)
	return count, value
}
