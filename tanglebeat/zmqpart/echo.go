package zmqpart

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/hashcache"
	"time"
)

const (
	echoBufferHashLen            = 12
	echoBufferSegmentDurationSec = 60
	echoBufferRetentionPeriodSec = 30 * 60
)

type echoEntry struct {
	whenSent      uint64
	seen          bool
	whenSeenFirst uint64
	whenSeenLast  uint64
}

var (
	echoBuffer *hashcache.HashCacheBase
)

func init() {
	echoBuffer = hashcache.NewHashCacheBase(
		"echoBuffer", echoBufferHashLen, echoBufferSegmentDurationSec, echoBufferRetentionPeriodSec)
	go func() {
		debugf("Started echo latency calculation routine")
		var percNotSeen, avgSeenFirstMs, avgSeenLastMs uint64
		for {
			time.Sleep(10 * time.Second)
			percNotSeen, avgSeenFirstMs, avgSeenLastMs = calcAvgEchoParams()
			updateEchoMetrics(percNotSeen, avgSeenFirstMs, avgSeenLastMs)
		}
	}()
}

func TxSentForEcho(txhash string, ts uint64) {
	var entry hashcache.CacheEntry

	ee := echoEntry{
		whenSent: ts,
	}
	if txcache.FindNoTouch(txhash, &entry) {
		ee.whenSeenFirst = entry.FirstSeen
		ee.whenSeenLast = entry.LastSeen
		ee.seen = true
	}
	echoBuffer.SeenHashBy(txhash, 0, &ee, nil)
	debugf("++++++Promo tx waiting for echo: %v...", txhash[:12])
}

// it is called for each tx message
func checkForEcho(txhash string, ts uint64) {
	var entry hashcache.CacheEntry
	echoBuffer.Lock()
	defer echoBuffer.Unlock()

	if echoBuffer.FindNoTouch__(txhash, &entry) {
		d := entry.Data.(*echoEntry)
		if d.seen {
			debugf("+++++++ Promo tx echo in %v msec. %v..", ts-d.whenSeenFirst, txhash[:12])
			d.whenSeenLast = ts
		} else {
			d.whenSeenFirst = ts
			d.whenSeenLast = ts
			d.seen = true
		}
	}
}

func calcAvgEchoParams() (uint64, uint64, uint64) {
	var numAll, numSeen, avgSeenFirstMs, avgSeenLastMs uint64
	var data *echoEntry
	earliest := utils.UnixMsNow() - 30*60*1000 //30min
	echoBuffer.ForEachEntry(func(entry *hashcache.CacheEntry) {
		data = entry.Data.(*echoEntry)
		numAll++
		if data.seen {
			numSeen++
		}
		avgSeenFirstMs += data.whenSeenFirst - data.whenSent
		avgSeenLastMs += data.whenSeenLast - data.whenSent
	}, earliest, true)
	if numSeen != 0 {
		avgSeenFirstMs = avgSeenFirstMs / numSeen
		avgSeenLastMs = avgSeenLastMs / numSeen
	}
	percNotSeen := uint64(0)
	if numAll != 0 {
		percNotSeen = (numSeen * 100) / numAll
	}
	return percNotSeen, avgSeenFirstMs, avgSeenLastMs
}
