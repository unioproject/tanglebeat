package main

import (
	"errors"
	"fmt"
	"github.com/lunfardo314/tanglebeat/tbsender/sender_update"
)

// update is uniquely identified by SeqUID and UpdateTs
// make sure the same is not published twice by saving last ts and skipping
// updates with ts less than that
var alreadyPublished = make(map[string]uint64)

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
		updateSenderMetrics(upd)
	}
	if Config.SenderUpdateCollector.Publish {
		log.Infof("Publish update '%v' received from '%v', seq: %v(%v), index: %v",
			upd.UpdType, sourceName, upd.SeqUID, upd.SeqName, upd.Index)
		if err := updatePublisher.PublishAsJSON(upd); err != nil {
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
			if err = runUpdateCollectorSource(name, srcData.Target); err == nil {
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

func runUpdateCollectorSource(sourceName string, uri string) error {
	if sourceName == "local" {
		return nil
	}

	chIn, err := sender_update.NewUpdateChan(uri)
	if err != nil {
		return errors.New(fmt.Sprintf("failed to initialize sender update source '%v': %v", sourceName, err))

	}
	go func() {
		log.Infof("Start listening external sender update source '%v'", sourceName)
		for upd := range chIn {
			_ = processUpdate(sourceName, upd)
		}
	}()
	return nil
}
