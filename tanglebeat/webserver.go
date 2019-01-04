package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

func runWebServer(port int) {
	infof("Web server for Prometheus metrics and debug dashboard will be running on port '%d'", port)
	http.HandleFunc("/index", indexHandler)
	http.HandleFunc("/stats", statsHandler)
	http.HandleFunc("/loadjs", loadjsHandler)
	http.HandleFunc("/api1/", api1Handler)
	http.Handle("/metrics", promhttp.Handler())
	panic(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprintf(w, string(getGlbStatsJSON()))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprint(w, indexPage)
}

func loadjsHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprint(w, loadjs)
}

func api1Handler(w http.ResponseWriter, r *http.Request) {
	req := r.URL.Path[len("/api1/"):]

	_, _ = fmt.Fprint(w, req)
}
