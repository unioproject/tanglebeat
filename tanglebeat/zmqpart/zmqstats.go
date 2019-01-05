package zmqpart

import (
	"fmt"
	"math"
	"sync"
	"time"
)

type ZmqOutputStatsStruct struct {
	LastMin uint64 `json:"lastMin"`

	TXCount  int `json:"txCount"`
	SNCount  int `json:"snCount"`
	ConfRate int `json:"confRate"`

	ConfirmedTransferCount int    `json:"confirmedValueBundleCount"`
	ValueVolumeApprox      uint64 `json:"valueVolumeApprox"`
}

type ZmqCacheStatsStruct struct {
	SizeTXCache            string `json:"sizeTXCache"`
	SizeSNCache            string `json:"sizeSNCache"`
	SizeValueTxCache       string `json:"sizeValueTxCache"`
	SizeValueBundleCache   string `json:"sizeValueBundleCache"`
	SizeConfirmedTransfers string `json:"sizeConfirmedTransfers"`

	TXSeenOnceCount      int `json:"txSeenOnceCount"`
	SNSeenOnceCount      int `json:"snSeenOnceCount"`
	TXNonPropagationRate int `json:"txNonPropagationRate"`
	SNNonPropagationRate int `json:"snNonPropagationRate"`

	TXSeenOnceCount10min      int `json:"txSeenOnceCount10min"`
	SNSeenOnceCount10min      int `json:"snSeenOnceCount10min"`
	TXNonPropagationRate10min int `json:"txNonPropagationRate10min"`
	SNNonPropagationRate10min int `json:"snNonPropagationRate10min"`

	TXLatencySecAvg float64 `json:"txLatencySecAvg"`
	SNLatencySecAvg float64 `json:"snLatencySecAvg"`

	TXLatencySecAvg10min float64 `json:"txLatencySecAvg10min"`
	SNLatencySecAvg10min float64 `json:"snLatencySecAvg10min"`

	mutex *sync.RWMutex
}

var (
	zmqCacheStats       = &ZmqCacheStatsStruct{mutex: &sync.RWMutex{}}
	zmqOutputStatsMutex = &sync.RWMutex{}
	zmqOutputStats      = &ZmqOutputStatsStruct{}
	zmqOutputStats10min = &ZmqOutputStatsStruct{}
)

func GetZmqCacheStats() *ZmqCacheStatsStruct {
	zmqCacheStats.mutex.RLock()
	defer zmqCacheStats.mutex.RUnlock()
	ret := *zmqCacheStats
	return &ret
}

func GetOutputStats() (*ZmqOutputStatsStruct, *ZmqOutputStatsStruct) {
	zmqCacheStats.mutex.RLock()
	defer zmqCacheStats.mutex.RUnlock()
	ret := *zmqOutputStats
	ret10min := *zmqOutputStats10min
	return &ret, &ret10min
}

func InitZmqStatsCollector(refreshEverySec int) {
	go func() {
		for {
			updateZmqCacheStats()
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

func updateZmqCacheStats() {
	zmqCacheStats.mutex.Lock()
	defer zmqCacheStats.mutex.Unlock()

	var s, e int
	s, e = txcache.Size()
	zmqCacheStats.SizeTXCache = fmt.Sprintf("%v, %v", s, e)
	s, e = sncache.Size()
	zmqCacheStats.SizeSNCache = fmt.Sprintf("%v, %v", s, e)
	s, e = positiveValueTxCache.Size()
	zmqCacheStats.SizeValueTxCache = fmt.Sprintf("%v, %v", s, e)
	s, e = valueBundleCache.Size()
	zmqCacheStats.SizeValueBundleCache = fmt.Sprintf("%v, %v", s, e)
	s, e = confirmedTransfers.Size()
	zmqCacheStats.SizeConfirmedTransfers = fmt.Sprintf("%v, %v", s, e)

	// 1 hour stats
	txcacheStats := txcache.Stats(0)
	sncacheStats := sncache.Stats(0)
	zmqCacheStats.TXSeenOnceCount = txcacheStats.SeenOnce
	zmqCacheStats.SNSeenOnceCount = sncacheStats.SeenOnce

	if txcacheStats.TxCount != 0 {
		zmqCacheStats.TXNonPropagationRate = (txcacheStats.SeenOnce * 100) / txcacheStats.TxCount
	}
	if sncacheStats.TxCount != 0 {
		zmqCacheStats.SNNonPropagationRate = (sncacheStats.SeenOnce * 100) / sncacheStats.TxCount
	}

	zmqCacheStats.TXLatencySecAvg = math.Round(txcacheStats.LatencySecAvg*100) / 100
	zmqCacheStats.SNLatencySecAvg = math.Round(sncacheStats.LatencySecAvg*100) / 100

	txcacheStats10min := txcache.Stats(10 * 60 * 1000)
	sncacheStats10min := sncache.Stats(10 * 60 * 1000)
	zmqCacheStats.TXSeenOnceCount10min = txcacheStats10min.SeenOnce
	zmqCacheStats.SNSeenOnceCount10min = sncacheStats10min.SeenOnce

	if txcacheStats10min.TxCount != 0 {
		zmqCacheStats.TXNonPropagationRate10min = (txcacheStats10min.SeenOnce * 100) / txcacheStats10min.TxCount
	}
	if sncacheStats10min.TxCount != 0 {
		zmqCacheStats.SNNonPropagationRate10min = (sncacheStats10min.SeenOnce * 100) / sncacheStats10min.TxCount
	}

	zmqCacheStats.TXLatencySecAvg10min = math.Round(txcacheStats10min.LatencySecAvg*100) / 100
	zmqCacheStats.SNLatencySecAvg10min = math.Round(sncacheStats10min.LatencySecAvg*100) / 100
}

func updateZmqOutputSlowStats() {

	// all retentionPeriod stats
	var st ZmqOutputStatsStruct
	txs := txcache.Stats(0)
	st.TXCount = txs.TxCountPassed

	sns := sncache.Stats(0)
	st.SNCount = sns.TxCountPassed

	if st.TXCount != 0 {
		st.ConfRate = (st.SNCount * 100) / st.TXCount
	}
	st.ConfirmedTransferCount, st.ValueVolumeApprox = getValueConfirmationStats(0)

	// 10 min stats
	const msecBack = 10 * 60 * 1000
	var st10 ZmqOutputStatsStruct
	txs = txcache.Stats(msecBack)
	st10.TXCount = txs.TxCountPassed

	sns = sncache.Stats(msecBack)
	st10.SNCount = sns.TxCountPassed

	if st10.TXCount != 0 {
		st10.ConfRate = (st10.SNCount * 100) / st10.TXCount
	}
	st10.ConfirmedTransferCount, st10.ValueVolumeApprox = getValueConfirmationStats(msecBack)

	zmqOutputStatsMutex.Lock()

	*zmqOutputStats = st
	zmqOutputStats.LastMin = retentionPeriodSec / 60
	*zmqOutputStats10min = st10
	zmqOutputStats10min.LastMin = msecBack / (60 * 1000)

	zmqOutputStatsMutex.Unlock()
}

type latencyMetrics10min struct {
	txAvgLatencySec     float64
	txNotPropagatedPerc float64
	snAvgLatencySec     float64
	snNotPropagatedPerc float64
}

func getLatencyStats10minForMetrics(ret *latencyMetrics10min) {
	zmqCacheStats.mutex.Lock()
	defer zmqCacheStats.mutex.Unlock()

	ret.txAvgLatencySec = zmqCacheStats.TXLatencySecAvg10min
	ret.snAvgLatencySec = zmqCacheStats.SNLatencySecAvg10min
	ret.txNotPropagatedPerc = float64(zmqCacheStats.TXNonPropagationRate10min)
	ret.snNotPropagatedPerc = float64(zmqCacheStats.SNNonPropagationRate10min)
}
