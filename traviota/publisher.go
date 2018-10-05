package main

import (
	"github.com/lunfardo314/giota"
	"sync"
	"time"
)

type updateType int

const (
	UPD_WAIT     updateType = 0
	UPD_SEND     updateType = 1
	UPD_REATTACH updateType = 2
	UPD_PROMOTE  updateType = 3
	UPD_NOGO     updateType = 4
	UPD_CONFIRM  updateType = 5
	// internal
	UPD_START_SENDING  updateType = 6
	UPD_FINISH_SENDING updateType = 7
)

func (u updateType) String() string {
	switch u {
	case UPD_WAIT:
		return "Wait"
	case UPD_SEND:
		return "Send"
	case UPD_REATTACH:
		return "Reattach"
	case UPD_PROMOTE:
		return "Promote"
	case UPD_NOGO:
		return "Nogo"
	case UPD_CONFIRM:
		return "Confirmed"
		// internal
	case UPD_START_SENDING:
		return "Start sending"
	case UPD_FINISH_SENDING:
		return "Stop sending"
	}
	return "Wrong update type"
}

type senderUpdate struct {
	senderUID               string
	updType                 updateType
	index                   int
	addr                    giota.Address
	numReattaches           int       // number of out bundles in tha tangle
	numPromotions           int       // number of promotions in the current session (starts with 0 after restart)
	sendingStarted          time.Time // time when sending started in this session. Not correct after restart
	sinceSendingMsec        int       // time passed until the update. Based on the same clock as sendingStarted
	avgPoWDurationPerTxMsec int       // total millisec spent on attachToTangle calls / nnumer of tx attached
	avgGTTADurationMsec     int       // total millisec spent on getTransactionsToApproves calls
	// sender's configuration
	bundleSize            int // size of the spending bundle in number of tx
	promoBundleSize       int // size of the promo bundle in number of tx
	promoteEveryNumSec    int
	forceReattachAfterSec int
	promoteNochain        bool // promo strategy. false means 'blowball', true mean 'chain'
}

var inChannel chan *senderUpdate

func initPublisher() func() {
	inChannel = make(chan *senderUpdate)
	var wg sync.WaitGroup

	go func() {
		wg.Add(1)
		defer wg.Done()

		for cmd := range inChannel {
			log.Infof("---- publisher received: sender=%v type='%v' idx=%v address=%v",
				cmd.senderUID, cmd.updType, cmd.index, cmd.addr)
		}
	}()
	return func() {
		close(inChannel)
		wg.Done()
	}
}

func publish(upd *senderUpdate) {
	inChannel <- upd
}

func publishStartSending(index int, addr giota.Address) {
	publish(&senderUpdate{
		updType: UPD_START_SENDING,
		index:   index,
		addr:    addr,
	})
}

func publishFinishSending(index int, addr giota.Address) {
	publish(&senderUpdate{
		updType: UPD_FINISH_SENDING,
		index:   index,
		addr:    addr,
	})
}
