package main

import (
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/lunfardo314/tanglebeat/pubsub"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"math"
	"net/http"
	"time"
)

type sumsTransfers struct {
	count              int64
	confDurationsMsec  int64
	powDurationsMsec   int64
	tipselDurationMsec int64
	numTransactions    int64
}

var (
	tfphMetrics = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tanglebeat_tfph",
		Help: "Transfers per hour per sequence.",
	})
	tfphMetricsAdjusted = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tanglebeat_tfph_adjusted",
		Help: "Transfers per hour per sequence adjusted",
	})
	numberOfSequences1h = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tanglebeat_num_seq",
		Help: "Number of active senders last hour",
	})
	numberOfSequencesAdjusted1h = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tanglebeat_num_seq_adjusted",
		Help: "Number of active senders last hour less those with big deviation from average",
	})
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(tfphMetrics)
	prometheus.MustRegister(tfphMetricsAdjusted)
	prometheus.MustRegister(numberOfSequences1h)
	prometheus.MustRegister(numberOfSequencesAdjusted1h)
}
func exposeMetrics() {
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func refreshMetrics(sec int) {
	for {
		dbCacheMutex.Lock()
		calcMetrics()
		dbCacheMutex.Unlock()
		log.Debug("----- set metrics")
		time.Sleep(time.Duration(sec) * time.Second)
	}
}

func calcMetrics() {
	sumsBySeq1h := sumUpBySequences(60)
	sumsBySeq10min := sumUpBySequences(10)

	confNumVect1h := confirmNumsVect(sumsBySeq1h)
	confNumVect10min := confirmNumsVect(sumsBySeq10min)

	avgNumConfirm1h, stddevNumConfirm1h := stddevMean(confNumVect1h)
	avgNumConfirm10min, stddevNumConfirm10min := stddevMean(confNumVect10min)

	var seqToIgnore1h []string
	var seqToIgnore10min []string

	for k, v := range sumsBySeq1h {
		if float64(v.count) < avgNumConfirm1h-stddevNumConfirm1h/2 {
			seqToIgnore1h = append(seqToIgnore1h, k)
		}
	}
	for k, v := range sumsBySeq10min {
		if float64(v.count) < avgNumConfirm10min-stddevNumConfirm10min/2 {
			seqToIgnore10min = append(seqToIgnore10min, k)
		}
	}
	sumsBySeqAdjusted1h := make(map[string]*sumsTransfers)
	for k, v := range sumsBySeq1h {
		if !lib.StringInSlice(k, seqToIgnore1h) {
			sumsBySeqAdjusted1h[k] = v
		}
	}
	sumsBySeqAdjusted10min := make(map[string]*sumsTransfers)
	for k, v := range sumsBySeq10min {
		if !lib.StringInSlice(k, seqToIgnore10min) {
			sumsBySeqAdjusted10min[k] = v
		}
	}

	confNumVectAdjusted1h := confirmNumsVect(sumsBySeqAdjusted1h)
	//confNumVectAdjusted10min := confirmNumsVect(sumsBySeqAdjusted10min)

	metricsNumSequences1h := float64(len(sumsBySeq1h))
	metricsNumSequencesAdjusted1h := float64(len(sumsBySeqAdjusted1h))

	metricsTFpH := float64(0)
	if metricsNumSequences1h != 0 {
		metricsTFpH = sum(confNumVect1h) / metricsNumSequences1h
	}

	metricsTFpHAdjusted := float64(0)
	if metricsNumSequencesAdjusted1h != 0 {
		metricsTFpHAdjusted = sum(confNumVectAdjusted1h) / metricsNumSequencesAdjusted1h
	}

	numberOfSequences1h.Set(metricsNumSequences1h)
	log.Debugf("num_seq = %v", metricsNumSequences1h)

	numberOfSequencesAdjusted1h.Set(metricsNumSequencesAdjusted1h)
	log.Debugf("num_seq_adjusted = %v", metricsNumSequencesAdjusted1h)

	tfphMetrics.Set(metricsTFpH)
	log.Debugf("tfph = %v", metricsTFpH)

	tfphMetricsAdjusted.Set(metricsTFpHAdjusted)
	log.Debugf("tfph_adjusted = %v", metricsTFpHAdjusted)

	//metricsNumSequencesAdjusted10min := len(sumsBySeqAdjusted10min)
	//metricsNumSequencesAdjusted10min := len(sumsBySeqAdjusted10min)

}

func sumUpBySequences(lastMin int64) map[string]*sumsTransfers {
	ret := make(map[string]*sumsTransfers)
	oldest := lib.UnixMs(time.Now()) - lastMin*60*1000
	for k, v := range dbCache1hConfirmed {
		if v.last_update_msec < oldest || v.last_state != string(pubsub.UPD_CONFIRM) {
			continue
		}
		_, ok := ret[k.seqid]
		if ok {
			ret[k.seqid].count += 1
			ret[k.seqid].confDurationsMsec += v.last_update_msec - v.started_ts_msec
			ret[k.seqid].powDurationsMsec += v.total_pow_duration_msec
			ret[k.seqid].tipselDurationMsec += v.total_tipsel_duration_msec
			ret[k.seqid].numTransactions += v.num_attaches*v.bundle_size + v.num_promotions*v.promo_bundle_size
		} else {
			ret[k.seqid] = &sumsTransfers{
				count:              1,
				confDurationsMsec:  v.last_update_msec - v.started_ts_msec,
				powDurationsMsec:   v.total_pow_duration_msec,
				tipselDurationMsec: v.total_tipsel_duration_msec,
				numTransactions:    v.num_attaches*v.bundle_size + v.num_promotions*v.promo_bundle_size,
			}
		}
	}
	return ret
}

func confirmNumsVect(sumMap map[string]*sumsTransfers) []float64 {
	var ret []float64
	for _, v := range sumMap {
		ret = append(ret, float64(v.count))
	}
	return ret
}

func sum(data []float64) float64 {
	var ret float64
	for _, n := range data {
		ret += n
	}
	return ret
}

func mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	return float64(sum(data)) / float64(len(data))
}

func stddevMean(data []float64) (float64, float64) {
	if len(data) == 0 {
		return 0, 0
	}
	vid := mean(data)
	d := make([]float64, len(data))
	var x float64
	for i, n := range data {
		x = float64(n) - vid
		d[i] = x * x
	}
	return vid, math.Sqrt(mean(d))
}
