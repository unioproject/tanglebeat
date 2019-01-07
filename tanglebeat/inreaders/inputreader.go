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
	setIdle__(time.Duration)
	isTimeToRestart__() bool

	SetId__(byte)
	GetId() byte

	isRunning__() bool

	SetReading(bool)

	GetLastErr__() string
	SetLastErr(string)

	SetLastHeartbeatNow()
	GetLastHeartbeat() time.Time
	Run(string)
	GetReaderBaseStats__() *InputReaderBaseStats
}

type InputReaderBase struct {
	id            byte
	running       bool
	reading       bool
	lastErr       string
	restartAt     time.Time
	readingSince  time.Time
	lastHeartbeat time.Time
	mutex         *sync.RWMutex
}

type InputReaderBaseStats struct {
	Running         bool   `json:"running"`
	LastErr         string `json:"lastErr"`
	RunningSinceTs  uint64 `json:"runningSince"`
	LastHeartbeatTs uint64 `json:"lastHeartbeat"`
}

func NewInputReaderBase() *InputReaderBase {
	return &InputReaderBase{
		restartAt:     time.Now(),
		lastHeartbeat: time.Now(),
		mutex:         &sync.RWMutex{},
	}
}

func (r *InputReaderBase) Lock() {
	r.mutex.Lock()
}

func (r *InputReaderBase) Unlock() {
	r.mutex.Unlock()
}

func (r *InputReaderBase) SetId__(id byte) {
	r.id = id
}

func (r *InputReaderBase) GetId() byte {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.id
}

func (r *InputReaderBase) SetReading(reading bool) {
	r.Lock()
	defer r.Unlock()

	if reading && !r.reading {
		r.readingSince = time.Now()
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

func (r *InputReaderBase) setIdle__(restartAfter time.Duration) {
	r.running = false
	r.restartAt = time.Now().Add(restartAfter)
}

func (r *InputReaderBase) isTimeToRestart__() bool {
	return time.Now().After(r.restartAt)
}

func (r *InputReaderBase) GetReaderBaseStats__() *InputReaderBaseStats {
	return &InputReaderBaseStats{
		Running:         r.running && r.reading,
		LastErr:         r.lastErr,
		RunningSinceTs:  utils.UnixMs(r.readingSince),
		LastHeartbeatTs: utils.UnixMs(r.lastHeartbeat),
	}
}
