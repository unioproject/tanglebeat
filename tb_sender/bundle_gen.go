package main

import (
	"errors"
	"github.com/lunfardo314/giota"
	"github.com/op/go-logging"
	"time"
)

// structure produced by bundle generator
type firstBundleData struct {
	bundle                giota.Bundle // bundle to confirm
	totalDurationATTMsec  int64        // > 0 if new bundle, ==0 if existing bundle
	totalDurationGTTAMsec int64        // ...
	numAttach             int64        // number of tails with the same bundle hash
	started               time.Time    // if new bundle, whe started attach, otherwise		// timestamp of oldest of all tails
}

func NewBundleSource(params *SenderParams, logger *logging.Logger) (chan *firstBundleData, error) {
	if params.externalSource {
		return nil, errors.New("External sources are not implemented")
	}
	return NewTraviotaGenerator(params, logger)

}
