package zmqpart

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/hashcache"
	"github.com/op/go-logging"
	. "github.com/prometheus/client_golang/prometheus"
	"time"
)

var (
	zmqMetricsConfirmedValueTxCounter      Counter
	zmqMetricsConfirmedValueTxTotalCounter Counter

	zmqMetricsConfirmedValueTxSubTiCounter     Counter
	zmqMetricsConfirmedValueTxSubGiCounter     Counter
	zmqMetricsConfirmedValueTxLastIndexCounter Counter

	zmqMetricsConfirmedValueTxSubTiTotalCounter        Counter
	zmqMetricsConfirmedValueTxSubGiTotalCounter        Counter
	zmqMetricsConfirmedValueTxNotLastIndexTotalCounter Counter

	zmqMetricsConfirmedValueBundleCounter Counter

	metricsMiotaPriceUSD Gauge

	zmqMetricsTxCounterCompound  Counter
	zmqMetricsCtxCounterCompound Counter

	zmqMetricsLatencyTXAvg        Gauge
	zmqMetricsLatencyTXMax        Gauge
	zmqMetricsNotPropagatedPercTX Gauge

	zmqMetricsLatencySNAvg        Gauge
	zmqMetricsLatencySNMax        Gauge
	zmqMetricsNotPropagatedPercSN Gauge

	//zmqMetricsTxCounterVec  *CounterVec
	//zmqMetricsCtxCounterVec *CounterVec

)

func init() {
	//------------------------------- value tx
	zmqMetricsConfirmedValueTxCounter = NewCounter(CounterOpts{
		Name: "tanglebeat_confirmed_value_tx_counter",
		Help: "Counter of confirmed value transactions",
	})
	MustRegister(zmqMetricsConfirmedValueTxCounter)

	zmqMetricsConfirmedValueTxTotalCounter = NewCounter(CounterOpts{
		Name: "tanglebeat_confirmed_value_tx_total_counter",
		Help: "Counter total value confirmed value transactions",
	})
	MustRegister(zmqMetricsConfirmedValueTxTotalCounter)

	zmqMetricsConfirmedValueTxSubTiCounter = NewCounter(CounterOpts{
		Name: "tanglebeat_confirmed_value_tx_subti_counter",
		Help: "Counter confirmed value sub-tiota transactions",
	})
	MustRegister(zmqMetricsConfirmedValueTxSubTiCounter)

	zmqMetricsConfirmedValueTxSubGiCounter = NewCounter(CounterOpts{
		Name: "tanglebeat_confirmed_value_tx_subgi_counter",
		Help: "Counter value sub Tiota transactions",
	})
	MustRegister(zmqMetricsConfirmedValueTxSubGiCounter)

	zmqMetricsConfirmedValueTxLastIndexCounter = NewCounter(CounterOpts{
		Name: "tanglebeat_confirmed_value_tx_lastindex_counter",
		Help: "Counter value transactions last in bundle",
	})
	MustRegister(zmqMetricsConfirmedValueTxLastIndexCounter)

	zmqMetricsConfirmedValueTxSubTiTotalCounter = NewCounter(CounterOpts{
		Name: "tanglebeat_confirmed_value_tx_subti_total_counter",
		Help: "Total of value transactions < Ti",
	})
	MustRegister(zmqMetricsConfirmedValueTxSubTiTotalCounter)

	zmqMetricsConfirmedValueTxSubGiTotalCounter = NewCounter(CounterOpts{
		Name: "tanglebeat_confirmed_value_tx_subgi_total_counter",
		Help: "Total of value transactions < Gi",
	})
	MustRegister(zmqMetricsConfirmedValueTxSubGiTotalCounter)

	zmqMetricsConfirmedValueTxNotLastIndexTotalCounter = NewCounter(CounterOpts{
		Name: "tanglebeat_confirmed_value_tx_notlastindex_total_counter",
		Help: "Total of value transactions not last in bundle",
	})
	MustRegister(zmqMetricsConfirmedValueTxNotLastIndexTotalCounter)

	zmqMetricsConfirmedValueBundleCounter = NewCounter(CounterOpts{
		Name: "tanglebeat_confirmed_bundle_counter",
		Help: "Number of confirmed positive value bundles",
	})
	MustRegister(zmqMetricsConfirmedValueBundleCounter)

	//---------------------------------------------- value tx end

	//---------------------------------------------- latency begin
	zmqMetricsLatencyTXAvg = NewGauge(GaugeOpts{
		Name: "tanglebeat_latency_tx_avg",
		Help: "Average relative latency of transaction messages",
	})
	MustRegister(zmqMetricsLatencyTXAvg)

	zmqMetricsLatencyTXMax = NewGauge(GaugeOpts{
		Name: "tanglebeat_latency_tx_max",
		Help: "Maximum relative latency of transaction messages",
	})
	MustRegister(zmqMetricsLatencyTXMax)

	zmqMetricsNotPropagatedPercTX = NewGauge(GaugeOpts{
		Name: "tanglebeat_not_propagated_tx_perc",
		Help: "Percentage of tx messages not propagated",
	})
	MustRegister(zmqMetricsNotPropagatedPercTX)

	zmqMetricsLatencySNAvg = NewGauge(GaugeOpts{
		Name: "tanglebeat_latency_confirm_avg",
		Help: "Average relative latency of confirmation messages",
	})
	MustRegister(zmqMetricsLatencySNAvg)

	zmqMetricsLatencySNMax = NewGauge(GaugeOpts{
		Name: "tanglebeat_latency_confirm_max",
		Help: "Maximum relative latency of confirmation messages",
	})
	MustRegister(zmqMetricsLatencySNMax)

	zmqMetricsNotPropagatedPercSN = NewGauge(GaugeOpts{
		Name: "tanglebeat_not_propagated_confirm_perc",
		Help: "Percentage of confirmation messages not propagated",
	})
	MustRegister(zmqMetricsNotPropagatedPercSN)
	//---------------------------------------------- latency end

	zmqMetricsTxCounterCompound = NewCounter(CounterOpts{
		Name: "tanglebeat_tx_counter_compound",
		Help: "Transaction counter.",
	})
	MustRegister(zmqMetricsTxCounterCompound)

	zmqMetricsCtxCounterCompound = NewCounter(CounterOpts{
		Name: "tanglebeat_ctx_counter_compound",
		Help: "Confirmed transaction counter.",
	})
	MustRegister(zmqMetricsCtxCounterCompound)

	//zmqMetricsTxCounterVec = NewCounterVec(CounterOpts{
	//	Name: "tanglebeat_tx_counter_vec",
	//	Help: "Transaction counter labeled by host.",
	//}, []string{"host"})
	//MustRegister(zmqMetricsTxCounterVec)
	//
	//zmqMetricsCtxCounterVec = NewCounterVec(CounterOpts{
	//	Name: "tanglebeat_ctx_counter_vec",
	//	Help: "Confirmed transaction counter labeled by host.",
	//}, []string{"host"})
	//MustRegister(zmqMetricsCtxCounterVec)

	metricsMiotaPriceUSD = NewGauge(GaugeOpts{
		Name: "tanglebeat_miota_price_usd",
		Help: "Price USD/MIOTA, labeled by source",
	})
	MustRegister(metricsMiotaPriceUSD)
	startCollectingMiotaPrice(nil)
}

const (
	Miota = 1000000
	Giota = Miota * 1000
	Tiota = Giota * 1000
)

func updateConfirmedValueTxMetrics(value uint64, lastInBundle bool) {
	zmqMetricsConfirmedValueTxCounter.Inc()
	zmqMetricsConfirmedValueTxTotalCounter.Add(float64(value))

	if value < Giota {
		zmqMetricsConfirmedValueTxSubGiCounter.Inc()
		zmqMetricsConfirmedValueTxSubGiTotalCounter.Add(float64(value))
	}

	if value < Tiota {
		zmqMetricsConfirmedValueTxSubTiCounter.Inc()
		zmqMetricsConfirmedValueTxSubTiTotalCounter.Add(float64(value))
	}

	if lastInBundle {
		zmqMetricsConfirmedValueTxLastIndexCounter.Inc()
	} else {
		zmqMetricsConfirmedValueTxNotLastIndexTotalCounter.Add(float64(value))
	}
}

func updateConfirmedValueBundleMetrics() {
	zmqMetricsConfirmedValueBundleCounter.Inc()
}

func updateCompoundMetrics(msgtype string) {
	switch msgtype {
	case "tx":
		zmqMetricsTxCounterCompound.Inc()
	case "sn":
		zmqMetricsCtxCounterCompound.Inc()
	}
}

//func updateVecMetrics(msgtype string, host string) {
//	switch msgtype {
//	case "tx":
//		zmqMetricsTxCounterVec.With(Labels{"host": host}).Inc()
//	case "sn":
//		zmqMetricsCtxCounterVec.With(Labels{"host": host}).Inc()
//	}
//}

func startCollectingMiotaPrice(localLog *logging.Logger) {
	var price float64
	var err error
	go func() {
		for {
			price, err = utils.GetMiotaPriceUSD()
			if err == nil && price > 0 {
				metricsMiotaPriceUSD.Set(price)
				if localLog != nil {
					localLog.Debugf("Coincap price = %v USD/MIOTA", price)
				}
				time.Sleep(30 * time.Second)
			} else {
				if localLog != nil {
					localLog.Errorf("Can't Get MIOTA price from Coincap: %v", err)
				}
				time.Sleep(10 * time.Second)
			}
		}
	}()
}

const latencyMsecBack = 10 * 60 * 1000

func startCollectingLatencyMetrics(txc *hashcache.HashCacheBase, snc *hashCacheSN) {
	go func() {
		for {
			time.Sleep(10 * time.Second)

			txcSats := txc.Stats(latencyMsecBack)
			zmqMetricsLatencyTXAvg.Set(txcSats.LatencySecAvg)
			zmqMetricsLatencyTXMax.Set(txcSats.LatencySecMax)
			zmqMetricsNotPropagatedPercTX.Set(float64(txcSats.NumNoVisitPerc))

			sncStats := snc.Stats(latencyMsecBack)
			zmqMetricsLatencySNAvg.Set(sncStats.LatencySecAvg)
			zmqMetricsLatencySNMax.Set(sncStats.LatencySecMax)
			zmqMetricsNotPropagatedPercSN.Set(float64(sncStats.NumNoVisitPerc))
		}
	}()
}
