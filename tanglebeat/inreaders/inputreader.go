package inreaders

import (
	"sync"
	"time"
)

type InputReader interface {
	Lock()
	Unlock()
	GetName() string
	GetState() (bool, bool, time.Time)
	setRunning(bool)
	isRunning() bool

	SetReading(bool)

	GetLastErr() string
	SetLastErr(string)

	SetLastHeartbeatNow()
	GetLastHeartbeat() time.Time
	Run()
}

type InputReaderBase struct {
	name          string
	running       bool
	reading       bool
	lastErr       string
	readingSince  time.Time
	lastHeartbeat time.Time
	mutex         *sync.Mutex
}

func NewInputReaderBase(name string) *InputReaderBase {
	return &InputReaderBase{
		name:          name,
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

func (r *InputReaderBase) GetName() string {
	r.Lock()
	defer r.Unlock()
	return r.name
}

func (r *InputReaderBase) GetState() (bool, bool, time.Time) {
	return r.running, r.reading, r.readingSince
}

func (r *InputReaderBase) SetReading(reading bool) {
	if reading && !r.reading {
		r.readingSince = time.Now()
	}
	r.reading = reading
}

func (r *InputReaderBase) GetLastErr() string {
	return r.lastErr
}

func (r *InputReaderBase) SetLastErr(err string) {
	r.Lock()
	defer r.Unlock()
	r.lastErr = err
}

func (r *InputReaderBase) SetLastHeartbeatNow() {
	r.lastHeartbeat = time.Now()
}

func (r *InputReaderBase) GetLastHeartbeat() time.Time {
	return r.lastHeartbeat
}

func (r *InputReaderBase) isRunning() bool {
	return r.running
}

func (r *InputReaderBase) setRunning(running bool) {
	r.running = running
}
