package zmqpart

import (
	"github.com/lunfardo314/tanglebeat/tanglebeat/hashcache"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type ZmqOutputStatsStruct struct {
	TXout                 uint64 `json:"txOut"`
	SNout                 uint64 `json:"snOut"`
	ValuePositiveTx       uint64 `json:"valuePositiveTxOut"`
	ValueNegativeTx       uint64 `json:"valueNegativeTxOut"`
	ValueZeroTx           uint64 `json:"valueZeroOutTx"`
	ValuePositiveTotal    uint64 `json:"valuePositiveTotalOut"`
	ValueNegativeTotal    uint64 `json:"valueNegativeTotalOut"`
	ConfirmedValueTx      uint64 `json:"confirmedValueTx"`
	ConfirmedValueTxTotal uint64 `json:"confirmedValueTxTotal"`

	TXNumseg        int     `json:"tx_numseg"`
	TXNumtx         int     `json:"tx_numtx"`
	TXNumNoVisit    int     `json:"tx_numNoVisit"`
	TXLatencySecMax float64 `json:"tx_latencySecMax"`
	TXLatencySecAvg float64 `json:"tx_latencySecAvg"`

	SNNumseg        int     `json:"sn_numseg"`
	SNNumtx         int     `json:"sn_numtx"`
	SNNumNoVisit    int     `json:"sn_numNoVisit"`
	SNLatencySecMax float64 `json:"sn_latencySecMax"`
	SNLatencySecAvg float64 `json:"sn_latencySecAvg"`

	ConfBundleNumseg       int `json:"confBundleNumseg"`
	ConfBundleNumhashes    int `json:"confBundleNumhashes"`
	ConfBundleNumconfirmed int `json:"confBundleNumconfirmed"`

	MemAllocMB   float32 `json:"memAllocMB"`
	NumGoroutine int     `json:"numGoroutine"`
	mutex        *sync.Mutex
}

var ZmqStats *ZmqOutputStatsStruct

func init() {
	ZmqStats = &ZmqOutputStatsStruct{
		mutex: &sync.Mutex{},
	}
	go func() {
		for {
			time.Sleep(5 * time.Second)
			ZmqStats.updateSlowStats()
		}
	}()
}

func abs(n int) uint64 {
	if n < 0 {
		return uint64(-n)
	}
	return uint64(n)
}

func (glb *ZmqOutputStatsStruct) updateMsgStats(msg []string) {
	glb.mutex.Lock()
	defer glb.mutex.Unlock()
	switch msg[0] {
	case "tx":
		glb.TXout++
		if len(msg) >= 4 {
			value, err := strconv.Atoi(msg[3])
			if err == nil {
				switch {
				case value < 0:
					glb.ValueNegativeTx++
					glb.ValueNegativeTotal += abs(value)
				case value == 0:
					glb.ValueZeroTx++
				case value > 0:
					glb.ValuePositiveTx++
					glb.ValuePositiveTotal += uint64(value)
				}
			}
		}
	case "sn":
		glb.SNout++
	}
}

const min10ms = 10 * 60 * 1000

func (glb *ZmqOutputStatsStruct) updateSlowStats() {
	glb.mutex.Lock()
	defer glb.mutex.Unlock()

	txs := txcache.Stats(0)
	glb.TXNumseg = txs.Numseg
	glb.TXNumtx = txs.Numtx
	glb.TXNumNoVisit = txs.NumNoVisit
	glb.TXLatencySecMax = txs.LatencySecMax
	glb.TXLatencySecAvg = txs.LatencySecAvg

	sns := sncache.Stats(0)
	glb.SNNumseg = sns.Numseg
	glb.SNNumtx = sns.Numtx
	glb.SNNumNoVisit = sns.NumNoVisit
	glb.SNLatencySecMax = sns.LatencySecMax
	glb.SNLatencySecAvg = sns.LatencySecAvg

	vbs := valueBundleCache.Stats(0) // count all of it
	var confbundles int
	var confirmed *bool

	valueBundleCache.Lock()
	valueBundleCache.ForEachEntry(func(entry *hashcache.CacheEntry2) {
		confirmed, _ = entry.Data.(*bool)
		if *confirmed {
			confbundles++
		}
	})
	valueBundleCache.Unlock()

	glb.ConfBundleNumseg = vbs.Numseg
	glb.ConfBundleNumhashes = vbs.Numtx
	glb.ConfBundleNumconfirmed = confbundles

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	glb.MemAllocMB = float32(mem.Alloc/1024) / 1024
	glb.NumGoroutine = runtime.NumGoroutine()
}

func (glb *ZmqOutputStatsStruct) updateConfirmedValueTxStats(value uint64) {
	glb.mutex.Lock()
	defer glb.mutex.Unlock()
	glb.ConfirmedValueTx++
	glb.ConfirmedValueTxTotal += value
}

func (glb *ZmqOutputStatsStruct) GetCopy() ZmqOutputStatsStruct {
	glb.mutex.Lock()
	defer glb.mutex.Unlock()
	ret := *glb
	ret.mutex = nil
	return ret
}
