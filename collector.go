package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/confirmer"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/tcp"
	"time"
)

// TODO routing

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

// update is uniquely identified by SeqUID and UpdateTs
// make sure the same is not published twice by saving last ts and skipping
// updates with ts less than that
var alreadyPublished map[string]int64 = make(map[string]int64)

// this is called by collector or local sender every time update arrives
// Updates are used to calculate sender metrics.
// Update are published is enabled
func processUpdate(sourceName string, upd *SenderUpdate) error {
	if src, ok := Config.SenderDataCollector.Sources[sourceName]; !ok || !src.Enabled {
		// source is disabled, do nothing
		return nil
	}
	log.Infof("Processing update '%v' source: '%v', seq: %v(%v), index: %v",
		upd.UpdType, sourceName, upd.SeqUID, upd.SeqName, upd.Index)

	ts, ok := alreadyPublished[upd.SeqUID]
	if ok && upd.UpdateTs <= ts {
		return nil // same update received twice, skip it
	}
	alreadyPublished[upd.SeqUID] = upd.UpdateTs

	if Config.Prometheus.Enabled && Config.Prometheus.SenderMetricsEnabled {
		log.Debugf("Update metrics for %v(%v), index = %v",
			upd.SeqUID, upd.SeqName, upd.Index)
		updateSenderMetrics(upd)
	}
	if Config.SenderDataCollector.Publish {
		log.Infof("Publish update '%v' for %v(%v) from '%v', index = %v",
			upd.UpdType, upd.SeqUID, upd.SeqName, sourceName, upd.Index)
		if err := publishUpdate(upd); err != nil {
			return err
		}
	}
	return nil
}

func initSenderDataCollector() {
	var count int
	var err error
	for name, srcData := range Config.SenderDataCollector.Sources {
		if !srcData.Enabled {
			log.Infof("Sender data collector source '%v' DISABLED", name)
			continue
		} else {
			if err = runDataCollectorSource(name, srcData.Target); err == nil {
				count += 1
				log.Infof("Sender data collector source '%v' ENABLED: target = %v", name, srcData.Target)
			} else {
				log.Errorf("Failed to initialize sender data collector source '%v': %v", name, err)
				srcData.Enabled = false
			}
		}
	}
	log.Infof("Number sender data collector sources initialized successfully: %v", count)

}

func runDataCollectorSource(sourceName string, uri string) error {
	if sourceName == "local" {
		return nil
	}

	var sock mangos.Socket
	var err error

	if sock, err = sub.NewSocket(); err != nil {
		return errors.New(fmt.Sprintf("sender update source '%v': Can't get new sub socket: %v", sourceName, err))
	}
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Dial(uri); err != nil {
		return errors.New(fmt.Sprintf("sender update source '%v': Can't dial sub socket: %v", sourceName, err))
	}
	err = sock.SetOption(mangos.OptionSubscribe, []byte(""))
	if err != nil {
		return errors.New(fmt.Sprintf("sender update source '%v'. Can't subscribe to all topics: %v", sourceName, err))
	}
	var msg []byte
	var upd *SenderUpdate
	go func() {
		log.Infof("Start listening external sender update source '%v'", sourceName)
		defer sock.Close()
		for {
			msg, err = sock.Recv()
			if err == nil {
				upd = &SenderUpdate{}
				err = json.Unmarshal(msg, &upd)
				if err == nil {
					log.Infof("Received '%v' update from source '%v': seq = %v(%v)",
						upd.UpdType, sourceName, upd.SeqUID, upd.SeqName)
					processUpdate(sourceName, upd)
				} else {
					log.Errorf("Error while receiving sender update from source %v(%v): %v", sourceName, uri, err)
					time.Sleep(2 * time.Second)
				}
			}
		}
	}()
	return nil
}
