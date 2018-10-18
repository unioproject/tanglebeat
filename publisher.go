package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/confirmer"
	"github.com/op/go-logging"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/pub"
	"nanomsg.org/go-mangos/transport/tcp"
	"path"
)

const (
	SENDER_UPD_UNDEF          SenderUpdateType = "undef"
	SENDER_UPD_NO_ACTION      SenderUpdateType = "no action"
	SENDER_UPD_START_SEND     SenderUpdateType = "send"
	SENDER_UPD_START_CONTINUE SenderUpdateType = "continue"
	SENDER_UPD_REATTACH       SenderUpdateType = "reattach"
	SENDER_UPD_PROMOTE        SenderUpdateType = "promote"
	SENDER_UPD_CONFIRM        SenderUpdateType = "confirm"
)

type SenderUpdate struct {
	SeqUID           string           `json:"uid"`
	SeqName          string           `json:"nam"`
	UpdType          SenderUpdateType `json:"typ"`
	Index            int              `json:"idx"`
	Addr             giota.Address    `json:"adr"`
	Bundle           giota.Trytes     `json:"bun"`  // bundle hash
	SendingStartedTs int64            `json:"str"`  // time when sending started in this session. Not correct after restart
	UpdateTs         int64            `json:"now"`  // time when the update created. Based on the same clock as sendingStarted
	NumAttaches      int64            `json:"rea"`  // number of out bundles in tha tangle
	NumPromotions    int64            `json:"prom"` // number of promotions in the current session (starts with 0 after restart)
	TotalPoWMsec     int64            `json:"pow"`  // total millisec spent on attachToTangle calls
	TotalTipselMsec  int64            `json:"gtta"` // total millisec spent on getTransactionsToApproves calls
	NodeATT          string           `json:"natt"`
	NodeGTTA         string           `json:"ngta"`
	// sender's configuration
	BundleSize            int  `json:"bsiz"`  // size of the spending bundle in number of tx
	PromoBundleSize       int  `json:"pbsiz"` // size of the promo bundle in number of tx
	PromoteEveryNumSec    int  `json:"psec"`
	ForceReattachAfterMin int  `json:"frm"`
	PromoteChain          bool `json:"chn"` // promo strategy. false means 'blowball', true mean 'chain'
}

func confirmerUpdType2Sender(confUpdType confirmer.UpdateType) SenderUpdateType {
	switch confUpdType {
	case confirmer.UPD_NO_ACTION:
		return SENDER_UPD_NO_ACTION
	case confirmer.UPD_REATTACH:
		return SENDER_UPD_REATTACH
	case confirmer.UPD_PROMOTE:
		return SENDER_UPD_PROMOTE
	case confirmer.UPD_CONFIRM:
		return SENDER_UPD_CONFIRM
	}
	return SENDER_UPD_UNDEF // can't be
}

type SenderUpdateType string

var chanDataToPub chan []byte

func configPublisherLogging() {
	if Config.Publisher.LogConsoleOnly {
		logPub = log
		return
	}
	var err error
	var level logging.Level
	if Config.Debug {
		level = logging.DEBUG
	} else {
		level = logging.INFO
	}
	formatter := logging.MustStringFormatter(Config.Publisher.LogFormat)
	logPub, err = createChildLogger(
		"publisher",
		path.Join(Config.SiteDataDir, Config.Sender.LogDir),
		&masterLoggingBackend,
		&formatter,
		level)
	if err != nil {
		log.Panicf("Can't create publisher log")
	}
}

func publishUpdate(upd *SenderUpdate) error {
	if !Config.MetricsUpdater.Disabled {
		logPub.Debugf("Update metrics '%v' for %v(%v), index = %v",
			upd.UpdType, upd.SeqUID, upd.SeqName, upd.Index)
		updateMetrics(upd)
	}
	if !Config.Publisher.Disabled {
		logPub.Debugf("Publish '%v' for %v(%v), index = %v",
			upd.UpdType, upd.SeqUID, upd.SeqName, upd.Index)
		if err := sendUpdate(upd); err != nil {
			return err
		}
	}
	return nil
}

func initAndRunPublisher() {
	configPublisherLogging()
	err := runPublisher(Config.Publisher.OutPort)
	if err != nil {
		logPub.Errorf("Failed to create publishing channel. Publisher is disabled: %v", err)
		Config.Publisher.Disabled = true
		return
	}
}

// reads input stream of byte arrays and sends them to publish channel
func runPublisher(port int) error {
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

func publishData(data []byte) {
	chanDataToPub <- data
}

func sendUpdate(upd *SenderUpdate) error {
	data, err := json.Marshal(upd)
	if err != nil {
		return err
	}
	publishData(data)
	return nil
}
