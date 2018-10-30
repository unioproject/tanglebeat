package main

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib"
	"os"
	"path/filepath"
	"time"
)

// testing lib.rotatelog

const (
	path   = "C:/Users/evaldas/Documents/proj/site_data/log"
	fname  = "rwtest.log"
	period = 10 * time.Second
	retain = 1 * time.Minute
)

func main() {
	fmt.Printf("Creating rotating writer: %v -- %v -- %v\n",
		filepath.Join(path, fname), period, retain)
	rw, err := lib.NewRotateWriter(path, fname, period, retain)
	if err != nil {
		fmt.Println("Failed: %v", err)
		os.Exit(1)
	}
	for {
		nowis := time.Now()
		t := fmt.Sprintf("%v\n", nowis)
		num, err := rw.Write([]byte(t))
		fmt.Printf("--> %v -- num = %v err = %v\n", nowis, num, err)
		time.Sleep(1 * time.Second)
	}

}
