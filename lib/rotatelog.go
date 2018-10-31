package lib

// https://stackoverflow.com/questions/28796021/how-can-i-log-in-golang-to-a-file-with-log-rotation

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type RotateWriter struct {
	lock         sync.Mutex
	dir          string // directory where to put logs
	filename     string // should be set to the actual filename, without path
	rotatePeriod time.Duration
	retainPeriod time.Duration
	rotateAfter  time.Time
	fp           *os.File
}

// Make a new RotateWriter. Return nil if error occurs during setup.
func NewRotateWriter(dir string, filename string, rot time.Duration, retain time.Duration) (*RotateWriter, error) {
	w := &RotateWriter{
		dir:          dir,
		filename:     filename,
		rotatePeriod: rot,
		retainPeriod: retain,
		rotateAfter:  time.Now(),
	}
	err := w.rotateIfNeeded()
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (w *RotateWriter) fullPath() string {
	return filepath.Join(w.dir, w.filename)
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

func (w *RotateWriter) purgeOlder() {
	files, err := ioutil.ReadDir(w.dir)
	if err != nil {
		return
	}
	purgeOlderThan := time.Now().Add(-w.retainPeriod)
	//fmt.Printf("--- purge before %v\n", purgeOlderThan)
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if !strings.HasPrefix(f.Name(), w.filename) || f.Name() == w.filename {
			continue
		}
		//fmt.Printf("--- %v -- mod time %v\n", f.Name(), f.ModTime())
		if f.ModTime().After(purgeOlderThan) {
			continue
		}
		toremove := filepath.Join(w.dir, f.Name())
		//fmt.Printf("Remove %v\n", toremove)
		os.Remove(toremove)
	}
}

// Perform the actual act of rotating and reopening file.
func (w *RotateWriter) rotateIfNeeded() error {
	var err error

	if !w.rotationNeeded() {
		return nil
	}
	// rotation needed
	// Purge older files
	w.purgeOlder()
	// Close existing file if open
	if w.fp != nil {
		m := fmt.Sprintf("---------------- leaving rotating log %v at %v\n", w.fullPath(), time.Now())
		_, err = w.fp.Write([]byte(m))
		if err != nil {
			return err
		}
		err = w.fp.Close()
		w.fp = nil
		if err != nil {
			return err
		}
	}
	// Rename dest file if it already exists
	_, err = os.Stat(w.fullPath())
	stamp := time.Now().Format(time.Stamp)
	stamp = strings.Replace(stamp, ":", "_", -1)
	if err == nil {
		err = os.Rename(w.fullPath(), w.fullPath()+"."+stamp)
		if err != nil {
			return err
		}
	}

	// Create a file.
	w.fp, err = os.OpenFile(w.fullPath(), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	w.rotateAfter = time.Now().Add(w.rotatePeriod)
	m := fmt.Sprintf("---------------- rotating log %v created %v\n", w.fullPath(), time.Now())
	_, err = w.fp.Write([]byte(m))
	return err
}
