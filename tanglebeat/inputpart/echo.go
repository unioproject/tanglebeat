package inputpart

import (
	"github.com/unioproject/tanglebeat/lib/utils"
	"github.com/unioproject/tanglebeat/tanglebeat/hashcache"
	"time"
)

const (
	echoBufferHashLen            = 12
	echoBufferSegmentDurationSec = 60
	echoBufferRetentionPeriodSec = 30 * 60
)

const whenSeenArrayLen = 10

type echoEntry struct {
	whenSent     uint64
	seen         bool
	whenSeenLast uint64
	whenSeenNth  [whenSeenArrayLen]uint64
}

var (
	echoBuffer *hashcache.HashCacheBase
)

func startEchoLatencyRoutine() {
	echoBuffer = hashcache.NewHashCacheBase(
		"echoBuffer", echoBufferHashLen, echoBufferSegmentDurationSec, echoBufferRetentionPeriodSec)
	go func() {
		debugf("Started echo latency calculation routine")
		var echoParams avgEchoParams
		for {
			time.Sleep(10 * time.Second)
			calcAvgEchoParams(&echoParams)
			updateEchoMetrics(&echoParams)
		}
	}()
}

// called by update_collector for each PROMOTE update received from the tbsender
// never called in case tbsender isn't running
// normally called once (as tail promoted is once) and results in record in echoBuffer

func TxSentForEcho(txhash string, ts uint64) {
	echoBuffer.SeenHashBy(txhash, 0, &echoEntry{
		whenSent: ts,
	}, nil)
	debugf("++++++Promo tx waiting for echo: %v...", txhash[:12])
}

// it is called for each tx message to check if echo is expected

func checkForEcho(txhash string, ts uint64) {
	var entry hashcache.CacheEntry
	echoBuffer.Lock()
	defer echoBuffer.Unlock()

	if echoBuffer.FindNolock(echoBuffer.ShortHash(txhash), &entry, true) {
		d := entry.Data.(*echoEntry)
		if 1 <= entry.Visits && entry.Visits <= whenSeenArrayLen {
			d.whenSeenNth[entry.Visits-1] = ts
			d.whenSeenLast = ts
			d.seen = true
		}
		debugf("+++++++ Promo tx echo nr = %v in %v msec. %v..", entry.Visits, ts-d.whenSent, txhash[:12])
	}
}

func nonNegativeDuration(whenSent, whenSeen uint64) uint64 {
	var ret int64
	ret = int64(whenSeen) - int64(whenSent)
	if ret < 0 {
		ret = 0
	}
	return uint64(ret)
}

type avgEchoParams struct {
	avgLatencyNthMs      [whenSeenArrayLen]uint64
	avgLastSeenLatencyMs uint64
}

func calcAvgEchoParams(res *avgEchoParams) {
	var numSeenAll uint64
	var numSeen [whenSeenArrayLen]uint64
	var data *echoEntry

	earliest := utils.UnixMsNow() - 30*60*1000 //30min
	echoBuffer.ForEachEntry(func(entry *hashcache.CacheEntry) {
		data = entry.Data.(*echoEntry)
		if data.seen {
			numSeenAll++
			res.avgLastSeenLatencyMs += nonNegativeDuration(data.whenSent, data.whenSeenLast)
			for i := 0; i < whenSeenArrayLen; i++ {
				if data.whenSeenNth[i] != 0 {
					numSeen[i]++
					res.avgLatencyNthMs[i] += nonNegativeDuration(data.whenSent, data.whenSeenNth[i])
				}
			}
		}
	}, earliest, true)
	// averages are calculated only if enough data
	if numSeenAll > 5 {
		res.avgLastSeenLatencyMs = res.avgLastSeenLatencyMs / numSeenAll
	} else {
		res.avgLastSeenLatencyMs = 0
	}
	for i := 0; i < whenSeenArrayLen; i++ {
		if numSeen[i] > 5 {
			res.avgLatencyNthMs[i] = res.avgLatencyNthMs[i] / numSeen[i]
		} else {
			res.avgLatencyNthMs[i] = 0
		}

	}
	debugf("avgSeenLastLatencyMs = %v avgSeenMth = %v ", res.avgLastSeenLatencyMs, res.avgLatencyNthMs)
}
