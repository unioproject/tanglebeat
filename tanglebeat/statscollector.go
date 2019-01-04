package main

import (
	"encoding/json"
	"fmt"
	"github.com/lunfardo314/tanglebeat/tanglebeat/zmqpart"
	"runtime"
	"sync"
	"time"
)

type GlbStats struct {
	GoRuntimeStats      memStatsStruct                `json:"goRuntimeStats"`
	ZmqRuntimeStats     zmqpart.ZmqRuntimeStatsStruct `json:"zmqRuntimeStats"`
	ZmqOutputStats      zmqpart.ZmqOutputStatsStruct  `json:"zmqOutputStats"`
	ZmqOutputStats10min zmqpart.ZmqOutputStatsStruct  `json:"zmqOutputStats10min"`
	ZmqInputStats       []*zmqpart.ZmqRoutineStats    `json:"zmqInputStats"`

	mutex *sync.RWMutex
}

type memStatsStruct struct {
	MemAllocMB   float32 `json:"memAllocMB"`
	NumGoroutine int     `json:"numGoroutine"`
	mutex        *sync.Mutex
}

var glbStats = &GlbStats{mutex: &sync.RWMutex{}}

func initGlobStatsCollector(refreshEverySec int) {
	zmqpart.InitZmqStatsCollector(refreshEverySec)
	go updateGlbStatsLoop(refreshEverySec)
}

func updateGlbStatsLoop(refreshStatsEverySec int) {
	var mem runtime.MemStats
	for {
		runtime.ReadMemStats(&mem)

		inp := zmqpart.GetInputStats()

		glbStats.mutex.Lock()
		glbStats.ZmqInputStats = inp
		glbStats.ZmqRuntimeStats = *zmqpart.GetRuntimeStats()
		t1, t2 := zmqpart.GetOutputStats()
		glbStats.ZmqOutputStats, glbStats.ZmqOutputStats10min = *t1, *t2

		glbStats.GoRuntimeStats.MemAllocMB = float32(mem.Alloc/1024) / 1024
		glbStats.GoRuntimeStats.NumGoroutine = runtime.NumGoroutine()
		glbStats.mutex.Unlock()

		time.Sleep(time.Duration(refreshStatsEverySec) * time.Second)
	}
}

func getGlbStatsJSON() []byte {
	data, err := json.MarshalIndent(glbStats, "", "   ")
	if err != nil {
		return []byte(fmt.Sprintf("marshal error: %v", err))
	}
	return data
}
