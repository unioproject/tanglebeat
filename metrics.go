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
	confirmationDurationSecGauge *prometheus.GaugeVec
	confirmationPoWCostGauge     *prometheus.GaugeVec
)

func exposeMetrics(port int) {
	http.Handle("/metrics", promhttp.Handler())
	panic(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func updateMetrics(upd *SenderUpdate) {
	if upd.UpdType != SENDER_UPD_CONFIRM {
		return
	}
	confirmationDurationSecGauge.
		With(prometheus.Labels{"seqid": upd.SeqUID}).Set(float64(upd.UpdateTs-upd.SendingStartedTs) / 1000)

	powCost := float64(upd.NumAttaches*int64(upd.BundleSize) + upd.NumPromotions*int64(upd.PromoBundleSize))
	confirmationPoWCostGauge.
		With(prometheus.Labels{"seqid": upd.SeqUID}).Set(powCost)
}

func initAndRunMetricsUpdater(port int) {
	confirmationDurationSecGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tanglebeat_confirmation_duration_sec",
		Help: "Confirmation duration of the transfer.",
	}, []string{"seqid"})

	confirmationPoWCostGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tanglebeat_pow_cost",
		Help: "Confirmation cost in PoW done to confirm. = num. attachments * bundle size + num. promotions * promo bundle size",
	}, []string{"seqid"})

	prometheus.MustRegister(confirmationDurationSecGauge)
	prometheus.MustRegister(confirmationPoWCostGauge)

	go exposeMetrics(port)
}
