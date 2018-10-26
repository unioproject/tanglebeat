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
	confirmationCounter                 *prometheus.CounterVec
	confirmationDurationSecGauge        *prometheus.GaugeVec
	confirmationPoWCostCounter          *prometheus.CounterVec
	confirmationPoWDurationMsecGauge    *prometheus.GaugeVec
	confirmationTipselDurationMsecGauge *prometheus.GaugeVec
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

	confirmationDurationSecGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tanglebeat_confirmation_duration_sec",
		Help: "Confirmation duration of the transfer.",
	}, []string{"seqid"})

	confirmationPoWCostCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tanglebeat_pow_cost",
		Help: "Counter for number of tx attached during the confirmation = num. attachments * bundle size + num. promotions * promo bundle size",
	}, []string{"seqid"})

	confirmationPoWDurationMsecGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tanglebeat_pow_duration_msec",
		Help: "Total duration it took to do PoW for confirmation.",
	}, []string{"seqid", "node_pow"})

	confirmationTipselDurationMsecGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tanglebeat_tipsel_duration_msec",
		Help: "Total duration it took to do tip selection for confirmation.",
	}, []string{"seqid", "node_tipsel"})

	prometheus.MustRegister(confirmationCounter)
	prometheus.MustRegister(confirmationDurationSecGauge)
	prometheus.MustRegister(confirmationPoWCostCounter)
	prometheus.MustRegister(confirmationPoWDurationMsecGauge)
	prometheus.MustRegister(confirmationTipselDurationMsecGauge)

	go exposeMetrics(Config.Prometheus.ScrapeTargetPort)
}

func updateSenderMetrics(upd *SenderUpdate) {
	if upd.UpdType != SENDER_UPD_CONFIRM {
		return
	}
	confirmationCounter.With(prometheus.Labels{"seqid": upd.SeqUID}).Inc()

	confirmationDurationSecGauge.
		With(prometheus.Labels{"seqid": upd.SeqUID}).Set(float64(upd.UpdateTs-upd.SendingStartedTs) / 1000)

	powCost := float64(upd.NumAttaches*int64(upd.BundleSize) + upd.NumPromotions*int64(upd.PromoBundleSize))
	confirmationPoWCostCounter.
		With(prometheus.Labels{"seqid": upd.SeqUID}).Add(powCost)

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
