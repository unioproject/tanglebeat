package main

import (
	"github.com/lunfardo314/tanglebeat/lib"
	"math"
	"time"
)

type sums struct {
	count              int64
	confDurationsMsec  int64
	powDurationsMsec   int64
	tipselDurationMsec int64
	numTransactions    int64
}

// TODO filtering rows
var sqlSelect = `select seqid, 
		count(*), 
		sum(last_update_msec-started_ts_msec), 
		sum(total_pow_duration_msec),
		sum(total_tipsel_duration_msec),
		sum(num_attaches * bundle_size + num_promotions * promo_bundle_size)
	 from transfers
	 where last_update_msec >= ?
	`

func sumUpBySequence(lastMsec int64) (map[string]sums, error) {
	ret := make(map[string]sums)
	rows, err := dbconn.Query(sqlSelect, lib.UnixMs(time.Now())-lastMsec)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seqid string
	var count int64
	var confDurationsMsec int64
	var powDurationsMsec int64
	var tipselDurationsMsec int64
	var numTransactions int64

	for rows.Next() {
		err = rows.Scan(&seqid, &count, &confDurationsMsec, &powDurationsMsec, &tipselDurationsMsec, &numTransactions)
		if err != nil {
			return nil, err
		}
		ret[seqid] = sums{
			count:              count,
			confDurationsMsec:  confDurationsMsec,
			powDurationsMsec:   powDurationsMsec,
			tipselDurationMsec: tipselDurationsMsec,
			numTransactions:    numTransactions,
		}
	}
	return ret, nil
}

func toFloat64(data []int64) []float64 {
	ret := make([]float64, len(data))
	for i, n := range data {
		ret[i] = float64(n)
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
	return float64(sum(data)) / float64(len(data))
}

func stddev(data []float64) float64 {
	vid := mean(data)
	d := make([]float64, len(data))
	var x float64
	for i, n := range data {
		x = float64(n) - vid
		d[i] = x * x
	}
	return math.Sqrt(mean(d))
}

// filters out sequence, which counts are less than mean by more tha 1 stddev
func adjustToStddev(bySeq map[string]sums) map[string]sums {
	if len(bySeq) == 0 {
		return nil
	}
	data := make([]float64, len(bySeq))
	idx := 0
	for _, v := range bySeq {
		data[idx] = float64(v.count)
		idx += 1
	}
	vid := mean(data)
	sdev := stddev(data)
	ret := make(map[string]sums, len(bySeq))
	for k, v := range bySeq {
		if float64(v.count) >= vid-sdev {
			ret[k] = bySeq[k]
		}
	}
	return ret
}

func transfersPerSequence(bySeq map[string]sums) float64 {
	if len(bySeq) == 0 {
		return 0
	}
	adjusted := adjustToStddev(bySeq)
	var d []float64
	for _, v := range adjusted {
		d = append(d, float64(v.count))
	}
	return mean(d)
}

// TODO not include not confirmed records
func avgPOWCostPerTransfer(bySeq map[string]sums) float64 {
	if len(bySeq) == 0 {
		return 0
	}
	var numTransfers float64
	var numTx float64
	for _, v := range bySeq {
		numTransfers += float64(v.count)
		numTx += float64(v.numTransactions)
	}
	return numTx / numTransfers
}

func testMetrics() {
	for {
		byseq, err := sumUpBySequence(1 * 60 * 60 * 1000)
		if err != nil {
			log.Errorf("sumUpBySequence: %v", err)
		} else {
			tfph := transfersPerSequence(byseq)
			avgPOW := avgPOWCostPerTransfer(byseq)
			log.Infof("tfphGauge = %v  avgPow = %v", tfph, avgPOW)

			tfphGauge.Set(tfph)
			avgPOWGauge.Set(avgPOW)
		}
		time.Sleep(5 * time.Second)
	}
}
