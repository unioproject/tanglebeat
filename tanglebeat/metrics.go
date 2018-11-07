package main

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/sender_update"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

// TODO restart metrics
// TODO att and gtta duration metrics
// pow/min quota metrics
// TPS/CTPS/con rate metrics
// milestone metrics

var (
	confCounter                  *prometheus.CounterVec
	confPoWCostCounter           *prometheus.CounterVec
	confDurationSecCounter       *prometheus.CounterVec
	confPoWDurationSecCounter    *prometheus.CounterVec
	confTipselDurationSecCounter *prometheus.CounterVec
	//confDurationSummary          prometheus.Summary
)

func exposeMetrics(port int) {
	http.Handle("/metrics", promhttp.Handler())
	listenAndServeOn := fmt.Sprintf(":%d", port)
	log.Infof("Exposing Prometheus metrics on %v", listenAndServeOn)
	panic(http.ListenAndServe(listenAndServeOn, nil))
}

func initExposeToPometheus() {
	confCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tanglebeat_confirmation_counter",
		Help: "Increases every time sender confirms a transfer",
	}, []string{"seqid"})

	confPoWCostCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tanglebeat_pow_cost_counter",
		Help: "Counter for number of tx attached during the confirmation = num. attachments * bundle size + num. promotions * promo bundle size",
	}, []string{"seqid"})

	confDurationSecCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tanglebeat_confirmation_duration_counter",
		Help: "Sums up confirmation durations of the transfer.",
	}, []string{"seqid"})

	confPoWDurationSecCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tanglebeat_pow_duration_counter",
		Help: "Sums up total duration it took to do PoW for confirmation.",
	}, []string{"seqid", "node_pow"})

	confTipselDurationSecCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tanglebeat_tipsel_duration_counter",
		Help: "Sums up total duration it took to do tip selection for confirmation.",
	}, []string{"seqid", "node_tipsel"})

	//buck := map[float64]float64{
	//	0.2:  0.015,
	//	0.25: 0.015,
	//	0.5:  0.015,
	//	0.75: 0.015,
	//	0.8:  0.015,
	//}
	//confDurationSummary = prometheus.NewSummary(prometheus.SummaryOpts{
	//	Name:       "tanglebeat_conf_duration_summary",
	//	Help:       "Used to calculate fixed quantiles of confirm duration",
	//	Objectives: buck,
	//	MaxAge:     1 * time.Hour,
	//})

	prometheus.MustRegister(confCounter)
	prometheus.MustRegister(confDurationSecCounter)
	prometheus.MustRegister(confPoWCostCounter)
	prometheus.MustRegister(confPoWDurationSecCounter)
	prometheus.MustRegister(confTipselDurationSecCounter)
	//prometheus.MustRegister(confDurationSummary)

	go exposeMetrics(Config.Prometheus.ScrapeTargetPort)
}

func updateSenderMetrics(upd *sender_update.SenderUpdate) {
	if upd.UpdType != sender_update.SENDER_UPD_CONFIRM {
		return
	}
	confCounter.With(prometheus.Labels{"seqid": upd.SeqUID}).Inc()

	durSec := float64(upd.UpdateTs-upd.StartTs) / 1000
	confDurationSecCounter.
		With(prometheus.Labels{"seqid": upd.SeqUID}).Add(durSec)

	//confDurationSummary.Observe(durSec)

	powCost := float64(upd.NumAttaches*int64(upd.BundleSize) + upd.NumPromotions*int64(upd.PromoBundleSize))
	confPoWCostCounter.
		With(prometheus.Labels{"seqid": upd.SeqUID}).Add(powCost)

	confPoWDurationSecCounter.
		With(prometheus.Labels{
			"seqid":    upd.SeqUID,
			"node_pow": upd.NodeATT,
		}).Add(float64(upd.TotalPoWMsec) / 1000)

	confTipselDurationSecCounter.
		With(prometheus.Labels{
			"seqid":       upd.SeqUID,
			"node_tipsel": upd.NodeGTTA,
		}).Add(float64(upd.TotalTipselMsec) / 1000)
}
