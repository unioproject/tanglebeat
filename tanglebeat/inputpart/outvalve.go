package inputpart

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
		var avgTps, num float64
		for _, st := range stats {
			if st.Ctps > 0 {
				avgTps += st.Tps
				num++
			}
		}
		if num == 0 {
			continue
		}
		avgTps = avgTps / num
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
		infof("Output valve: open %v, closed %v", numOpen, numClosed)
	}
}
