package pubsub

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lunfardo314/giota"
	"github.com/op/go-logging"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/pub"
	"nanomsg.org/go-mangos/transport/tcp"
)

var log *logging.Logger

type UpdateType string

const (
	UPD_UNDEF     UpdateType = "undef"
	UPD_NO_ACTION UpdateType = "no action"
	UPD_SEND      UpdateType = "send"
	UPD_REATTACH  UpdateType = "reattach"
	UPD_PROMOTE   UpdateType = "promote"
	UPD_CONFIRM   UpdateType = "confirm"
)

type SendingStats struct {
	NumAttaches           int
	NumPromotions         int
	TotalDurationATTMsec  int64
	NumATT                int
	TotalDurationGTTAMsec int64
	NumGTTA               int
}

type SenderUpdate struct {
	SeqUID                  string        `json:"uid"`
	SeqName                 string        `json:"nam"`
	UpdType                 UpdateType    `json:"typ"`
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

var chanDataToPub chan []byte

// reads input stream of byte arrays and sends them to publish channel
func RunPublisher(port int) error {
	var sock mangos.Socket
	var err error
	if sock, err = pub.NewSocket(); err != nil {
		return errors.New(fmt.Sprintf("can't get new sub socket: %v", err))
	}

	chanDataToPub = make(chan []byte)
	// sock.AddTransport(ipc.NewTransport())
	sock.AddTransport(tcp.NewTransport())
	url := fmt.Sprintf("tcp://localhost:%v", port)
	if err = sock.Listen(url); err != nil {
		return errors.New(fmt.Sprintf("can't listen new pub socket: %v", err))
	}
	go func() {
		defer sock.Close()
		for data := range chanDataToPub {
			err := sock.Send(data)
			if err != nil {
				log.Error(err)
			}
		}
	}()
	return nil
}

func PublishData(data []byte) {
	chanDataToPub <- data
}

func SendUpdate(upd *SenderUpdate) error {
	data, err := json.Marshal(upd)
	if err != nil {
		return err
	}
	PublishData(data)
	return nil
}
