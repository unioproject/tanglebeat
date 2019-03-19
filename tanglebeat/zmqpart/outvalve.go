package zmqpart

import "time"

func startOutValveRoutine() {
	go outputValveLoop()
	infof("Started 'outputValveLoop'")
}

func outputValveLoop() {
	time.Sleep(3 * time.Minute)
	for {
		time.Sleep(2 * time.Minute)

		stats := GetInputStats()
		if len(stats) == 0 {
			continue
		}
		var avgTps float64
		for _, st := range stats {
			avgTps += st.Tps
		}
		avgTps = avgTps / float64(len(stats))
		var numOpen, numClosed int
		for _, st := range stats {
			closeValve := st.Ctps == 0 && st.Tps > 2*avgTps
			st.routine.SetOutputClosed(closeValve)
			if closeValve {
				numClosed++
			} else {
				numOpen++
			}
		}
		infof("Output valve open: %v closed %v", numOpen, numClosed)
	}
}
