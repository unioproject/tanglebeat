package main

import (
	"database/sql"
	"fmt"
	"github.com/lunfardo314/tanglebeat/pubsub"
	"path"
)

var dbconn *sql.DB

func initDB() {
	log.Infof("Database file: '%v'", Config.DbFile)
	var err error
	dbPathName := path.Join(Config.SiteDataDir, Config.DbFile)
	dbconn, err = sql.Open("sqlite3", dbPathName+"?_timeout=5000")
	defer dbconn.Close()
	if err != nil {
		log.Panicf("Failed to open dbconn at %v: %v%", dbPathName, err)
	}
	createTables()
}

func createTables() {
	tx, err := dbconn.Begin()
	if err != nil {
		log.Panicf("Failed to Begin dbconn tx %v%", err)
	}

	sqlTableTempl :=
		`CREATE TABLE IF NOT EXISTS transfers (
			seqid char(%d) not null,
			idx integer not null,
	        addr char(81) not null unique,
			seqname char(40),
			lastState char(10) not null,
			started_ts_msec integer,
			last_update_msec integer,
			num_attaches integer,
			num_promotions integer,
			avg_pow_duration_per_tx_msec integer,
			avg_gtta_duration_msec integer,
			node_att char(50),
			node_gtta char(50),
			bundle_size integer,
			promo_bundle_size integer,
			promote_every_sec integer,
			force_reattach_every_min integer,

            primary key (seqid, idx)
		)`
	sqlTable := fmt.Sprintf(sqlTableTempl, pubsub.SEQUENCE_UID_LEN)
	_, err = dbconn.Exec(sqlTable)
	if err != nil {
		tx.Rollback()
	} else {
		tx.Commit()
	}
	if err != nil {
		log.Panicf("Failed to create table dbconn tx %v\nSQL = %v", err, sqlTable)
	}
}

SeqUID                  string        `json:"uid"`
SeqName                 string        `json:"nam"`
UpdType                 UpdateType    `json:"typ"`
Index                   int           `json:"idx"`
Addr                    giota.Address `json:"adr"`
SendingStartedTs        int64         `json:"str"`  // time when sending started in this session. Not correct after restart
SinceSendingMsec        int64         `json:"now"`  // time passed until the update. Based on the same clock as sendingStarted
NumAttaches             int           `json:"rea"`  // number of out bundles in tha tangle
NumPromotions           int           `json:"prom"` // number of promotions in the current session (starts with 0 after restart)
AvgPoWDurationPerTxMsec int64         `json:"pow"`  // total millisec spent on attachToTangle calls / nnumer of tx attached
AvgGTTADurationMsec     int64         `json:"gtta"` // total millisec spent on getTransactionsToApproves calls
NodeATT                 string        `json:"natt"`
NodeGTTA                string        `json:"ngta"`
// sender's configuration
BundleSize            int     `json:"bsiz"`  // size of the spending bundle in number of tx
PromoBundleSize       int     `json:"pbsiz"` // size of the promo bundle in number of tx
PromoteEveryNumSec    int     `json:"psec"`
ForceReattachAfterSec int     `json:"fre"`
PromoteNochain        bool    `json:"bb"`  // promo strategy. false means 'blowball', true mean 'chain'
TPS                   float32 `json:"tps"` // contribution to tps
