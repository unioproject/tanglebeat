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
