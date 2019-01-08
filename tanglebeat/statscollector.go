package main

import (
	"encoding/json"
	"fmt"
	"github.com/lunfardo314/tanglebeat/tanglebeat/zmqpart"
	"math"
	"runtime"
	"sync"
	"time"
)

type GlbStats struct {
	GoRuntimeStats      memStatsStruct               `json:"goRuntimeStats"`
	ZmqCacheStats       zmqpart.ZmqCacheStatsStruct  `json:"zmqRuntimeStats"`
	ZmqOutputStats      zmqpart.ZmqOutputStatsStruct `json:"zmqOutputStats"`
	ZmqOutputStats10min zmqpart.ZmqOutputStatsStruct `json:"zmqOutputStats10min"`
	ZmqInputStats       []*zmqpart.ZmqRoutineStats   `json:"zmqInputStats"`

	mutex *sync.RWMutex
}

type memStatsStruct struct {
	MemAllocMB   float64 `json:"memAllocMB"`
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
		glbStats.ZmqCacheStats = *zmqpart.GetZmqCacheStats()
		t1, t2 := zmqpart.GetOutputStats()
		glbStats.ZmqOutputStats, glbStats.ZmqOutputStats10min = *t1, *t2

		glbStats.GoRuntimeStats.MemAllocMB = math.Round(100*(float64(mem.Alloc/1024)/1024)) / 100
		glbStats.GoRuntimeStats.NumGoroutine = runtime.NumGoroutine()
		glbStats.mutex.Unlock()

		time.Sleep(time.Duration(refreshStatsEverySec) * time.Second)
	}
}

func getGlbStatsJSON(formatted bool) []byte {
	glbStats.mutex.RLock()
	defer glbStats.mutex.RUnlock()

	var data []byte
	var err error
	if formatted {
		data, err = json.MarshalIndent(glbStats, "", "   ")
	} else {
		data, err = json.Marshal(glbStats)
	}
	if err != nil {
		return []byte(fmt.Sprintf("marshal error: %v", err))
	}
	return data
}
