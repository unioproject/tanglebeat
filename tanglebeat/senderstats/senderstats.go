package senderstats

import (
	"encoding/json"
	"fmt"
	"github.com/gonum/stat"
	"github.com/lunfardo314/tanglebeat/lib/ebuffer"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tbsender/sender_update"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

const version = "0.2"

// JSON returned by the stats WS
type statsResponse struct {
	Nowis     uint64             `json:"nowis"` // unix time in miliseconds
	Last1h    confTimeDataStruct `json:"1h"`
	Last30min confTimeDataStruct `json:"30min"`
	Last10min confTimeDataStruct `json:"10min"`
}

// stat data is seconds
type confTimeDataStruct struct {
	Since        uint64  `json:"since"`      // samples since when samples were collected unix time in miliseconds
	NumSamples   uint64  `json:"numSamples"` // number of confirmations (samples) collected
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
	sampleDurations   *ebuffer.EventTsWithIntExpiringBuffer
	confTimeData10min confTimeDataStruct
	confTimeData30min confTimeDataStruct
	confTimeData1h    confTimeDataStruct
	confTimeDataMutex *sync.RWMutex
)

func init() {
	sampleDurations = ebuffer.NewEventTsWithIntExpiringBuffer("sampleDurations", 10*60, 60*60)
	confTimeDataMutex = &sync.RWMutex{}
	go calcStatsLoop()
}

func calcStatsLoop() {
	for {
		confTimeDataMutex.Lock()
		calcStats(10*60*1000, &confTimeData10min)
		calcStats(30*60*1000, &confTimeData30min)
		calcStats(60*60*1000, &confTimeData1h)
		confTimeDataMutex.Unlock()

		time.Sleep(5 * time.Second)
	}
}

func calcStats(msecAgo uint64, ret *confTimeDataStruct) {
	var arr []float64
	arr, ret.Since = sampleDurations.ToFloat64(msecAgo)
	if len(arr) == 0 {
		*ret = confTimeDataStruct{}
		return
	}
	sort.Float64s(arr)

	ret.NumSamples = uint64(len(arr))
	ret.Minimum, ret.Maximum = minMax(arr)
	ret.Mean, ret.Stddev = stat.MeanStdDev(arr, nil)
	if math.IsNaN(ret.Stddev) {
		ret.Stddev = 0
	}

	ret.Percentile20 = stat.Quantile(0.2, stat.Empirical, arr, nil)
	ret.Percentile25 = stat.Quantile(0.25, stat.Empirical, arr, nil)
	ret.Median = stat.Quantile(0.5, stat.Empirical, arr, nil)
	ret.Percentile75 = stat.Quantile(0.75, stat.Empirical, arr, nil)
	ret.Percentile80 = stat.Quantile(0.8, stat.Empirical, arr, nil)

	// convert milliseconds to seconds and round to 2 decimal places
	ret.Minimum = math.Round(ret.Minimum/10) / 100
	ret.Maximum = math.Round(ret.Maximum/10) / 100
	ret.Mean = math.Round(ret.Mean/10) / 100
	ret.Stddev = math.Round(ret.Stddev/10) / 100
	ret.Percentile20 = math.Round(ret.Percentile20/10) / 100
	ret.Percentile25 = math.Round(ret.Percentile25/10) / 100
	ret.Median = math.Round(ret.Median/10) / 100
	ret.Percentile75 = math.Round(ret.Percentile75/10) / 100
	ret.Percentile80 = math.Round(ret.Percentile80/10) / 100
}

func HandlerConfStats(w http.ResponseWriter, r *http.Request) {
	debugf("%v: Request get_stats %v from %v\n", time.Now().Format(time.RFC3339), r.RequestURI, r.RemoteAddr)

	resp := statsResponse{}

	confTimeDataMutex.RLock()
	resp.Last10min = confTimeData10min
	resp.Last30min = confTimeData30min
	resp.Last1h = confTimeData1h
	confTimeDataMutex.RUnlock()
	resp.Nowis = utils.UnixMsNow()

	data, err := json.MarshalIndent(resp, "", "   ")
	if err == nil {
		_, _ = w.Write(data)
	} else {
		_, _ = fmt.Fprintf(w, "Error while marshaling stats response: %v\n", err)
	}
}

func SenderUpdateToStats(upd *sender_update.SenderUpdate) {
	if upd.UpdType == sender_update.SENDER_UPD_CONFIRM {
		sampleDurations.RecordInt(int(upd.UpdateTs) - int(upd.StartTs))
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
