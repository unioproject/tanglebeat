package zmqpart

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/hashcache"
	"strconv"
)

var repeatToAcceptTX int

func SetRepeatToAcceptTX(val int) {
	repeatToAcceptTX = val
}

func toOutput(msgData []byte, msgSplit []string, repeatedTimes int) {
	// check if message was seen exactly number of times as configured (usually 2)
	if repeatedTimes == repeatToAcceptTX {
		// publish message to output Nanomsg channel exactly as reaceived from ZeroMQ. For others to consume
		if err := compoundOutPublisher.PublishData(msgData); err != nil {
			errorf("Error while publishing data: %v", err)
		}
		// update metrics based on compound (resulting) message stream (TPS, CTPS etc)
		updateCompoundMetrics(msgSplit[0])
		// update global stats do display on internal debug dashboard
		ZmqStats.updateMsgStats(msgSplit)
		// analyze if this is value transaction. Process to collect necessary metrics
		processValueTxMsg(msgSplit)
	}
}

type valueTxData struct {
	value        uint64
	tag          string
	lastInBundle bool
}

func processValueTxMsg(msgSplit []string) {
	switch msgSplit[0] {
	case "tx":
		// track hashes of >0 value transaction if 'valueTxCache'.
		if len(msgSplit) >= 9 {
			if value, err := strconv.Atoi(msgSplit[3]); err == nil && value > 0 {
				data := &valueTxData{
					value:        uint64(value),
					tag:          msgSplit[4],
					lastInBundle: msgSplit[6] == msgSplit[7],
				}
				// Store tx hash is seen first time to wait for corresponding 'sn' message
				valueTxCache.SeenHash(msgSplit[1], data, nil) // transaction
				conf := false
				// Store bundle hash is seen first time to wait for corresponding 'sn' message (track bundle confirmation)
				valueBundleCache.SeenHash(msgSplit[8], &conf, nil) // bundle. data is *bool
			}
		} else {
			errorf("toOutput: expected at least 9 fields in TX message")
		}
	case "sn":
		if len(msgSplit) >= 7 {
			var entry hashcache.CacheEntry
			// confirmed value transaction received
			// checking if it was seen valueTxCache.
			// If so, delete it from there and update corresponding metrics
			// tx is not needed in the cache anymore because another message with the same hash won't come
			seen := valueTxCache.FindWithDelete(msgSplit[2], &entry)
			if seen {
				if entry.Data == nil {
					errorf("ValueTX entry.data == nil")
					panic("ValueTX entry.data == nil")
				}
				if vtd, ok := entry.Data.(*valueTxData); ok {
					ZmqStats.updateConfirmedValueTxStats(vtd.value)
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
