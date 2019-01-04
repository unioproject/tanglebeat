package zmqpart

import (
	"fmt"
	"math"
	"sync"
	"time"
)

type ZmqOutputStatsStruct struct {
	LastMin uint64 `json:"lastMin"`

	TXCount         int     `json:"txCount"`
	TXSeenOnceCount int     `json:"txSeenOnceCount"`
	TXLatencySecAvg float64 `json:"txLatencySecAvg"`

	SNCount         int     `json:"snCount"`
	SNSeenOnceCount int     `json:"snSeenOnceCount"`
	SNLatencySecAvg float64 `json:"snLatencySecAvg"`

	ConfirmedTransferCount int    `json:"ConfirmedValueBundleCount"`
	ValueVolumeApprox      uint64 `json:"valueVolumeApprox"`
}

type ZmqRuntimeStatsStruct struct {
	SizeTXCache          string `json:"sizeTXCache"`
	SizeSNCache          string `json:"sizeSNCache"`
	SizeValueTxCache     string `json:"sizeValueTxCache"`
	SizeValueBundleCache string `json:"sizeValueBundleCache"`
	mutex                *sync.RWMutex
}

var (
	zmqRuntimeStats     = &ZmqRuntimeStatsStruct{mutex: &sync.RWMutex{}}
	zmqOutputStatsMutex = &sync.RWMutex{}
	zmqOutputStats      = &ZmqOutputStatsStruct{}
	zmqOutputStats10min = &ZmqOutputStatsStruct{}
)

func GetRuntimeStats() *ZmqRuntimeStatsStruct {
	zmqRuntimeStats.mutex.RLock()
	defer zmqRuntimeStats.mutex.RUnlock()
	ret := *zmqRuntimeStats
	return &ret
}

func GetOutputStats() (*ZmqOutputStatsStruct, *ZmqOutputStatsStruct) {
	zmqRuntimeStats.mutex.RLock()
	defer zmqRuntimeStats.mutex.RUnlock()
	ret := *zmqOutputStats
	ret10min := *zmqOutputStats10min
	return &ret, &ret10min
}

func InitZmqStatsCollector(refreshEverySec int) {
	go func() {
		for {
			updateZmqRuntimeStats()
			time.Sleep(time.Duration(refreshEverySec) * time.Second)
		}
	}()
	go func() {
		for {
			updateZmqOutputSlowStats()
			time.Sleep(time.Duration(refreshEverySec) * time.Second)
		}
	}()
}

func updateZmqRuntimeStats() {
	zmqRuntimeStats.mutex.Lock()
	defer zmqRuntimeStats.mutex.Unlock()

	var s, e int
	s, e = txcache.Size()
	zmqRuntimeStats.SizeTXCache = fmt.Sprintf("%v, %v", s, e)
	s, e = sncache.Size()
	zmqRuntimeStats.SizeSNCache = fmt.Sprintf("%v, %v", s, e)
	s, e = positiveValueTxCache.Size()
	zmqRuntimeStats.SizeValueTxCache = fmt.Sprintf("%v, %v", s, e)
	s, e = valueBundleCache.Size()
	zmqRuntimeStats.SizeValueBundleCache = fmt.Sprintf("%v, %v", s, e)
}

func updateZmqOutputSlowStats() {

	// all retentionPeriod stats
	var st ZmqOutputStatsStruct
	txs := txcache.Stats(0)
	st.TXCount = txs.TxCount
	st.TXSeenOnceCount = txs.SeenOnce
	st.TXLatencySecAvg = math.Round(txs.LatencySecAvg*100) / 100

	sns := sncache.Stats(0)
	st.SNCount = sns.TxCount
	st.SNSeenOnceCount = sns.SeenOnce
	st.SNLatencySecAvg = math.Round(sns.LatencySecAvg*100) / 100

	st.ConfirmedTransferCount, st.ValueVolumeApprox = getValueConfirmationStats(0)

	// 10 min stats
	const msecBack = 10 * 60 * 1000
	var st10 ZmqOutputStatsStruct
	txs = txcache.Stats(msecBack)
	st10.TXCount = txs.TxCount
	st10.TXSeenOnceCount = txs.SeenOnce
	st10.TXLatencySecAvg = math.Round(txs.LatencySecAvg*100) / 100

	sns = sncache.Stats(msecBack)
	st10.SNCount = sns.TxCount
	st10.SNSeenOnceCount = sns.SeenOnce
	st10.SNLatencySecAvg = math.Round(sns.LatencySecAvg*100) / 100

	st10.ConfirmedTransferCount, st.ValueVolumeApprox = getValueConfirmationStats(msecBack)

	zmqOutputStatsMutex.Lock()

	*zmqOutputStats = st
	zmqOutputStats.LastMin = retentionPeriodSec / 60
	*zmqOutputStats10min = st10
	zmqOutputStats10min.LastMin = msecBack / (60 * 1000)

	zmqOutputStatsMutex.Unlock()
}

type latencyMetrics10min struct {
	txAvgLatencySec     float64
	txNotPropagatedPerc int
	snAvgLatencySec     float64
	snNotPropagatedPerc int
}

func getLatencyStats10minForMetrics(ret *latencyMetrics10min) {
	zmqOutputStatsMutex.RLock()
	defer zmqOutputStatsMutex.RUnlock()

	ret.txAvgLatencySec = zmqOutputStats10min.TXLatencySecAvg
	ret.snAvgLatencySec = zmqOutputStats10min.SNLatencySecAvg

	ret.txAvgLatencySec = ret.txAvgLatencySec
	ret.snAvgLatencySec = ret.snAvgLatencySec

	ret.txNotPropagatedPerc = 0
	if zmqOutputStats10min.TXCount != 0 {
		ret.txNotPropagatedPerc = (zmqOutputStats10min.TXSeenOnceCount * 100) / zmqOutputStats10min.TXCount
	}
	ret.snNotPropagatedPerc = 0
	if zmqOutputStats10min.SNCount != 0 {
		ret.snNotPropagatedPerc = (zmqOutputStats10min.SNSeenOnceCount * 100) / zmqOutputStats10min.SNCount
	}
}
