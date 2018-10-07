package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/lib"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/pub"
	"nanomsg.org/go-mangos/transport/tcp"
	"time"
)

type updateType int

const (
	UPD_UNDEF          updateType = 0
	UPD_WAIT           updateType = 1
	UPD_SEND           updateType = 2
	UPD_REATTACH       updateType = 3
	UPD_PROMOTE        updateType = 4
	UPD_NOGO           updateType = 5
	UPD_CONFIRM        updateType = 6
	UPD_START_SENDING  updateType = 7
	UPD_FINISH_SENDING updateType = 8
)

var (
	chanUpdates       chan *senderUpdate
	chanDataToPublish chan []byte
)

// reads input stream of byte arrays and sends them to publish channel
func initChanDataPublish() error {
	if Config.Publisher.Disabled {
		return errors.New("publisher is disabled, 'chanDataToPublish' channel wasn't be created")
	}
	var sock mangos.Socket
	var err error
	if sock, err = pub.NewSocket(); err != nil {
		return errors.New(fmt.Sprintf("can't get new sub socket: %v", err))
	}

	chanDataToPublish = make(chan []byte)
	// sock.AddTransport(ipc.NewTransport())
	sock.AddTransport(tcp.NewTransport())
	log.Infof("Publisher port is %v", Config.Publisher.OutPort)
	url := fmt.Sprintf("tcp://localhost:%v", Config.Publisher.OutPort)
	if err = sock.Listen(url); err != nil {
		return errors.New(fmt.Sprintf("can't listen new pub socket: %v", err))
	}
	go func() {
		defer sock.Close()
		for data := range chanDataToPublish {
			log.Debugf("======= data received from chanDataToPublish")
			err := sock.Send(data)
			if err != nil {
				log.Errorf("======= chanDataToPublish.Send error: %v Data='%v'", err, string(data))
			} else {
				log.Debugf("======= data sent to chanDataToPublish")
			}
		}
	}()
	return nil
}

//func initChanDataToZMQ() error {
//	if Config.Publisher.Disabled {
//		return errors.New("publisher is disabled, 'chanDataToPublish' channel wasn't be created")
//	}
//
//	chanDataToPublish = make(chan []byte)
//	pub := zmq4.NewPub(context.Background())
//
//	log.Infof("Publisher ZMQ port is %v", Config.Publisher.OutPort)
//	err := pub.Listen(fmt.Sprintf("tcp://*:%v", Config.Publisher.OutPort))
//	if err != nil {
//		return err
//	}
//	go func() {
//		defer pub.Close()
//		for data := range chanDataToPublish {
//			log.Debugf("======= data received from chanDataToPublish")
//			msg := zmq4.NewMsg(data)
//			err := pub.Send(msg)
//			if err != nil {
//				log.Errorf("======= chanDataToPublish.zm4.Send error: %v Data='%v'", err, string(data))
//			} else {
//				log.Debugf("======= data sent to zmq chanDataToPublish")
//			}
//		}
//	}()
//	return nil
//}

func publishData(data []byte) {
	if !Config.Publisher.Disabled {
		chanDataToPublish <- data
	}
}

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
	return "Undef"
}

type senderUpdate struct {
	SenderUID               string        `json:"uid"`
	UpdType                 updateType    `json:"typ"`
	Index                   int           `json:"idx"`
	Addr                    giota.Address `json:"adr"`
	NumAttaches             int           `json:"rea"`  // number of out bundles in tha tangle
	NumPromotions           int           `json:"prom"` // number of promotions in the current session (starts with 0 after restart)
	SendingStartedTs        int64         `json:"str"`  // time when sending started in this session. Not correct after restart
	SinceSendingMsec        int64         `json:"now"`  // time passed until the update. Based on the same clock as sendingStarted
	AvgPoWDurationPerTxMsec int64         `json:"pow"`  // total millisec spent on attachToTangle calls / nnumer of tx attached
	AvgGTTADurationMsec     int64         `json:"gtta"` // total millisec spent on getTransactionsToApproves calls
	NodeATT                 string        `json:"natt"`
	NodeGTTA                string        `json:"ngta"`
	// sender's configuration
	BundleSize            int     `json:"bsiz"`  // size of the spending bundle in number of tx
	PromoBundleSize       int     `json:"pbsiz"` // size of the promo bundle in number of tx
	PromoteEveryNumSec    int     `json:"psec"`
	ForceReattachAfterSec int     `json:"fre"`
	PromoteNochain        bool    `json:"bb"`  // promo strategy. false means 'blowball', true mean 'chain'
	TPS                   float32 `json:"tps"` // contribution to tps
}

func initPublisher() {
	if Config.Publisher.Disabled {
		log.Infof("Publisher is disabled!!!")
		return
	}
	chanUpdates = make(chan *senderUpdate)
	err := initChanDataPublish()
	if err != nil {
		log.Errorf("Failed to create publishing channel. Publisher is disabled: %v", err)
		Config.Publisher.Disabled = true
		return
	}
	go func() {
		for update := range chanUpdates {
			if data, err := json.Marshal(update); err != nil {
				log.Errorf("json.Marshal:", err)
			} else {
				publishData(data)
			}
		}
	}()
}

func publishUpdate(upd *senderUpdate) {
	if !Config.Publisher.Disabled {
		chanUpdates <- upd
	}
}

func (seq *Sequence) publishState(state *sendingState, updType updateType) {
	upd := senderUpdate{
		SenderUID:             seq.UID,
		UpdType:               updType,
		Index:                 state.index,
		Addr:                  state.addr,
		SendingStartedTs:      lib.UnixMs(state.sendingStarted),
		NumAttaches:           state.numAttach,
		NumPromotions:         state.numPromote,
		NodeATT:               seq.Params.IOTANodeATT[0],
		NodeGTTA:              seq.Params.IOTANodeGTTA[0],
		PromoteEveryNumSec:    seq.Params.PromoteEverySec,
		ForceReattachAfterSec: seq.Params.ForceReattachAfterMin,
		PromoteNochain:        seq.Params.PromoteNoChain,
	}
	timeSinceStart := time.Since(state.sendingStarted)
	timeSinceStartMsec := int64(timeSinceStart / time.Millisecond)
	upd.SinceSendingMsec = timeSinceStartMsec
	securityLevel := 2
	upd.BundleSize = securityLevel + 2
	upd.PromoBundleSize = 1
	totalTx := upd.BundleSize*upd.NumAttaches + upd.PromoBundleSize*upd.NumPromotions
	if state.numATT != 0 {
		upd.AvgPoWDurationPerTxMsec = int64(state.totalDurationATTMsec / (state.numATT * totalTx))
	}
	if state.numGTTA != 0 {
		upd.AvgGTTADurationMsec = int64(state.totalDurationGTTAMsec / state.numGTTA)
	}
	timeSinceStartSec := float32(timeSinceStartMsec) / float32(1000)
	if timeSinceStartSec > 0.1 {
		upd.TPS = float32(totalTx) / timeSinceStartSec
	} else {
		upd.TPS = 0
	}

	publishUpdate(&upd)
}
