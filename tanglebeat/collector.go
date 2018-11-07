package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lunfardo314/tanglebeat/confirmer"
	"github.com/lunfardo314/tanglebeat/sender_update"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/tcp"
	"time"
)

func confirmerUpdType2Sender(confUpdType confirmer.UpdateType) sender_update.SenderUpdateType {
	switch confUpdType {
	case confirmer.UPD_NO_ACTION:
		return sender_update.SENDER_UPD_NO_ACTION
	case confirmer.UPD_REATTACH:
		return sender_update.SENDER_UPD_REATTACH
	case confirmer.UPD_PROMOTE:
		return sender_update.SENDER_UPD_PROMOTE
	case confirmer.UPD_CONFIRM:
		return sender_update.SENDER_UPD_CONFIRM
	}
	return sender_update.SENDER_UPD_UNDEF // can't be
}

// update is uniquely identified by SeqUID and UpdateTs
// make sure the same is not published twice by saving last ts and skipping
// updates with ts less than that
var alreadyPublished = make(map[string]int64)

// this is called by updates or local sender every time update arrives
// Updates are used to calculate sender metrics.
// Updates are published if publisher is enabled
func processUpdate(sourceName string, upd *sender_update.SenderUpdate) error {
	if src, ok := Config.SenderUpdateCollector.Sources[sourceName]; !ok || !src.Enabled {
		// source is disabled, do nothing
		return nil
	}
	log.Infof("Processing update '%v', source: '%v', seq: %v(%v), index: %v",
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
	if Config.SenderUpdateCollector.Publish {
		log.Infof("Publish update '%v' received from '%v', seq: %v(%v), index: %v",
			upd.UpdType, sourceName, upd.SeqUID, upd.SeqName, upd.Index)
		if err := publishUpdate(upd); err != nil {
			log.Errorf("Process update: %v", err)
			return err
		}
	}
	return nil
}

func initSenderDataCollector() {
	var count int
	var err error
	log.Infof("Starting sender data updates sources")
	for name, srcData := range Config.SenderUpdateCollector.Sources {
		if !srcData.Enabled {
			log.Infof("Sender data updates source '%v' DISABLED", name)
			continue
		} else {
			if err = runDataCollectorSource(name, srcData.Target); err == nil {
				count += 1
				log.Infof("Sender data updates source '%v' ENABLED: target = %v", name, srcData.Target)
			} else {
				log.Errorf("Failed to initialize sender data updates source '%v': %v", name, err)
				srcData.Enabled = false
			}
		}
	}
	log.Infof("Number sender data updates sources initialized successfully: %v", count)
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
	var upd *sender_update.SenderUpdate
	go func() {
		log.Infof("Start listening external sender update source '%v'", sourceName)
		defer sock.Close()
		for {
			msg, err = sock.Recv()
			if err == nil {
				upd = &sender_update.SenderUpdate{}
				err = json.Unmarshal(msg, &upd)
				if err == nil {
					log.Debugf("Received '%v' update from source '%v': seq = %v(%v)",
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
