package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gonum/stat"
	"github.com/lunfardo314/tanglebeat/sender_update"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

// JSON returned by the WS
type response struct {
	Nowis     int64        `json:"nowis"`
	Last1h    confTimeData `json:"1h"`
	Last30min confTimeData `json:"30min"`
}

// time is in seconds
type confTimeData struct {
	Since        int64   `json:"since"`      // samples since when samples were collected unix time miliseconds
	NumSamples   int     `json:"numSamples"` // number of confirmations (samples) collected
	Minimum      float64 `json:"min"`
	Maximum      float64 `json:"max"`
	Mean         float64 `json:"mean"`
	Stddev       float64 `json:"stddev"`
	Percentile20 float64 `json:"p20"`
	Percentile25 float64 `json:"p25"`
	Median       float64 `json:"median"`
	Percentile75 float64 `json:"p75"`
	Percentile80 float64 `json:"p80"`
}

var (
	sampleDurations = make([]float64, 0, 100)
	sampleTs        = make([]int64, 0, 100)
	samples30min    = make([]float64, 0, 100)
	listMutex       sync.Mutex
	since1hTs       int64
	since30minTs    int64
)

func deleteOlderThan1h() {
	ago1h := unixms(time.Now().Add(-1 * time.Hour))
	for i, ts := range sampleTs {
		if ts < ago1h {
			if i+1 < len(sampleTs) {
				copy(sampleDurations[i:], sampleDurations[i+1:])
				copy(sampleTs[i:], sampleTs[i+1:])
			}
		}
	}
	if since1hTs < ago1h {
		since1hTs = ago1h
	}
}

func addSample(confDurationSec float64, ts int64) {
	listMutex.Lock()
	defer listMutex.Unlock()

	deleteOlderThan1h()
	sampleDurations = append(sampleDurations, confDurationSec)
	sampleTs = append(sampleTs, ts)

	ago30min := unixms(time.Now().Add(-30 * time.Minute))
	samples30min = samples30min[:0]
	for i, ts := range sampleTs {
		if ts >= ago30min {
			samples30min = append(samples30min, sampleDurations[i])
		}
	}
	if since30minTs < ago30min {
		since30minTs = ago30min
	}
}

func minMax(arr []float64) (float64, float64) {
	if len(arr) == 0 {
		return 0, 0
	}
	min := arr[0]
	max := arr[0]
	for _, e := range arr {
		if e < min {
			min = e
		}
		if e > max {
			max = e
		}
	}
	return min, max
}

func calcStats(samples []float64, ret *confTimeData) {
	if len(sampleTs) == 0 {
		return
	}
	ret.NumSamples = len(samples)
	ret.Minimum, ret.Maximum = minMax(samples)
	ret.Mean, ret.Stddev = stat.MeanStdDev(samples, nil)
	if math.IsNaN(ret.Stddev) {
		ret.Stddev = 0
	}

	sorted := make([]float64, len(samples))
	copy(sorted, sampleDurations)
	sort.Float64s(sorted)

	ret.Percentile20 = stat.Quantile(0.2, stat.Empirical, sorted, nil)
	ret.Percentile25 = stat.Quantile(0.25, stat.Empirical, sorted, nil)
	ret.Median = stat.Quantile(0.5, stat.Empirical, sorted, nil)
	ret.Percentile75 = stat.Quantile(0.75, stat.Empirical, sorted, nil)
	ret.Percentile80 = stat.Quantile(0.8, stat.Empirical, sorted, nil)
}

func handler(w http.ResponseWriter, r *http.Request) {
	resp := response{}

	listMutex.Lock()

	calcStats(sampleDurations, &resp.Last1h)
	resp.Last1h.Since = since1hTs

	calcStats(samples30min, &resp.Last30min)
	resp.Last30min.Since = since30minTs

	listMutex.Unlock()
	resp.Nowis = unixms(time.Now())

	data, err := json.Marshal(resp)
	if err == nil {
		w.Write(data)
	} else {
		fmt.Fprintf(w, "Error while marshaling: %v\n", err)
	}
}

func goListen(uri string) {
	chIn, err := sender_update.NewUpdateChan(uri)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Start receiving confirmation updates from %v\n", uri)
	go func() {
		for upd := range chIn {
			if upd.UpdType == sender_update.SENDER_UPD_CONFIRM {
				fmt.Printf("Received 'confirm' from %v(%v), idx = %v, addr = %v\n",
					upd.SeqUID, upd.SeqName, upd.Index, upd.Addr)
				addSample(float64(upd.UpdateTs-upd.StartTs)/1000, upd.UpdateTs)
			}
		}
	}()

}

var port = "3200"
var debug = false

func main() {

	puri := flag.String("target", "tcp://tanglebeat.com:3100", "target to listen")
	pport := flag.Int("port", 3200, "Http port to listen to'")
	flag.Parse()

	uri := *puri
	port := *pport

	fmt.Printf("Initializing statsws for %v. Http port %v\n", uri, port)

	since1hTs = unixms(time.Now())
	since30minTs = unixms(time.Now())

	goListen(uri)

	http.HandleFunc("/", handler)
	panic(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func unixms(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}
