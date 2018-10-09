package main

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/lunfardo314/tanglebeat/pubsub"
	_ "github.com/mattn/go-sqlite3"
	"path"
	"strings"
)

var dbconn *sql.DB

func initDB() {
	log.Infof("Database file: '%v'", Config.DbFile)
	var err error
	dbPathName := path.Join(Config.SiteDataDir, Config.DbFile)
	dbconn, err = sql.Open("sqlite3", dbPathName+"?_timeout=5000")
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
			total_pow_duration_msec integer,
			total_tipsel_duration_msec integer,
			node_att char(50),
			node_gtta char(50),
			bundle_size integer,
			promo_bundle_size integer,
			promote_every_sec integer,
			force_reattach_every_min integer,
			promote_chain_yn char(1),
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

func runUpdateDb() {
	chanUpdate, err := pubsub.OpenSenderUpdateChan(Config.SenderURI, log)
	if err != nil {
		log.Criticalf("can't get new sub socket: %v", err)
	}

	log.Info("Started listening to data stream from sender")
	for upd := range chanUpdate {
		log.Debugf("Received update '%v'", upd.UpdType)
		if err := updateRecord(upd); err != nil {
			log.Errorf("Error from updateRecord: %v", err)
		}
	}
}

func existsTransferRecord(uid string, index int) (bool, error) {
	sqlSelect := "select idx from transfers where seqid=? and idx=?"
	var idx int64
	row := dbconn.QueryRow(sqlSelect, uid, index)
	switch err := row.Scan(&idx); err {
	case sql.ErrNoRows:
		return false, nil
	case nil:
		return true, nil
	default:
		return false, err
	}
}

const insertTemplate = "insert into transfers (%s) values (%s)"
const updateTemplate = "update transfers set %s where %s"

func makeInsertSql(cols []string) string {
	qmlist := make([]string, len(cols))
	for i := range qmlist {
		qmlist[i] = "?"
	}
	return fmt.Sprintf(insertTemplate, strings.Join(allColumns, ", "), strings.Join(qmlist, ", "))
}

func makeUpdateSql(updateCols []string, pkCols []string) string {
	var colAssignList []string
	for _, col := range updateCols {
		colAssignList = append(colAssignList, col+"=?")
	}
	var whereCondList []string
	for _, col := range pkCols {
		whereCondList = append(whereCondList, col+"=?")
	}
	return fmt.Sprintf(updateTemplate, strings.Join(colAssignList, ", "), strings.Join(whereCondList, " and "))
}

var allColumns = []string{
	"seqid", "idx", "addr", "seqname", "lastState", "started_ts_msec", "last_update_msec",
	"num_attaches", "num_promotions", "total_pow_duration_msec", "total_tipsel_duration_msec",
	"node_att", "node_gtta", "bundle_size", "promo_bundle_size", "promote_every_sec",
	"force_reattach_every_min", "promote_chain_yn",
}

func allColumnsExcept(except []string) []string {
	var ret []string
	for _, s := range allColumns {
		if !lib.StringInSlice(s, except) {
			ret = append(ret, s)
		}
	}
	return ret
}

func errorWithSql(err error, sqlStmt string) error {
	return errors.New(fmt.Sprintf("Error: %v\nSQL = %v", err, sqlStmt))
}

func updateRecord(upd *pubsub.SenderUpdate) error {
	exists, errRet := existsTransferRecord(upd.SeqUID, upd.Index)
	if errRet != nil {
		return errRet
	}
	tx, errRet := dbconn.Begin()
	if errRet != nil {
		return errRet
	}
	defer func(perr *error) {
		if *perr != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}(&errRet)

	var sqlStmt string
	var stmt *sql.Stmt
	chain := "N"
	if upd.PromoteChain {
		chain = "Y"
	}
	if exists {
		sqlStmt = makeUpdateSql(
			allColumnsExcept([]string{"started_ts_msec", "seqid", "idx", "addr"}),
			[]string{"seqid", "idx"},
		)
		stmt, errRet = dbconn.Prepare(sqlStmt)
		if errRet != nil {
			return errorWithSql(errRet, sqlStmt)
		}
		defer stmt.Close()
		_, errRet = stmt.Exec(
			upd.SeqName, upd.UpdType, upd.UpdateTs,
			upd.NumAttaches, upd.NumPromotions, upd.TotalPoWMsec, upd.TotalTipselMsec,
			upd.NodeATT, upd.NodeGTTA, upd.BundleSize, upd.PromoBundleSize, upd.PromoteEveryNumSec,
			upd.ForceReattachAfterSec, chain,
			upd.SeqUID, upd.Index,
		)
		if errRet != nil {
			return errorWithSql(errRet, sqlStmt)
		}
	} else {
		sqlStmt = makeInsertSql(allColumns)
		stmt, errRet = dbconn.Prepare(sqlStmt)
		if errRet != nil {
			return errorWithSql(errRet, sqlStmt)
		}
		defer stmt.Close()

		_, errRet = stmt.Exec(
			upd.SeqUID, upd.Index, upd.Addr, upd.SeqName, upd.UpdType, upd.SendingStartedTs, upd.UpdateTs,
			upd.NumAttaches, upd.NumPromotions, upd.TotalPoWMsec, upd.TotalTipselMsec,
			upd.NodeATT, upd.NodeGTTA, upd.BundleSize, upd.PromoBundleSize, upd.PromoteEveryNumSec,
			upd.ForceReattachAfterSec, chain,
		)
		if errRet != nil {
			return errorWithSql(errRet, sqlStmt)
		}
	}
	errRet = nil
	return errRet
}
