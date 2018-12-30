package inreaders

import (
	"sync"
	"time"
)

type InputReader interface {
	Lock()
	Unlock()
	GetState__() (bool, bool, time.Time)
	setRunning__(bool)
	isRunning__() bool

	SetReading(bool)

	GetLastErr__() string
	SetLastErr(string)

	SetLastHeartbeatNow()
	GetLastHeartbeat() time.Time
	Run(string)
}

type InputReaderBase struct {
	running       bool
	reading       bool
	lastErr       string
	readingSince  time.Time
	lastHeartbeat time.Time
	mutex         *sync.Mutex
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

func (r *InputReaderBase) GetState__() (bool, bool, time.Time) {
	return r.running, r.reading, r.readingSince
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
