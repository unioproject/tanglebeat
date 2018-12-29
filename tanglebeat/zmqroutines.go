package main

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	useFirstHashTrytes           = 18 // first positions of the hash will only be used in hash table. To spare memory
	segmentDurationTXMs          = uint64((1 * time.Minute) / time.Millisecond)
	segmentDurationValueTXMs     = uint64((10 * time.Minute) / time.Millisecond)
	segmentDurationValueBundleMs = uint64((10 * time.Minute) / time.Millisecond)
	segmentDurationSNMs          = uint64((1 * time.Minute) / time.Millisecond)
	retentionPeriodMs            = uint64((1 * time.Hour) / time.Millisecond)
)

var (
	txcache          *hashCacheBase
	sncache          *hashCacheSN
	valueTxCache     *hashCacheBase
	valueBundleCache *hashCacheBase
)

func init() {
	txcache = newHashCacheBase(useFirstHashTrytes, segmentDurationTXMs, retentionPeriodMs)
	sncache = newHashCacheSN(useFirstHashTrytes, segmentDurationSNMs, retentionPeriodMs)
	valueTxCache = newHashCacheBase(useFirstHashTrytes, segmentDurationValueTXMs, retentionPeriodMs)
	valueBundleCache = newHashCacheBase(useFirstHashTrytes, segmentDurationValueBundleMs, retentionPeriodMs)
	startCollectingLatencyMetrics(txcache, sncache)
}

type routineStatus struct {
	uri          string
	running      bool
	reading      bool
	lastErr      string
	readingSince time.Time
	lastTX       time.Time
	lastSN       time.Time
	behindTX     *ringArray
	behindSN     *ringArray
	totalTX      uint64
	totalSN      uint64
	mutex        *sync.Mutex
}

const ringArrayLen = 100

func newRoutineStatus(uri string) *routineStatus {
	return &routineStatus{
		uri:      uri,
		behindTX: newRingArray(ringArrayLen),
		behindSN: newRingArray(ringArrayLen),
		mutex:    &sync.Mutex{},
	}
}

var (
	routines      map[string]*routineStatus
	mutexRoutines *sync.Mutex
)

func init() {
	routines = make(map[string]*routineStatus)
	mutexRoutines = &sync.Mutex{}
	go runZmqStarter()
}

func newZmq(uri string) {
	mutexRoutines.Lock()
	defer mutexRoutines.Unlock()

	_, ok := routines[uri]
	if ok {
		return
	}
	routines[uri] = newRoutineStatus(uri)
}

func runZmqStarter() {
	for {
		mutexRoutines.Lock()
		for uri, status := range routines {
			status.mutex.Lock()
			if !routines[uri].running {
				routines[uri].running = true
				go routines[uri].zmqRoutine(uri)
			}
			status.mutex.Unlock()
		}
		mutexRoutines.Unlock()
		time.Sleep(10 * time.Second)
	}
}

var topics = []string{"tx", "sn"}

func (status *routineStatus) zmqRoutine(uri string) {
	infof("Starting zmq routine for %v", uri)
	defer func() {
		status.mutex.Lock()
		defer status.mutex.Unlock()
		routines[uri].running = false
	}()
	defer infof("Leaving zmq routine for %v", uri)

	status.clearCounters()

	socket, err := utils.OpenSocketAndSubscribe(uri, topics)
	if err != nil {
		errorf("Error while starting zmq channel for %v", uri)
		status.mutex.Lock()
		status.lastErr = fmt.Sprintf("%v", err)
		status.mutex.Unlock()
		return
	}

	// updating status to reading
	status.mutex.Lock()
	status.reading = true
	status.readingSince = time.Now()
	status.mutex.Unlock()

	infof("Successfully started zmq routine and channel for %v", uri)
	for {
		msg, err := socket.Recv()
		if err != nil {
			errorf("reading ZMQ socket for '%v': socket.Recv() returned %v", uri, err)

			status.mutex.Lock()
			status.lastErr = fmt.Sprintf("%v", err)
			status.mutex.Unlock()
			return // exit routine
		}
		if len(msg.Frames) == 0 {
			errorf("+++++++++ empty zmq msgSplit for '%v': %+v", uri, msg)
			status.mutex.Lock()
			status.lastErr = fmt.Sprintf("empty msgSplit from zmq")
			status.mutex.Unlock()
			return // exit routine
		}
		msgSplit := strings.Split(string(msg.Frames[0]), " ")

		//updateVecMetrics(msgSplit[0], status.uri)

		switch msgSplit[0] {
		case "tx":
			status.processTXMsg(msg.Frames[0], msgSplit)
		case "sn":
			status.processSNMsg(msg.Frames[0], msgSplit)
		}
	}
}

func (status *routineStatus) processTXMsg(msgData []byte, msgSplit []string) {
	var seen bool
	var behind uint64
	var entry cacheEntry

	if len(msgSplit) < 2 {
		errorf("%v: Message %v is invalid", status.uri, string(msgData))
		return
	}
	seen = txcache.seenHash(msgSplit[1], nil, &entry)

	if seen {
		behind = utils.SinceUnixMs(entry.firstSeen)
	} else {
		behind = 0
	}
	status.mutex.Lock()
	status.lastTX = time.Now()
	status.behindTX.push(behind)
	status.totalTX++
	status.mutex.Unlock()

	toOutput(msgData, msgSplit, entry.visits)
}

func (status *routineStatus) processSNMsg(msgData []byte, msgSplit []string) {
	var hash string
	var seen bool
	var behind uint64
	var index int
	var err error
	var entry cacheEntry

	if len(msgSplit) < 3 {
		errorf("%v: Message %v is invalid", status.uri, string(msgData))
		return
	}
	index, err = strconv.Atoi(msgSplit[1])
	seen = false
	if err != nil {
		errorf("expected index, found %v", msgSplit[1])
		return
	}
	obsolete, whenSeen := sncache.obsoleteIndex(index)
	if obsolete {
		debugf("%v: obsolete 'sn' message: %v. Last index since %v sec ago",
			status.uri, string(msgData), utils.SinceUnixMs(whenSeen)/1000)
		return
	}
	hash = msgSplit[2]

	seen = sncache.seenHash(hash, nil, &entry)
	if seen {
		behind = utils.SinceUnixMs(entry.firstSeen)
	} else {
		behind = 0
	}

	status.mutex.Lock()
	status.lastSN = time.Now()
	status.behindSN.push(behind)
	status.totalSN++
	status.mutex.Unlock()

	toOutput(msgData, msgSplit, entry.visits)
}

func (status *routineStatus) clearCounters() {
	status.mutex.Lock()
	defer status.mutex.Unlock()
	status.readingSince = time.Time{}
	status.lastTX = time.Time{}
	status.lastSN = time.Time{}
	status.totalTX = 0
	status.totalSN = 0
}

type routineStats struct {
	Running           bool    `json:"running"`
	TotalTX           uint64  `json:"totalTX"`
	TotalSN           uint64  `json:"totalSN"`
	SNbyTxPerc        float32 `json:"SNbyTX"`
	AvgBehindTXSec    float32 `json:"avgBehindTXSec"`
	AvgBehindSNSec    float32 `json:"avgBehindSNSec"`
	LeaderTXPerc      uint64  `json:"leaderTXPerc"`
	LeaderSNPerc      uint64  `json:"leaderSNPerc"`
	LastTXMsecAgo     uint64  `json:"lastTXMsecAgo"`
	LastSNMsecAgo     uint64  `json:"lastSNMsecAgo"`
	RunningAlreadyMin float32 `json:"runningAlreadyMin"`
	LastErr           string  `json:"lastErr"`
}

func (status *routineStatus) getStats() *routineStats {
	status.mutex.Lock()
	defer status.mutex.Unlock()
	var errs string
	if status.running && status.reading {
		errs = ""
	} else {
		errs = status.lastErr
	}
	var tmpVal float32
	if status.totalTX != 0 {
		tmpVal = 100 * float32(status.totalSN) / float32(status.totalTX)
	} else {
		tmpVal = -1
	}
	return &routineStats{
		Running:           status.running && status.reading,
		TotalTX:           status.totalTX,
		TotalSN:           status.totalSN,
		SNbyTxPerc:        tmpVal,
		AvgBehindTXSec:    float32(status.behindTX.sum()) / (1000 * float32(status.behindTX.len())),
		AvgBehindSNSec:    float32(status.behindSN.sum()) / (1000 * float32(status.behindSN.len())),
		LeaderTXPerc:      uint64(float32(status.behindTX.numzeros()) * 100 / float32(status.behindTX.len())),
		LeaderSNPerc:      uint64(float32(status.behindSN.numzeros()) * 100 / float32(status.behindSN.len())),
		LastTXMsecAgo:     utils.SinceUnixMs(utils.UnixMs(status.lastTX)),
		LastSNMsecAgo:     utils.SinceUnixMs(utils.UnixMs(status.lastSN)),
		RunningAlreadyMin: float32(utils.SinceUnixMs(utils.UnixMs(status.readingSince))) / 60000,
		LastErr:           errs,
	}
}

func getRoutineStats() map[string]*routineStats {
	mutexRoutines.Lock()
	defer mutexRoutines.Unlock()

	ret := make(map[string]*routineStats)
	for uri, status := range routines {
		ret[uri] = status.getStats()
	}
	return ret
}
