package lib

// https://stackoverflow.com/questions/28796021/how-can-i-log-in-golang-to-a-file-with-log-rotation

import (
	"os"
	"sync"
	"time"
)

type RotatePeriod int

const (
	ROTATE_HOURLY RotatePeriod = 0
	ROTATE_DAYLY  RotatePeriod = 1
)

type RotateWriter struct {
	lock        sync.Mutex
	filename    string // should be set to the actual filename
	rotate      RotatePeriod
	rotateAfter time.Time
	fp          *os.File
}

// Make a new RotateWriter. Return nil if error occurs during setup.
func NewRotateWriter(filename string, t RotatePeriod) *RotateWriter {
	w := &RotateWriter{
		filename:    filename,
		rotate:      t,
		rotateAfter: time.Now(),
	}
	err := w.rotateIfNeeded()
	if err != nil {
		return nil
	}
	return w
}

func (w *RotateWriter) setRotateAfter() {
	nowis := time.Now()
	var rounded time.Time
	switch w.rotate {
	case ROTATE_HOURLY:
		rounded = time.Date(nowis.Year(), nowis.Month(), nowis.Day(), nowis.Hour(), 0, 0, 0, nowis.Location())
		rounded = rounded.Add(1 * time.Hour)
	case ROTATE_DAYLY:
		rounded = time.Date(nowis.Year(), nowis.Month(), nowis.Day(), 0, 0, 0, 0, nowis.Location())
		rounded = rounded.Add(time.Duration(24) * time.Hour)
	}
	w.rotateAfter = rounded
}

// Write satisfies the io.Writer interface.
func (w *RotateWriter) Write(output []byte) (int, error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if err := w.rotateIfNeeded(); err != nil {
		return 0, err
	}
	return w.fp.Write(output)
}

func (w *RotateWriter) rotationNeeded() bool {
	return w.fp == nil || !w.rotateAfter.After(time.Now())
}

// Perform the actual act of rotating and reopening file.
func (w *RotateWriter) rotateIfNeeded() error {
	var err error

	if !w.rotationNeeded() {
		return nil
	}
	// rotation needed

	// Close existing file if open
	if w.fp != nil {
		err = w.fp.Close()
		w.fp = nil
		if err != nil {
			return err
		}
	}
	// Rename dest file if it already exists
	_, err = os.Stat(w.filename)
	if err == nil {
		err = os.Rename(w.filename, w.filename+"."+time.Now().Format(time.RFC3339))
		if err != nil {
			return err
		}
	}

	// Create a file.
	w.fp, err = os.Create(w.filename)
	return err
}
