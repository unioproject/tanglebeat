package main

import (
	"encoding/json"
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/cfg"
	"github.com/lunfardo314/tanglebeat/tanglebeat/zmqpart"
	"math"
	"net"
	"net/url"
	"runtime"
	"sync"
	"time"
)

type GlbStats struct {
	InstanceVersion     string                       `json:"instanceVersion"`
	InstanceStarted     uint64                       `json:"instanceStarted"`
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

var glbStats = &GlbStats{
	InstanceVersion: cfg.Version,
	InstanceStarted: utils.UnixMsNow(),
	mutex:           &sync.RWMutex{},
}

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

func getGlbStatsJSON(formatted bool, maskIP bool, hideInactive bool) []byte {
	glbStats.mutex.RLock()
	defer glbStats.mutex.RUnlock()

	toMarshal := getMaskedGlbStats(maskIP, hideInactive)
	var data []byte
	var err error
	if formatted {
		data, err = json.MarshalIndent(toMarshal, "", "   ")
	} else {
		data, err = json.Marshal(toMarshal)
	}
	if err != nil {
		return []byte(fmt.Sprintf("marshal error: %v", err))
	}
	return data
}

func isActiveRoutine(r *zmqpart.ZmqRoutineStats) bool {
	if !r.Running {
		return false
	}
	if utils.SinceUnixMs(r.LastHeartbeatTs) > 10*10*1000 {
		return false
	}
	return true
}

func isIpAddr(uri string) bool {
	p, err := url.Parse(uri)
	if err != nil {
		return false
	}
	return net.ParseIP(p.Hostname()) != nil
}

func getMaskedGlbStats(maskIP bool, hideInactive bool) *GlbStats {
	if !maskIP && !hideInactive {
		return glbStats
	}

	maskedInputs := make([]*zmqpart.ZmqRoutineStats, 0, len(glbStats.ZmqInputStats))
	for _, inp := range glbStats.ZmqInputStats {
		if !hideInactive || isActiveRoutine(inp) {
			if maskIP && isIpAddr(inp.Uri) {
				tmp := *inp
				tmp.Uri = "IP addr (masked)"
				maskedInputs = append(maskedInputs, &tmp)
			} else {
				maskedInputs = append(maskedInputs, inp)
			}
		}
	}
	ret := *glbStats
	ret.ZmqInputStats = maskedInputs
	return &ret
}
