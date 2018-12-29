package main

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

func runWebServer(port int) {
	infof("Web server for Prometheus metrics and debug dashboard will be running on port '%d'", port)
	http.HandleFunc("/index", indexHandler)
	http.HandleFunc("/stats", statsHandler)
	http.HandleFunc("/loadjs", loadjsHandler)
	http.Handle("/metrics", promhttp.Handler())
	panic(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

type allStatsStruct struct {
	RoutineStats map[string]*zmqRoutineStats `json:"zmqRoutineStats"`
	GlobalStats  glbStatsStruct              `json:"globalStats"`
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	stats := allStatsStruct{
		RoutineStats: getRoutineStats(),
		GlobalStats:  glbStats.getCopy(),
	}
	data, err := json.MarshalIndent(stats, "", "   ")
	if err != nil {
		errorf("marshal error: %v", err)
	}
	_, _ = fmt.Fprintf(w, string(data))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprint(w, indexPage)
}

func loadjsHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprint(w, loadjs)
}
