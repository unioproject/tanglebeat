package inputpart

import (
	"github.com/op/go-logging"
	. "github.com/prometheus/client_golang/prometheus"
	"github.com/unioproject/tanglebeat/lib/utils"
	"time"
)

var (
	zmqMetricsConfirmedValueTxCounter      Counter
	zmqMetricsConfirmedValueTxTotalCounter Counter

	zmqMetricsConfirmedValueTxLastIndexCounter Counter

	zmqMetricsConfirmedValueTxNotLastIndexTotalCounter Counter

	zmqMetricsTransferVolumeCounter Counter // new
	zmqMetricsTransferCounter       Counter // new

	zmqMetricsConfirmedValueBundleCounter Counter

	metricsMiotaPriceUSD Gauge

	zmqMetricsTxCounterCompound  Counter
	zmqMetricsCtxCounterCompound Counter

	zmqMetricsLatencyTXAvg        Gauge
	zmqMetricsNotPropagatedPercTX Gauge

	zmqMetricsLatencySNAvg        Gauge
	zmqMetricsNotPropagatedPercSN Gauge

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

	zmqMetricsConfirmedValueTxLastIndexCounter = NewCounter(CounterOpts{
		Name: "tanglebeat_confirmed_value_tx_lastindex_counter",
		Help: "Counter value transactions last in bundle",
	})
	MustRegister(zmqMetricsConfirmedValueTxLastIndexCounter)

	zmqMetricsConfirmedValueTxNotLastIndexTotalCounter = NewCounter(CounterOpts{
		Name: "tanglebeat_confirmed_value_tx_notlastindex_total_counter",
		Help: "Total of value transactions not last in bundle",
	})
	MustRegister(zmqMetricsConfirmedValueTxNotLastIndexTotalCounter)

	zmqMetricsTransferVolumeCounter = NewCounter(CounterOpts{
		Name: "tanglebeat_transfer_volume_counter",
		Help: "Approximation of the total transfer value",
	})
	MustRegister(zmqMetricsTransferVolumeCounter)

	zmqMetricsTransferCounter = NewCounter(CounterOpts{
		Name: "tanglebeat_transfer_counter",
		Help: "Number of confirmed transfers",
	})
	MustRegister(zmqMetricsTransferCounter)

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

func updateTransferVolumeMetrics(value uint64) {
	zmqMetricsTransferVolumeCounter.Add(float64(value))
}

func updateTransferCounter(numTransfers int) {
	zmqMetricsTransferCounter.Add(float64(numTransfers))
}

func updateConfirmedValueTxMetrics(value uint64, lastInBundle bool) {
	zmqMetricsConfirmedValueTxCounter.Inc()
	zmqMetricsConfirmedValueTxTotalCounter.Add(float64(value))

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
	echoNotSeenPerc.Set(float64(percNotSeen))
	echoMetricsAvgFirstSeen.Set(float64(avgSeenFirstMs))
	echoMetricsAvgLastSeen.Set(float64(avgSeenLastMs))
}

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
				lmConfRate5minMetrics.Set(0)
				lmConfRate10minMetrics.Set(0)
				lmConfRate15minMetrics.Set(0)
				lmConfRate30minMetrics.Set(0)

				errorf("Error while collecting LM conf rate metrics: %v", err)
			} else {
				lmConfRate5minMetrics.Set(valmap["avg_5"])
				lmConfRate10minMetrics.Set(valmap["avg_10"])
				lmConfRate15minMetrics.Set(valmap["avg_15"])
				lmConfRate30minMetrics.Set(valmap["avg_30"])

				debugf("LM conf rate: avg_5 = %v%%, avg_10 = %v%% avg_15 = %v%% avg_30 = %v%%",
					valmap["avg_5"], valmap["avg_10"], valmap["avg_15"], valmap["avg_30"])
			}
		}
	}()
}
