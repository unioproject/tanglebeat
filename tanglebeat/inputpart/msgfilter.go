package inputpart

import (
	"fmt"
	"github.com/unioproject/tanglebeat/lib/utils"
	"github.com/unioproject/tanglebeat/tanglebeat/cfg"
	"github.com/unioproject/tanglebeat/tanglebeat/hashcache"
	"math"
	"strconv"
	"sync"
)

const (
	useFirstHashTrytes   = 12 // first positions of the hash will only be used in hash table. To spare memory
	segmentDurationTXSec = 60
	segmentDurationSNSec = 1 * 60
)

var (
	txcache          *hashcache.HashCacheBase
	sncache          *hashCacheSN
	lastLMI          int
	lastLMITimesSeen int
	lastLMIFirstSeen uint64
	lastLMILastSeen  uint64
	lmiMutex         = &sync.RWMutex{}
	lmhsCache        *hashcache.HashCacheBase
)

type zmqMsg struct {
	routine  *inputRoutine
	msgData  []byte   // original data
	msgSplit []string // same split to strings
}

const filterChanBufSize = 100

var toFilterChan = make(chan *zmqMsg, filterChanBufSize)

func toFilter(routine *inputRoutine, msgData []byte, msgSplit []string) {
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
	// use all trytes of milestone hash
	lmhsCache = hashcache.NewHashCacheBase("lmhscache", 0, segmentDurationTXSec, retentionPeriodSec)

	startCollectingLatencyMetrics()
	startCollectingLMConfRate()

	go msgFilterLoop()
}

func msgFilterLoop() {
	for msg := range toFilterChan {
		filterMsg(msg.routine, msg.msgData, msg.msgSplit)
	}
}

// only start processing tx and sn messages after first two lmi messages arrived
// the reason is to avoid (filter out) obsolete sn rubbish
func filterMsg(routine *inputRoutine, msgData []byte, msgSplit []string) {
	switch msgSplit[0] {
	case "tx":
		if sncache.firstMilestoneArrived() {
			filterTXMsg(routine, msgData, msgSplit)
		}
	case "sn":
		if sncache.firstMilestoneArrived() {
			filterSNMsg(routine, msgData, msgSplit)
		}
	case "lmi":
		filterLMIMsg(routine, msgData, msgSplit)

	case "lmhs":
		filterLMHSMsg(routine, msgData, msgSplit)
	}
}

func filterTXMsg(routine *inputRoutine, msgData []byte, msgSplit []string) {
	var entry hashcache.CacheEntry

	if len(msgSplit) < 2 {
		errorf("%v: Message %v is invalid", routine.GetUri(), string(msgData))
		return
	}

	routine.accountTx()
	if routine.IsOutputClosed() {
		return // not putting into the cache
	}

	txcache.SeenHashBy(msgSplit[1], routine.GetId__(), nil, &entry)

	// check and account for echo to the promotion transactions
	checkForEcho(msgSplit[1], utils.UnixMsNow())

	// check if message was seen exactly number of times as configured (usually 2)
	if int(entry.Visits) == GetTxQuorum() {
		toOutput(msgData, msgSplit)
	}
}

func filterSNMsg(routine *inputRoutine, msgData []byte, msgSplit []string) {
	var hash string
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
		// if index of the current confirmation message is less than the latest seen,
		// confirmation is ignored.
		// Reason: if it is not too old, it must had been seen from other sources
		routine.incObsoleteCount()
		return
	}

	routine.accountSn()
	if routine.IsOutputClosed() {
		return // not putting into the cache
	}
	hash = msgSplit[2]

	sncache.SeenHashBy(hash, routine.GetId__(), nil, &entry)

	// check if message was seen exactly number of times as configured (usually 2)
	if int(entry.Visits) == GetSnQuorum() {
		toOutput(msgData, msgSplit)
	}
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

func filterLMIMsg(routine *inputRoutine, msgData []byte, msgSplit []string) {
	index, err := strconv.Atoi(msgSplit[1])
	if err != nil {
		errorf("Invalid 'lmi' message: at index 1 expected to be milestone index: %v", err)
		return
	}
	if !sncache.firstMilestoneArrived() {
		uri := routine.GetUri()
		infof("+++++++++++++++++ Milestone %v arrived from %v", index, uri)
		sncache.checkCurrentMilestoneIndex(index, uri)
	}
	routine.accountLmi(index)
	if routine.IsOutputClosed() {
		return // not putting into the cache
	}

	lmiMutex.Lock()
	defer lmiMutex.Unlock()

	switch {
	case index > lastLMI:
		lastLMI = index
		lastLMITimesSeen = 0
		lastLMIFirstSeen = utils.UnixMsNow()
		lastLMILastSeen = utils.UnixMsNow()
	case index == lastLMI:
		lastLMITimesSeen++
		lastLMILastSeen = utils.UnixMsNow()
		if lastLMITimesSeen == GetLmiQuorum() {
			toOutput(msgData, msgSplit)
		}
	}
}

// TODO how to find out which lmhs message corresponds to the latest milestone

func filterLMHSMsg(routine *inputRoutine, msgData []byte, msgSplit []string) {
	if len(msgSplit) < 2 {
		errorf("strange message %v", string(msgData))
		return
	}
	if routine.IsOutputClosed() {
		return // not even checking against the cache
	}
	var entry hashcache.CacheEntry

	lmhsCache.SeenHashBy(msgSplit[1], routine.GetId__(), nil, &entry)
	//infof("+++++ New lmhs '%v' #%v", string(msgData), entry.Visits)

	// if msg is seen QuorumMilestoneHashToPass times during TimeIntervalMilestoneHashToPassMsec
	// it is passed
	if int(entry.Visits) == cfg.Config.QuorumMilestoneHashToPass {
		interv := entry.LastSeen - entry.FirstSeen
		if interv < cfg.Config.TimeIntervalMilestoneHashToPassMsec {
			toOutput(msgData, msgSplit)
			infof("New milestone hash '%v' pass: seen %v times within interval of %v msec", string(msgData), entry.Visits, interv)
		}
	}
}

func getLmiStats() (int, float64) {
	lmiMutex.RLock()
	defer lmiMutex.RUnlock()
	latencySec := float64(lastLMILastSeen-lastLMIFirstSeen) / 1000
	latencySec = math.Round(latencySec*100) / 100
	return lastLMI, latencySec
}
