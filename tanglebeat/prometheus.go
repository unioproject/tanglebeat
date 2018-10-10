package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	tfphGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tfph_metrics",
		Help: "Transfers per hour per sender sequence",
	})
	avgPOWGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "awgPOW_metrics",
		Help: "Average PoW per confirmed transfer",
	})
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(tfphGauge)
	prometheus.MustRegister(avgPOWGauge)
}
func exposeMetrics() {
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":8080", nil))
}
