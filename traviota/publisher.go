package main

import (
	"encoding/json"
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
	SenderUID               string        `json:"uid"`
	UpdType                 updateType    `json:"typ"`
	Index                   int           `json:"idx"`
	Addr                    giota.Address `json:"adr"`
	NumReattaches           int           `json:"rea"`  // number of out bundles in tha tangle
	NumPromotions           int           `json:"prom"` // number of promotions in the current session (starts with 0 after restart)
	SendingStartedTs        int64         `json:"str"`  // time when sending started in this session. Not correct after restart
	SinceSendingMs          int64         `json:"now"`  // time passed until the update. Based on the same clock as sendingStarted
	AvgPoWDurationPerTxMsec int           `json:"pow"`  // total millisec spent on attachToTangle calls / nnumer of tx attached
	AvgGTTADurationMsec     int           `json:"gtta"` // total millisec spent on getTransactionsToApproves calls
	// sender's configuration
	BundleSize            int  `json:"bsiz"`  // size of the spending bundle in number of tx
	PromoBundleSize       int  `json:"pbsiz"` // size of the promo bundle in number of tx
	PromoteEveryNumSec    int  `json:"psec"`
	ForceReattachAfterSec int  `json:"fre"`
	PromoteNochain        bool `json:"bb"` // promo strategy. false means 'blowball', true mean 'chain'
}

var inChannel chan *senderUpdate

func initPublisher() func() {
	inChannel = make(chan *senderUpdate)
	sendDataCh, err := ZMQPubChan()
	if err != nil {
		log.Errorf("ZMQPubChan: %v", err)
		log.Panic(err)
	}

	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		defer wg.Done()

		for cmd := range inChannel {
			if msg, err := json.Marshal(cmd); err != nil {
				log.Debug(err)
			} else {
				log.Debugf("---- Sending %v", string(msg))
				sendDataCh <- msg
				//var s senderUpdate
				//json.Unmarshal(msg, s)
				//log.Debugf("%v", s)
			}
			//log.Infof("---- publisher received: sender=%v type='%v' idx=%v address=%v",
			//	cmd.SenderUID, cmd.UpdType, cmd.Index, cmd.Addr)
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
		UpdType: UPD_START_SENDING,
		Index:   index,
		Addr:    addr,
	})
}

func publishFinishSending(index int, addr giota.Address) {
	publish(&senderUpdate{
		UpdType: UPD_FINISH_SENDING,
		Index:   index,
		Addr:    addr,
	})
	time.Now().Unix()
}
