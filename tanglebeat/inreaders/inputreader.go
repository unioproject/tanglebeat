package inreaders

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"sync"
	"time"
)

// InputReader is abstract interface to the object with go routine which reads input from
// ZeroMQ, Nanomsg or similar data sources
// Upon i/o error routines stops running. Then starter routine restarts it again after some time

type InputReader interface {
	Lock()
	Unlock()
	setRunning__()
	setIdle__(time.Duration, ReasonNotRunning)
	isTimeToRestart__() bool

	SetId__(byte)
	GetId__() byte

	isRunning__() bool

	SetReading(bool)

	GetLastErr__() string
	SetLastErr(string)

	SetLastHeartbeatNow()
	GetLastHeartbeat() time.Time
	Run(string) ReasonNotRunning
	GetReaderBaseStats__() *InputReaderBaseStats
}

type ReasonNotRunning string

const (
	REASON_NORUN_NONE         = "undef"
	REASON_NORUN_ERROR        = "error"
	REASON_NORUN_ONHOLD_10MIN = "onHold10min"
	REASON_NORUN_ONHOLD_30MIN = "onHold30min"
	REASON_NORUN_ONHOLD_1H    = "onHold1h"
)

type InputReaderBase struct {
	id               byte
	running          bool
	reading          bool
	reasonNotRunning ReasonNotRunning
	onHoldCount      int
	lastErr          string
	restartAt        time.Time
	ReadingSince     time.Time
	lastHeartbeat    time.Time
	mutex            *sync.RWMutex
}

type InputReaderBaseStats struct {
	Running         bool   `json:"running"`
	LastErr         string `json:"lastErr"`
	RunningSinceTs  uint64 `json:"runningSince"`
	LastHeartbeatTs uint64 `json:"lastHeartbeat"`
}

func NewInputReaderBase() *InputReaderBase {
	return &InputReaderBase{
		restartAt:        time.Now(),
		lastHeartbeat:    time.Now(),
		reasonNotRunning: REASON_NORUN_NONE,
		mutex:            &sync.RWMutex{},
	}
}

func (r *InputReaderBase) Lock() {
	r.mutex.Lock()
}

func (r *InputReaderBase) Unlock() {
	r.mutex.Unlock()
}

func (r *InputReaderBase) RLock() {
	r.mutex.RLock()
}

func (r *InputReaderBase) RUnlock() {
	r.mutex.RUnlock()
}

func (r *InputReaderBase) SetId__(id byte) {
	r.id = id
}

func (r *InputReaderBase) GetId__() byte {
	return r.id
}

func (r *InputReaderBase) SetReading(reading bool) {
	r.Lock()
	defer r.Unlock()

	if reading && !r.reading {
		r.ReadingSince = time.Now()
	}
	r.reading = reading
}

func (r *InputReaderBase) GetLastErr__() string {
	return r.lastErr
}

func (r *InputReaderBase) SetLastErr(err string) {
	r.Lock()
	defer r.Unlock()
	r.lastErr = err
}

func (r *InputReaderBase) SetLastHeartbeatNow() {
	r.Lock()
	defer r.Unlock()
	r.lastHeartbeat = time.Now()
}

func (r *InputReaderBase) GetLastHeartbeat() time.Time {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.lastHeartbeat
}

func (r *InputReaderBase) isRunning__() bool {
	return r.running
}

func (r *InputReaderBase) setRunning__() {
	r.running = true
}

func (r *InputReaderBase) setIdle__(restartAfter time.Duration, reason ReasonNotRunning) {
	r.running = false
	r.reasonNotRunning = reason
	r.onHoldCount++
	r.restartAt = time.Now().Add(restartAfter)
}

func (r *InputReaderBase) GetOnHoldInfo__() (int, ReasonNotRunning) {
	return r.onHoldCount, r.reasonNotRunning
}

func (r *InputReaderBase) ResetOnHoldInfo() {
	r.Lock()
	defer r.Unlock()
	r.onHoldCount = 0
	r.reasonNotRunning = REASON_NORUN_NONE
}

func (r *InputReaderBase) isTimeToRestart__() bool {
	return time.Now().After(r.restartAt)
}

func (r *InputReaderBase) GetReaderBaseStats__() *InputReaderBaseStats {
	return &InputReaderBaseStats{
		Running:         r.running && r.reading,
		LastErr:         r.lastErr,
		RunningSinceTs:  utils.UnixMs(r.ReadingSince),
		LastHeartbeatTs: utils.UnixMs(r.lastHeartbeat),
	}
}
