package main

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
	rows, err := dbconn.Query(sqlSelect, lastMsec)
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
