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

func NewInoutReaderBase(name string) *InputReaderBase {
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
	r.Lock()
	defer r.Unlock()
	return r.running, r.reading, r.readingSince
}

func (r *InputReaderBase) setRunning(running bool) {
	r.Lock()
	defer r.Unlock()
	r.running = running
}

func (r *InputReaderBase) SetReading(reading bool) {
	r.Lock()
	defer r.Unlock()
	if reading && !r.reading {
		r.readingSince = time.Now()
	}
	r.reading = reading
}

func (r *InputReaderBase) GetLastErr() string {
	r.Lock()
	defer r.Unlock()
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
