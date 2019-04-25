package inputpart

import (
	"fmt"
	"github.com/unioproject/tanglebeat/lib/utils"
	"github.com/unioproject/tanglebeat/tanglebeat/cfg"
	"math"
	"sync"
	"time"
)

type ZmqOutputStatsStruct struct {
	LastMin uint64 `json:"lastMin"`

	TXCount  int     `json:"txCount"`
	SNCount  int     `json:"snCount"`
	TPS      float64 `json:"tps"`
	CTPS     float64 `json:"ctps"`
	ConfRate int     `json:"confRate"`

	ConfirmedTransferCount int    `json:"confirmedValueBundleCount"`
	ValueVolumeApprox      uint64 `json:"valueVolumeApprox"`
}

type ZmqCacheStatsStruct struct {
	SizeTXCache     string `json:"sizeTXCache"`
	SizeSNCache     string `json:"sizeSNCache"`
	SizeBundleCache string `json:"sizeBundleCache"`

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

	LastLmi       int     `json:"lastLmi"`
	LmiLatencySec float64 `json:"lmiLatencySec"`

	seenOnceRateById5Min map[byte]int

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
	zmqCacheStats.SizeTXCache = fmt.Sprintf("%v, seg=%v", e, s)
	s, e = sncache.Size()
	zmqCacheStats.SizeSNCache = fmt.Sprintf("%v, seg=%v", e, s)
	s, e = transferBundleCache.Size()
	zmqCacheStats.SizeBundleCache = fmt.Sprintf("%v, seg=%v", e, s)

	s, e = positiveValueTxCache.Size()
	zmqCacheStats.SizeValueTxCache = fmt.Sprintf("%v, seg=%v", e, s)
	s, e = valueBundleCache.Size()
	zmqCacheStats.SizeValueBundleCache = fmt.Sprintf("%v, seg=%v", e, s)
	s, e = confirmedPositiveValueTx.Size()
	zmqCacheStats.SizeConfirmedTransfers = fmt.Sprintf("%v, seg=%v", e, s)

	// 1 hour stats
	txcacheStats := txcache.Stats(0, GetTxQuorum())
	sncacheStats := sncache.Stats(0, GetSnQuorum())

	zmqCacheStats.TXSeenOnceCount = txcacheStats.SeenOnce
	zmqCacheStats.SNSeenOnceCount = sncacheStats.SeenOnce

	if txcacheStats.TxCountOlder1Min != 0 {
		zmqCacheStats.TXNonPropagationRate = (txcacheStats.SeenOnce * 100) / txcacheStats.TxCountOlder1Min
	}
	if sncacheStats.TxCountOlder1Min != 0 {
		zmqCacheStats.SNNonPropagationRate = (sncacheStats.SeenOnce * 100) / sncacheStats.TxCountOlder1Min
	}

	zmqCacheStats.TXLatencySecAvg = math.Round(txcacheStats.LatencySecAvg*100) / 100
	zmqCacheStats.SNLatencySecAvg = math.Round(sncacheStats.LatencySecAvg*100) / 100

	txcacheStats10min := txcache.Stats(10*60*1000, GetTxQuorum())
	sncacheStats10min := sncache.Stats(10*60*1000, GetSnQuorum())

	zmqCacheStats.TXSeenOnceCount10min = txcacheStats10min.SeenOnce
	zmqCacheStats.SNSeenOnceCount10min = sncacheStats10min.SeenOnce

	if txcacheStats10min.TxCountOlder1Min != 0 {
		zmqCacheStats.TXNonPropagationRate10min = (txcacheStats10min.SeenOnce * 100) / txcacheStats10min.TxCountOlder1Min
	}
	if sncacheStats10min.TxCountOlder1Min != 0 {
		zmqCacheStats.SNNonPropagationRate10min = (sncacheStats10min.SeenOnce * 100) / sncacheStats10min.TxCountOlder1Min
	}

	txcacheStats5min := txcache.Stats(5*60*1000, GetTxQuorum())
	zmqCacheStats.seenOnceRateById5Min = txcacheStats5min.SeenOnceRateById

	zmqCacheStats.TXLatencySecAvg10min = math.Round(txcacheStats10min.LatencySecAvg*100) / 100
	zmqCacheStats.SNLatencySecAvg10min = math.Round(sncacheStats10min.LatencySecAvg*100) / 100

	zmqCacheStats.LastLmi, zmqCacheStats.LmiLatencySec = getLmiStats()
}

func getSeenOnceRate5to1Min(id byte) int {
	zmqCacheStats.mutex.RLock()
	defer zmqCacheStats.mutex.RUnlock()
	ret, ok := zmqCacheStats.seenOnceRateById5Min[id]
	if !ok {
		ret = 0
	}
	return ret
}

func updateZmqOutputSlowStats() {

	// all retentionPeriod stats
	retentionPeriodSec := uint64(cfg.Config.RetentionPeriodMin) * 60
	var st ZmqOutputStatsStruct
	txs := txcache.Stats(retentionPeriodSec*1000, GetTxQuorum())
	st.TXCount = txs.TxCountPassed

	sns := sncache.Stats(retentionPeriodSec*1000, GetSnQuorum())
	st.SNCount = sns.TxCountPassed

	secPassed := float64((utils.UnixMsNow() - txs.EarliestSeen) / 1000)

	if secPassed != 0 {
		st.TPS = float64(st.TXCount) / secPassed
		st.TPS = math.Round(st.TPS*100) / 100
		st.CTPS = float64(st.SNCount) / secPassed
		st.CTPS = math.Round(st.CTPS*100) / 100
	}

	if st.TXCount != 0 {
		st.ConfRate = (st.SNCount * 100) / st.TXCount
	}
	st.ConfirmedTransferCount, _, st.ValueVolumeApprox = getValueConfirmationStats(0)

	// 10 min stats
	const secBack10min = 10 * 60
	var st10 ZmqOutputStatsStruct
	txs10 := txcache.Stats(secBack10min*1000, GetTxQuorum())
	st10.TXCount = txs10.TxCountPassed

	sns10 := sncache.Stats(secBack10min*1000, GetSnQuorum())
	st10.SNCount = sns10.TxCountPassed

	secPassed10 := float64((utils.UnixMsNow() - txs10.EarliestSeen) / 1000)

	if secPassed10 != 0 {
		st10.TPS = float64(st10.TXCount) / secPassed10
		st10.TPS = math.Round(st10.TPS*100) / 100
		st10.CTPS = float64(st10.SNCount) / secPassed10
		st10.CTPS = math.Round(st10.CTPS*100) / 100
		//debugf("+++++++++++++ secPassed10 = %v st = %+v", secPassed10, st10)
	}

	if st10.TXCount != 0 {
		st10.ConfRate = (st10.SNCount * 100) / st10.TXCount
	}
	st10.ConfirmedTransferCount, _, st10.ValueVolumeApprox = getValueConfirmationStats(secBack10min * 1000)

	zmqOutputStatsMutex.Lock() //----
	*zmqOutputStats = st
	zmqOutputStats.LastMin = retentionPeriodSec / 60
	*zmqOutputStats10min = st10
	zmqOutputStats10min.LastMin = secBack10min / 60
	zmqOutputStatsMutex.Unlock() //----
}

type latencyMetrics10min struct {
	txAvgLatencySec     float64
	txNotPropagatedPerc float64
	snAvgLatencySec     float64
	snNotPropagatedPerc float64
}

func getLatencyStats10minForMetrics(ret *latencyMetrics10min) {
	zmqCacheStats.mutex.RLock()
	defer zmqCacheStats.mutex.RUnlock()

	ret.txAvgLatencySec = zmqCacheStats.TXLatencySecAvg10min
	ret.snAvgLatencySec = zmqCacheStats.SNLatencySecAvg10min
	ret.txNotPropagatedPerc = float64(zmqCacheStats.TXNonPropagationRate10min)
	ret.snNotPropagatedPerc = float64(zmqCacheStats.SNNonPropagationRate10min)
}
