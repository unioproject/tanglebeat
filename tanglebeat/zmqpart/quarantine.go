package zmqpart

import (
	"github.com/lunfardo314/tanglebeat/lib/ebuffer"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/cfg"
	"github.com/lunfardo314/tanglebeat/tanglebeat/hashcache"
	"sync"
	"time"
)

const (
	quarantineBufferSegmentDurationSec = 15
	quarantineBufferRetentionMin       = 3
)

func (r *zmqRoutine) setQuarantined(quarantined bool) {
	r.Lock()
	defer r.Unlock()
	if r.quarantined == quarantined {
		return // no action
	}
	if quarantined {
		r.quarantinedTx = ebuffer.NewEventTsWithDataExpiringBuffer(
			"quarantineBuffer-"+r.uri, quarantineBufferSegmentDurationSec, quarantineBufferRetentionMin*60)
	} else {
		r.quarantinedTx = nil // release, not needed
	}
	r.quarantined = quarantined
}

func (r *zmqRoutine) startQuaratineRoutine() func() {
	uri := r.GetUri()
	infof("Started quarantine routine for %v", uri)
	chCancel := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-chCancel:
				return
			case <-time.After(30 * time.Second):
			}
			if r.isQuarantined() {
				if r.toBeUnquarantined() {
					r.setQuarantined(false)
					infof("++++++ Unquarantined %v", r.GetUri())
				}
			} else {
				if r.toBeQuarantined() {
					r.setQuarantined(true)
					infof("++++++ Quarantined %v for at least 5 minutes", r.GetUri())
				}
			}
		}
	}()
	return func() {
		close(chCancel)
		wg.Wait()
		infof("Stopped quarantine routine for %v", uri)
	}
}

func (r *zmqRoutine) isQuarantined() bool {
	r.RLock()
	defer r.RUnlock()
	return r.quarantined
}

func (r *zmqRoutine) toBeQuarantined() bool {
	if r.isQuarantined() {
		return false
	}
	stats := r.getStats()
	//return stats.timeIntervalSec10min >= 5*60 && stats.SeenOnceRate > cfg.Config.QuarantineStartThreshold
	return stats.SeenOnceRate > cfg.Config.QuarantineStartThreshold
}

func (r *zmqRoutine) toBeUnquarantined() bool {
	if !r.isQuarantined() {
		return false
	}
	_, numEntries := r.quarantinedTx.Size()
	if numEntries == 0 {
		return true // ?????
	}
	numSeenOnce, earliest := r.calcSeenOnceCountQuarantined()
	if utils.SinceUnixMs(earliest) < 5*60*1000 {
		// minimum 5 min under quarantine
		return false
	}
	seenOncePerc := uint64((numSeenOnce * 100) / numEntries)
	return seenOncePerc < cfg.Config.QuarantineStopThreshold
}

func (r *zmqRoutine) calcSeenOnceCountQuarantined() (int, uint64) {
	var hash string
	var ret int
	var entry hashcache.CacheEntry
	earliest := r.quarantinedTx.ForEachEntry(func(ts uint64, data interface{}) bool {
		hash = data.(string)
		if !txcache.FindNoTouch(hash, &entry) {
			ret++
		}
		return true
	}, 0, true)
	return ret, earliest
}

func (r *zmqRoutine) toQuarantine(msgData []byte, msgSplit []string) {
	r.Lock()
	defer r.Unlock()
	if !r.quarantined {
		return
	}
	if msgSplit[0] != "tx" || len(msgSplit) < 2 {
		return
	}
	// using short hash produced by txcache because will search later by it
	r.quarantinedTx.RecordTS(txcache.ShortHash(msgSplit[1]))
}
