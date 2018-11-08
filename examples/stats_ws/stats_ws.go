package main

import (
	"encoding/json"
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/lunfardo314/tanglebeat/sender_update"
	"math"
	"net/http"
	"os"
	"sync"
	"time"
)

// time is in seconds
type confTimeData struct {
	Since        int64   `json:"sinceTs"`    // samples collected startTs unix time miliseconds
	NumSamples   int     `json:"numSamples"` // number of confirmations (samples) collected
	Minimum      float64 `json:"min"`
	Maximum      float64 `json:"max"`
	Mean         float64 `json:"mean"`
	Percentile20 float64 `json:"p20"`
	Percentile25 float64 `json:"p25"`
	Median       float64 `json:"median"`
	Percentile75 float64 `json:"p75"`
	Percentile80 float64 `json:"p80"`
}

type sample struct {
	confDurationSec float64
	ts              int64
}

var (
	sampleList = []*sample{}
	listMutex  sync.Mutex
	sinceTs    int64
)

func deleteOlderThat1h() {
	var tmp []*sample
	ago1h := lib.UnixMs(time.Now().Add(-1 * time.Hour))
	for _, s := range sampleList {
		if s.ts >= ago1h {
			tmp = append(tmp, s)
		}
	}
	sampleList = tmp
	if sinceTs < ago1h {
		sinceTs = ago1h
	}

}

func addSample(confDurationSec float64, ts int64) {
	listMutex.Lock()
	defer listMutex.Unlock()

	sampleList = append(sampleList, &sample{confDurationSec: confDurationSec, ts: ts})
	deleteOlderThat1h()
}

func goListen(uri string) {
	chIn, err := sender_update.NewUpdateChan(uri)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Start receiving sender updates from %v\n", uri)
	go func() {
		for upd := range chIn {
			fmt.Printf("Received '%v' from %v(%v), idx = %v, addr = %v\n",
				upd.UpdType, upd.SeqUID, upd.SeqName, upd.Index, upd.Addr)
			if upd.UpdType == sender_update.SENDER_UPD_CONFIRM {
				addSample(float64(upd.UpdateTs-upd.StartTs)/1000, upd.UpdateTs)
			}
		}
	}()

}

func calcStats() *confTimeData {
	listMutex.Lock()
	defer listMutex.Unlock()
	ret := &confTimeData{Since: sinceTs}
	if len(sampleList) == 0 {
		return ret
	}
	ret.Minimum = sampleList[0].confDurationSec
	ret.Maximum = sampleList[0].confDurationSec
	// TODO percentiles, startTs
	for _, s := range sampleList {
		ret.Minimum = math.Min(ret.Minimum, s.confDurationSec)
		ret.Maximum = math.Max(ret.Maximum, s.confDurationSec)
		ret.Mean += s.confDurationSec
	}
	ret.NumSamples = len(sampleList)
	ret.Mean = ret.Mean / float64(ret.NumSamples)
	return ret
}

func handler(w http.ResponseWriter, r *http.Request) {
	stats := calcStats()
	data, err := json.Marshal(stats)
	if err == nil {
		w.Write(data)
	} else {
		fmt.Fprintf(w, "Error while marshaling: %v\n", err)
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: stats_ws tcp://<host>:<port>\nExample: stats_ws tcp://my.host.com:3100")
		os.Exit(0)
	}
	fmt.Printf("Initializing stats_ws for %v\n", os.Args[1])

	sinceTs = lib.UnixMs(time.Now())
	goListen(os.Args[1])

	http.HandleFunc("/", handler)
	panic(http.ListenAndServe(":3200", nil))
}
