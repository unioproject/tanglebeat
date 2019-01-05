package zmqpart

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/ebuffer"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/cfg"
	"github.com/lunfardo314/tanglebeat/tanglebeat/hashcache"
	"strconv"
)

const (
	useFirstHashTrytes                   = 18 // first positions of the hash will only be used in hash table. To spare memory
	segmentDurationTXSec                 = 60
	segmentDurationValueTXSec            = 10 * 60
	segmentDurationValueBundleSec        = 10 * 60
	segmentDurationSNSec                 = 1 * 60
	segmentDurationConfirmedTransfersSec = 10 * 60
)

var (
	txcache              *hashcache.HashCacheBase
	sncache              *hashCacheSN
	positiveValueTxCache *hashcache.HashCacheBase
	valueBundleCache     *hashcache.HashCacheBase
	confirmedTransfers   *ebuffer.EventTsWithDataExpiringBuffer
)

type zmqMsg struct {
	routine  *zmqRoutine
	msgData  []byte   // original data
	msgSplit []string // same split to strings
}

const filterChanBufSize = 100

var toFilterChan = make(chan *zmqMsg, filterChanBufSize)

func toFilter(routine *zmqRoutine, msgData []byte, msgSplit []string) {
	toFilterChan <- &zmqMsg{
		routine:  routine,
		msgData:  msgData,
		msgSplit: msgSplit,
	}
}

func initMsgFilter() {
	retentionPeriodSec := cfg.Config.RetentionPeriodMin * 60

	txcache = hashcache.NewHashCacheBase(
		"txcache", useFirstHashTrytes, segmentDurationTXSec, retentionPeriodSec)
	sncache = newHashCacheSN(
		useFirstHashTrytes, segmentDurationSNSec, retentionPeriodSec)
	positiveValueTxCache = hashcache.NewHashCacheBase(
		"positiveValueTxCache", useFirstHashTrytes, segmentDurationValueTXSec, retentionPeriodSec)
	valueBundleCache = hashcache.NewHashCacheBase(
		"valueBundleCache", useFirstHashTrytes, segmentDurationValueBundleSec, retentionPeriodSec)
	confirmedTransfers = ebuffer.NewEventTsWithDataExpiringBuffer(
		"confirmedTransfers", segmentDurationConfirmedTransfersSec, retentionPeriodSec)

	startCollectingLatencyMetrics()
	go msgFilterRoutine()
}

func msgFilterRoutine() {
	for msg := range toFilterChan {
		filterMsg(msg.routine, msg.msgData, msg.msgSplit)
	}
}

func filterMsg(routine *zmqRoutine, msgData []byte, msgSplit []string) {
	switch msgSplit[0] {
	case "tx":
		filterTXMsg(routine, msgData, msgSplit)
	case "sn":
		filterSNMsg(routine, msgData, msgSplit)
	}
}

func filterTXMsg(routine *zmqRoutine, msgData []byte, msgSplit []string) {
	var seen bool
	var behind uint64
	var entry hashcache.CacheEntry

	if len(msgSplit) < 2 {
		errorf("%v: Message %v is invalid", routine.GetUri(), string(msgData))
		return
	}
	seen = txcache.SeenHash(msgSplit[1], nil, &entry)

	if seen {
		behind = utils.SinceUnixMs(entry.FirstSeen)
	} else {
		behind = 0
	}
	routine.accountTx(behind)

	toOutput(msgData, msgSplit, entry.Visits)
}

func filterSNMsg(routine *zmqRoutine, msgData []byte, msgSplit []string) {
	var hash string
	var seen bool
	var behind uint64
	var err error
	var entry hashcache.CacheEntry

	if len(msgSplit) < 3 {
		errorf("%v: Message %v is invalid", routine.GetUri(), string(msgData))
		return
	}
	obsolete, err := checkObsoleteMsg(msgData, msgSplit, routine.GetUri())
	if err != nil {
		errorf("checkObsoleteMsg: %v", err)
		return
	}
	if obsolete {
		// if index of the current confirmetion message is less than the latest seen,
		// confirmation is ignored.
		// Reason: if it is not too old, it must had been seen from other sources
		routine.incObsoleteCount()
		return
	}
	hash = msgSplit[2]

	seen = sncache.SeenHash(hash, nil, &entry)
	if seen {
		behind = utils.SinceUnixMs(entry.FirstSeen)
	} else {
		behind = 0
	}
	routine.accountSn(behind)

	toOutput(msgData, msgSplit, entry.Visits)
}

func checkObsoleteMsg(msgData []byte, msgSplit []string, uri string) (bool, error) {
	if len(msgSplit) < 3 {
		return false, fmt.Errorf("%v: Message %v is invalid", uri, string(msgData))
	}
	index, err := strconv.Atoi(msgSplit[1])
	if err != nil {
		return false, fmt.Errorf("expected index, found %v", msgSplit[1])
	}
	obsolete, _ := sncache.checkCurrentMilestoneIndex(index, uri)
	return obsolete, nil
}
