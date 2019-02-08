package zmqpart

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
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
	zmqMetricsNotPropagatedPercTX Gauge

	zmqMetricsLatencySNAvg        Gauge
	zmqMetricsNotPropagatedPercSN Gauge

	//zmqMetricsTxCounterVec  *CounterVec
	//zmqMetricsCtxCounterVec *CounterVec

	echoNotSeenPerc         Gauge
	echoMetricsAvgFirstSeen Gauge
	echoMetricsAvgLastSeen  Gauge

	lmConfRate5minMetrics  Gauge
	lmConfRate10minMetrics Gauge
	lmConfRate15minMetrics Gauge
	lmConfRate30minMetrics Gauge
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

	echoNotSeenPerc = NewGauge(GaugeOpts{
		Name: "tanglebeat_echo_silence",
		Help: "Average echo silence %",
	})
	MustRegister(echoNotSeenPerc)

	echoMetricsAvgFirstSeen = NewGauge(GaugeOpts{
		Name: "tanglebeat_echo_first",
		Help: "Average msec first echo",
	})
	MustRegister(echoMetricsAvgFirstSeen)

	echoMetricsAvgLastSeen = NewGauge(GaugeOpts{
		Name: "tanglebeat_echo_last",
		Help: "Average msec last echo",
	})
	MustRegister(echoMetricsAvgLastSeen)

	//--------------------------------------------------
	lmConfRate5minMetrics = NewGauge(GaugeOpts{
		Name: "tanglebeat_lm_conf_rate_5min",
		Help: "Average conf rate by Luca Moser",
	})
	MustRegister(lmConfRate5minMetrics)

	lmConfRate10minMetrics = NewGauge(GaugeOpts{
		Name: "tanglebeat_lm_conf_rate_10min",
		Help: "Average conf rate by Luca Moser",
	})
	MustRegister(lmConfRate10minMetrics)

	lmConfRate15minMetrics = NewGauge(GaugeOpts{
		Name: "tanglebeat_lm_conf_rate_15min",
		Help: "Average conf rate by Luca Moser",
	})
	MustRegister(lmConfRate15minMetrics)

	lmConfRate30minMetrics = NewGauge(GaugeOpts{
		Name: "tanglebeat_lm_conf_rate_30min",
		Help: "Average conf rate by Luca Moser",
	})
	MustRegister(lmConfRate30minMetrics)

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

func updateEchoMetrics(percNotSeen, avgSeenFirstMs, avgSeenLastMs uint64) {
	if percNotSeen != 0 {
		echoNotSeenPerc.Set(float64(percNotSeen))
	}
	if avgSeenFirstMs != 0 {
		echoMetricsAvgFirstSeen.Set(float64(avgSeenFirstMs))
	}
	if avgSeenLastMs != 0 {
		echoMetricsAvgLastSeen.Set(float64(avgSeenLastMs))
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

func startCollectingLatencyMetrics() {
	var lm latencyMetrics10min
	go func() {
		for {
			time.Sleep(5 * time.Second)

			getLatencyStats10minForMetrics(&lm)

			zmqMetricsLatencyTXAvg.Set(lm.txAvgLatencySec)
			zmqMetricsNotPropagatedPercTX.Set(lm.txNotPropagatedPerc)

			zmqMetricsLatencySNAvg.Set(lm.snAvgLatencySec)
			zmqMetricsNotPropagatedPercSN.Set(lm.snNotPropagatedPerc)
		}
	}()
}

func startCollectingLMConfRate() {
	go func() {
		for {
			time.Sleep(10 * time.Second)

			valmap, err := GetLMConfRate()
			if err != nil {
				errorf("Error while collecting LM conf rate metrics: %v", err)
				continue
			}

			lmConfRate5minMetrics.Set(valmap["avg_5"])
			lmConfRate10minMetrics.Set(valmap["avg_10"])
			lmConfRate15minMetrics.Set(valmap["avg_15"])
			lmConfRate30minMetrics.Set(valmap["avg_30"])

			debugf("LM conf rate: avg_5 = %v%%, avg_10 = %v%% avg_15 = %v%% avg_30 = %v%%",
				valmap["avg_5"], valmap["avg_10"], valmap["avg_15"], valmap["avg_30"])
		}
	}()
}
