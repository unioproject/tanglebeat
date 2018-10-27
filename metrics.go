package main

import (
	"fmt"
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
	confirmationCounter                  *prometheus.CounterVec
	confirmationPoWCostCounter           *prometheus.CounterVec
	confirmationDurationSecCounter       *prometheus.CounterVec
	confirmationPoWDurationSecCounter    *prometheus.CounterVec
	confirmationTipselDurationSecCounter *prometheus.CounterVec
)

func exposeMetrics(port int) {
	http.Handle("/metrics", promhttp.Handler())
	listenAndServeOn := fmt.Sprintf(":%d", port)
	log.Infof("Exposing Prometheus metrics on %v", listenAndServeOn)
	panic(http.ListenAndServe(listenAndServeOn, nil))
}

func initExposeToPometheus() {
	confirmationCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tanglebeat_confirmation_counter",
		Help: "Increases every time sender confirms a transfer",
	}, []string{"seqid"})

	confirmationPoWCostCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tanglebeat_pow_cost_counter",
		Help: "Counter for number of tx attached during the confirmation = num. attachments * bundle size + num. promotions * promo bundle size",
	}, []string{"seqid"})

	confirmationDurationSecCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tanglebeat_confirmation_duration_counter",
		Help: "Sums up confirmation durations of the transfer.",
	}, []string{"seqid"})

	confirmationPoWDurationSecCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tanglebeat_pow_duration_counter",
		Help: "Sums up total duration it took to do PoW for confirmation.",
	}, []string{"seqid", "node_pow"})

	confirmationTipselDurationSecCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tanglebeat_tipsel_duration_counter",
		Help: "Sums up total duration it took to do tip selection for confirmation.",
	}, []string{"seqid", "node_tipsel"})

	prometheus.MustRegister(confirmationCounter)
	prometheus.MustRegister(confirmationDurationSecCounter)
	prometheus.MustRegister(confirmationPoWCostCounter)
	prometheus.MustRegister(confirmationPoWDurationSecCounter)
	prometheus.MustRegister(confirmationTipselDurationSecCounter)

	go exposeMetrics(Config.Prometheus.ScrapeTargetPort)
}

func updateSenderMetrics(upd *SenderUpdate) {
	if upd.UpdType != SENDER_UPD_CONFIRM {
		return
	}
	confirmationCounter.With(prometheus.Labels{"seqid": upd.SeqUID}).Inc()

	confirmationDurationSecCounter.
		With(prometheus.Labels{"seqid": upd.SeqUID}).Add(float64(upd.UpdateTs-upd.SendingStartedTs) / 1000)

	powCost := float64(upd.NumAttaches*int64(upd.BundleSize) + upd.NumPromotions*int64(upd.PromoBundleSize))
	confirmationPoWCostCounter.
		With(prometheus.Labels{"seqid": upd.SeqUID}).Add(powCost)

	confirmationPoWDurationSecCounter.
		With(prometheus.Labels{
			"seqid":    upd.SeqUID,
			"node_pow": upd.NodeATT,
		}).Add(float64(upd.TotalPoWMsec) / 1000)

	confirmationTipselDurationSecCounter.
		With(prometheus.Labels{
			"seqid":       upd.SeqUID,
			"node_tipsel": upd.NodeGTTA,
		}).Add(float64(upd.TotalTipselMsec) / 1000)
}
