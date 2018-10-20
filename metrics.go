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
	confirmationDurationSecGauge        *prometheus.GaugeVec
	confirmationPoWCostGauge            *prometheus.GaugeVec
	confirmationPoWDurationMsecGauge    *prometheus.GaugeVec
	confirmationTipselDurationMsecGauge *prometheus.GaugeVec
)

func exposeMetrics(port int) {
	http.Handle("/metrics", promhttp.Handler())
	panic(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func initAndRunMetricsUpdater(port int) {
	confirmationDurationSecGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tanglebeat_confirmation_duration_sec",
		Help: "Confirmation duration of the transfer.",
	}, []string{"seqid"})

	confirmationPoWCostGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tanglebeat_pow_cost",
		Help: "Number of tx attached for the confirmation = num. attachments * bundle size + num. promotions * promo bundle size",
	}, []string{"seqid"})

	confirmationPoWDurationMsecGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tanglebeat_pow_duration_msec",
		Help: "Total duration it took to do PoW for confirmation.",
	}, []string{"seqid", "node_pow"})

	confirmationTipselDurationMsecGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tanglebeat_tipsel_duration_msec",
		Help: "Total duration it took to do tip selection for confirmation.",
	}, []string{"seqid", "node_tipsel"})

	prometheus.MustRegister(confirmationDurationSecGauge)
	prometheus.MustRegister(confirmationPoWCostGauge)
	prometheus.MustRegister(confirmationPoWDurationMsecGauge)
	prometheus.MustRegister(confirmationTipselDurationMsecGauge)

	go exposeMetrics(port)
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

	confirmationPoWDurationMsecGauge.
		With(prometheus.Labels{
			"seqid":    upd.SeqUID,
			"node_pow": upd.NodeATT,
		}).Set(float64(upd.TotalPoWMsec))

	confirmationTipselDurationMsecGauge.
		With(prometheus.Labels{
			"seqid":       upd.SeqUID,
			"node_tipsel": upd.NodeGTTA,
		}).Set(float64(upd.TotalTipselMsec))
}
