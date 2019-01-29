package main

import . "github.com/prometheus/client_golang/prometheus"

var (
	metricsMemAllocMB Gauge
	//metricsGoRoutine     Gauge
)

func init() {
	metricsMemAllocMB = NewGauge(GaugeOpts{
		Name: "tanglebeat_rt_memalloc_mb",
		Help: "Allocated runtime memory in MB ",
	})
	MustRegister(metricsMemAllocMB)

	//metricsMemAllocMB = NewGauge(GaugeOpts{
	//	Name: "tanglebeat_rt_goroutines_mb",
	//	Help: "Number of goroutines",
	//})
	//MustRegister(metricsMemAllocMB)

}

func updateRuntimeMetrics(memAllocMB float64) {
	if memAllocMB > 0 {
		metricsMemAllocMB.Set(memAllocMB)
	}
}
