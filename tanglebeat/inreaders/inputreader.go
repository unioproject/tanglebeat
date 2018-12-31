package inreaders

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"sync"
	"time"
)

type InputReader interface {
	Lock()
	Unlock()
	setRunning__(bool)
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
	running       bool
	reading       bool
	lastErr       string
	readingSince  time.Time
	lastHeartbeat time.Time
	mutex         *sync.Mutex
}

type InputReaderBaseStats struct {
	Running         bool   `json:"running"`
	LastErr         string `json:"lastErr"`
	RunningSinceTs  uint64 `json:"runningSince"`
	LastHeartbeatTs uint64 `json:"lastHeartbeat"`
}

func NewInputReaderBase() *InputReaderBase {
	return &InputReaderBase{
		lastHeartbeat: time.Now(),
		mutex:         &sync.Mutex{},
	}
}

func (r *InputReaderBase) Lock() {
	r.mutex.Lock()
}

func (r *InputReaderBase) Unlock() {
	r.mutex.Unlock()
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
	r.Lock()
	defer r.Unlock()
	return r.lastHeartbeat
}

func (r *InputReaderBase) isRunning__() bool {
	return r.running
}

func (r *InputReaderBase) setRunning__(running bool) {
	r.running = running
}

func (r *InputReaderBase) GetReaderBaseStats__() *InputReaderBaseStats {
	return &InputReaderBaseStats{
		Running:         r.running && r.reading,
		LastErr:         r.lastErr,
		RunningSinceTs:  utils.UnixMs(r.readingSince),
		LastHeartbeatTs: utils.UnixMs(r.lastHeartbeat),
	}
}
