package utils

import (
	"os"
	"sync"
)

const fpath = "C:/Users/evaldas/Documents/proj/site_data/wfile.txt"

var fout *os.File
var mx = &sync.Mutex{}

func Wfile(s string) {
	var err error
	mx.Lock()
	defer mx.Unlock()
	if fout == nil {
		fout, err = os.OpenFile(fpath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			panic(err)
		}
	}
	_, _ = fout.Write([]byte(s))
}
