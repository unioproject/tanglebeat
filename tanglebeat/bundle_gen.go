package main

import (
	"errors"
	"github.com/lunfardo314/giota"
	"github.com/op/go-logging"
	"time"
)

// structure produced by bundle generator
type firstBundleData struct {
	addr      giota.Address
	index     int
	bundle    giota.Bundle // bundle to confirm
	isNew     bool         // new bundle created or existing one found
	startTime time.Time    // 	for new bundle, when bundle attach,
	// 	for old bundle timestamp of oldest of all tails
	totalDurationATTMsec  int64 // > 0 if new bundle, ==0 if existing bundle
	totalDurationGTTAMsec int64 // > 0 if new bundle, ==0 if existing bundle
	numAttach             int64 // number of tails with the same bundle hash at the start
}

func NewBundleSource(params *senderParamsYAML, logger *logging.Logger) (chan *firstBundleData, error) {
	if params.externalSource {
		return nil, errors.New("External sources are not implemented")
	}
	return NewTransferBundleGenerator(params, logger)

}
