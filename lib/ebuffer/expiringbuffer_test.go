package ebuffer

import (
	"math/rand"
	"sync"
	"testing"
	"time"
)

func Test_EventTSExpiringSegment(t *testing.T) {
	SetLog(nil, true)
	retentionPeriodSec := 15
	bwe := NewEventTsExpiringBuffer("testEventTsExpiringBuffer", 5, retentionPeriodSec)
	wg := &sync.WaitGroup{}
	numrun := 10
	numentries := 10
	for i := 0; i < numrun; i++ {
		wg.Add(1)
		go f(i, bwe, numentries, wg)
	}
	wg.Wait()

	tracef("Start counting")
	wg.Add(1)
	go countLoop(1, bwe, wg)
	wg.Add(1)
	go countLoop(2, bwe, wg)

	tracef("1.Waiting retention period of %v sec + purge loop sleep seconds to expire", retentionPeriodSec)
	time.Sleep(time.Duration(retentionPeriodSec+6) * time.Second)
	nums, nume := bwe.Size()
	tracef("nums = %v nume = %v", nums, nume)
	if nume != 0 || nums != 0 {
		t.Errorf("Buffer not empty after retention period")
	}
	wg.Add(1)
	go f(10, bwe, numentries, wg)
	wg.Wait()

	tracef("2.Waiting retention period of %v sec + purge loop sleep seconds to expire", retentionPeriodSec)
	time.Sleep(time.Duration(retentionPeriodSec+6) * time.Second)
	nums, nume = bwe.Size()
	tracef("nums = %v nume = %v", nums, nume)
	if nume != 0 || nums != 0 {
		t.Errorf("Buffer not empty after retention period")
	}
}

func f(id int, b *EventTsExpiringBuffer, num int, wg *sync.WaitGroup) {
	for i := 0; i < num; i++ {
		b.RecordTS()
		numseg, numentr := b.Size()
		tracef("id = %v inserted entries = %v   numeseg = %v numentries = %v", id, i+1, numseg, numentr)
		rnd := rand.Int() % 5
		time.Sleep(time.Duration(rnd) * time.Second)
	}
	wg.Done()
}

func countLoop(id int, b *EventTsExpiringBuffer, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		c, _ := b.CountAll()
		tracef("%v. CountAll = %v", id, c)
		if c == 0 {
			tracef("%v. Finished CountAll", id)
			return
		}
		time.Sleep(2 * time.Second)
	}
}
