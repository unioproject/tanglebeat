package inputpart

import (
	"github.com/unioproject/tanglebeat/lib/utils"
	"github.com/unioproject/tanglebeat/tanglebeat/hashcache"
	"strconv"
)

type valueTxData struct {
	value        uint64
	lastInBundle bool
}

type valueBundleData struct {
	confirmed bool
	when      uint64
}

func processValueTxMsg(msgSplit []string) {
	switch msgSplit[0] {
	case "tx":
		// track hashes of >0 value transaction if 'positiveValueTxCache'.
		if len(msgSplit) >= 9 {
			if value, err := strconv.Atoi(msgSplit[3]); err == nil {
				if value > 0 {
					data := &valueTxData{
						value:        uint64(value),
						lastInBundle: msgSplit[6] == msgSplit[7],
					}
					// Store tx hash is seen first time to wait for corresponding 'sn' message
					positiveValueTxCache.SeenHashBy(msgSplit[1], 0, data, nil) // transaction
					// Store bundle hash is seen first time to wait for corresponding 'sn' message (track bundle confirmation)
					valueBundleCache.SeenHashBy(msgSplit[8], 0, &valueBundleData{}, nil)
				}
			} else {
				errorf("toOutput: expected integer in value field")
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
				if vtd, ok := entry.Data.(*valueTxData); ok {
					// move value tx data to confirmedPositiveValueTx
					confirmedPositiveValueTx.RecordTS(entry.Data)
					updateConfirmedValueTxMetrics(vtd.value, vtd.lastInBundle)
					infof("Confirmed value tx %v value = %v duration %v min",
						msgSplit[2], vtd.value, float32(utils.SinceUnixMs(entry.FirstSeen))/60000,
					)
				} else {
					errorf("confirmedPositiveValueTx cache: wrong data type")
					panic("confirmedPositiveValueTx cache: wrong data type")
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
				vbd, _ := entry.Data.(*valueBundleData)
				if !vbd.confirmed {
					// not 100% correct
					valueBundleCache.Lock()
					vbd.confirmed = true
					vbd.when = utils.UnixMsNow()
					valueBundleCache.Unlock()

					updateConfirmedValueBundleMetrics()
					infof("Confirmed bundle %v", msgSplit[6])
				}
			}
		} else {
			errorf("toOutput: expected at least 7 fields in SN message")
		}
	}
}

// return num confirmed bundles, total value, total value without last in bundle
func getValueConfirmationStats(msecBack uint64) (int, uint64, uint64) {
	var confBundlesCount int
	var valueTotal uint64
	var valueTotalNotLastInBundle uint64

	earliest := utils.UnixMsNow() - msecBack
	if msecBack == 0 {
		earliest = 0
	}

	confirmedPositiveValueTx.ForEachEntry(func(ts uint64, data interface{}) bool {
		if ts >= earliest {
			vtd, _ := data.(*valueTxData)
			valueTotal += vtd.value
			if !vtd.lastInBundle {
				valueTotalNotLastInBundle += vtd.value
			}
		}
		return true
	}, earliest, true)

	valueBundleCache.ForEachEntry(func(entry *hashcache.CacheEntry) {
		vbd, _ := entry.Data.(*valueBundleData)
		if vbd.confirmed && vbd.when >= earliest {
			confBundlesCount++
		}
	}, earliest, true)

	return confBundlesCount, valueTotal, valueTotalNotLastInBundle
}
