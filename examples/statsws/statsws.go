package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gonum/stat"
	"github.com/lunfardo314/tanglebeat1/sender_update"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

const version = "0.1"

// JSON returned by active seq WS
type activeSeqResponse struct {
	Version   string         `json:"version"`
	Nowis     int64          `json:"nowis"` // unix time in miliseconds
	Last5min  map[string]int `json:"5min"`
	Last15min map[string]int `json:"15min"`
	Last30min map[string]int `json:"30min"`
}

// JSON returned by the stats WS
type statsResponse struct {
	Version   string       `json:"version"`
	Nowis     int64        `json:"nowis"` // unix time in miliseconds
	Last1h    confTimeData `json:"1h"`
	Last30min confTimeData `json:"30min"`
}

// stat data is seconds
type confTimeData struct {
	Since        int64   `json:"since"`      // samples since when samples were collected unix time in miliseconds
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
	activity        = map[string][]int64{}
)

const debug = true

func addActivity(uid string, ts int64) {
	_, ok := activity[uid]
	if !ok {
		activity[uid] = make([]int64, 0, 100)
	} else {
		alistTmp := make([]int64, 0, len(activity[uid]))
		ago30min := unixms(time.Now().Add(-30 * time.Minute))
		for _, t := range activity[uid] {
			if t >= ago30min {
				alistTmp = append(alistTmp, t)
			}
		}
		activity[uid] = alistTmp
	}
	activity[uid] = append(activity[uid], ts)
}

func addConfTimeSample(confDurationSec float64, ts int64) {
	listMutex.Lock()
	defer listMutex.Unlock()

	if debug {
		fmt.Printf("Before adding sample -- num samples = %v num samples 30 min = %v\n",
			len(sampleDurations), len(samples30min))
	}

	sampleDurations = append(sampleDurations, confDurationSec)
	sampleTs = append(sampleTs, ts)

	// retain only those younger 1h and 30 min
	nowis := time.Now()
	ago1h := unixms(nowis.Add(-1 * time.Hour))
	ago30min := unixms(nowis.Add(-30 * time.Minute))

	sdTmp := make([]float64, 0, len(sampleDurations))
	sd30Tmp := make([]float64, 0, len(samples30min))
	tsTmp := make([]int64, 0, len(sampleTs))

	for i, ts := range sampleTs {
		if ts >= ago1h {
			tsTmp = append(tsTmp, ts)
			sdTmp = append(sdTmp, sampleDurations[i])
			if ts >= ago30min {
				sd30Tmp = append(sd30Tmp, sampleDurations[i])
			}
		}
	}
	sampleDurations = sdTmp
	sampleTs = tsTmp
	samples30min = sd30Tmp

	if since1hTs < ago1h {
		since1hTs = ago1h
	}
	if since30minTs < ago30min {
		since30minTs = ago30min
	}
	if debug {
		fmt.Printf("After adding sample -- num samples = %v num samples 30 min = %v\n",
			len(sampleDurations), len(samples30min))
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

func calcActivityResp(minAgo int, mp map[string]int) {
	nowis := time.Now()
	agoMoment := unixms(nowis.Add(-time.Duration(minAgo) * time.Minute))
	for uid, tslist := range activity {
		c := 0
		for _, ts := range tslist {
			if ts >= agoMoment {
				c += 1
			}
		}
		mp[uid] = c
	}
}

func handlerActivity(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("%v: Request get_activity %v from %v\n", time.Now().Format(time.RFC3339), r.RequestURI, r.RemoteAddr)

	resp := activeSeqResponse{
		Last5min:  make(map[string]int),
		Last15min: make(map[string]int),
		Last30min: make(map[string]int),
	}
	listMutex.Lock()

	calcActivityResp(5, resp.Last5min)
	calcActivityResp(15, resp.Last15min)
	calcActivityResp(30, resp.Last30min)

	resp.Nowis = unixms(time.Now())
	resp.Version = version
	listMutex.Unlock()

	data, err := json.MarshalIndent(resp, "", "   ")
	if err == nil {
		w.Write(data)
	} else {
		fmt.Fprintf(w, "Error while marshaling activity response: %v\n", err)
	}
}

func handlerConfStats(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("%v: Request get_stats %v from %v\n", time.Now().Format(time.RFC3339), r.RequestURI, r.RemoteAddr)

	resp := statsResponse{}

	listMutex.Lock()

	calcStats(sampleDurations, &resp.Last1h)
	resp.Last1h.Since = since1hTs

	calcStats(samples30min, &resp.Last30min)
	resp.Last30min.Since = since30minTs

	listMutex.Unlock()

	resp.Nowis = unixms(time.Now())
	resp.Version = version

	data, err := json.MarshalIndent(resp, "", "   ")
	if err == nil {
		w.Write(data)
	} else {
		fmt.Fprintf(w, "Error while marshaling ststa response: %v\n", err)
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
			if upd.UpdType == sender_update.SENDER_UPD_CONFIRM || debug {
				fmt.Printf("Received '%v' from %v(%v), idx = %v, addr = %v\n",
					upd.UpdType, upd.SeqUID, upd.SeqName, upd.Index, upd.Addr)
			}
			if upd.UpdType == sender_update.SENDER_UPD_CONFIRM {
				addConfTimeSample(float64(upd.UpdateTs-upd.StartTs)/1000, upd.UpdateTs)
				addActivity(upd.SeqUID, upd.UpdateTs)
			}
		}
	}()

}

var port = "3200"

func main() {

	puri := flag.String("target", "tcp://tanglebeat.com:3100", "target to listen")
	pport := flag.Int("port", 3200, "Http port to listen to'")
	flag.Parse()

	uri := *puri
	port := *pport

	fmt.Printf("Initializing statsws ver %v for %v. Http port %v. debug = %v\n", version, uri, port, debug)

	since1hTs = unixms(time.Now())
	since30minTs = unixms(time.Now())

	goListen(uri)

	http.HandleFunc("/get_activity", handlerActivity)
	http.HandleFunc("/get_stats", handlerConfStats)
	panic(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func unixms(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}
