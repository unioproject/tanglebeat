package main

import (
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/bundle_source"
	"github.com/lunfardo314/tanglebeat/confirmer"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/lunfardo314/tanglebeat/sender_update"
	"github.com/op/go-logging"
	"net/http"
	"os"
	"time"
)

// TODO make Sequences more abstract

type TransferSequence struct {
	// common for all sequences
	bundleSource *bundle_source.BundleSourceChan
	confirmer    *confirmer.Confirmer
	log          *logging.Logger
	name         string
	// specific for sender sequences
	params *senderParamsYAML
}

func NewSequence(name string) (*TransferSequence, error) {
	params, err := getSeqParams(name)
	if err != nil {
		return nil, err
	}
	var logger *logging.Logger
	if Config.Logging.LogConsoleOnly || !Config.Logging.LogSequencesSeparately {
		logger = log
		log.Infof("Separate logger for the sequence won't be created")
	} else {
		logger, err = createChildLogger(
			name,
			Config.Logging.WorkingSubdir,
			&masterLoggingBackend)
		if err != nil {
			return nil, err
		}
	}
	// Creating Traviota style bundle generator hidden behind
	// abstract channel interface for incoming bundles
	bundleSource, err := NewTransferBundleGenerator(params, logger)
	if err != nil {
		return nil, err
	}
	conf, err := createConfirmer(params, logger)
	if err != nil {
		return nil, err
	}
	ret := TransferSequence{
		name:         name,
		params:       params,
		bundleSource: bundleSource,
		confirmer:    conf,
		log:          logger,
	}
	if params.SeqRestartAfterErr > 0 {
		ret.log.Debugf("Exit sequence '%v' after %d consecutive API errors", name, params.SeqRestartAfterErr)
	}
	ret.log.Infof("Created instance of the sequence UID = %v, name = %v", params.GetUID(), name)
	return &ret, nil
}

func createConfirmer(params *senderParamsYAML, logger *logging.Logger) (*confirmer.Confirmer, error) {
	iotaAPI := giota.NewAPI(
		params.IOTANode,
		&http.Client{
			Timeout: time.Duration(params.TimeoutAPI) * time.Second,
		},
	)
	AEC.registerAPI(iotaAPI, params.IOTANode)

	iotaAPIgTTA := giota.NewAPI(
		params.IOTANodeTipsel,
		&http.Client{
			Timeout: time.Duration(params.TimeoutTipsel) * time.Second,
		},
	)
	AEC.registerAPI(iotaAPIgTTA, params.IOTANodeTipsel)

	iotaAPIaTT := giota.NewAPI(
		params.IOTANodePoW,
		&http.Client{
			Timeout: time.Duration(params.TimeoutPoW) * time.Second,
		},
	)
	AEC.registerAPI(iotaAPIaTT, params.IOTANodePoW)

	txTagPromote, err := giota.ToTrytes(params.TxTagPromote)
	if err != nil {
		return nil, err
	}
	ret := confirmer.Confirmer{
		IotaAPI:               iotaAPI,
		IotaAPIaTT:            iotaAPIaTT,
		IotaAPIgTTA:           iotaAPIgTTA,
		TxTagPromote:          txTagPromote,
		ForceReattachAfterMin: params.ForceReattachAfterMin,
		PromoteChain:          params.PromoteChain,
		PromoteEverySec:       int64(params.PromoteEverySec),
		Log:                   logger,
		AEC:                   AEC,
	}
	return &ret, nil
}

func (seq *TransferSequence) Run() {
	seq.log.Info("Start running sequence")
	var bundleHash giota.Trytes

	for bundleData := range *seq.bundleSource {
		seq.processStartUpdate(bundleData)

		bundleHash = bundleData.Bundle.Hash()
		if chUpdate, err := seq.confirmer.RunConfirm(bundleData.Bundle); err != nil {
			seq.log.Errorf("RunConfirm returned: %v", err)
		} else {
			for updConf := range chUpdate {
				// summing up with stats collected during findOrCreateBundleToConfirm
				if updConf.Err != nil {
					seq.log.Errorf("TransferSequence: confirmer reported an error: %v", updConf.Err)
				} else {
					updConf.NumAttaches += bundleData.NumAttach
					updConf.TotalDurationATTMsec += bundleData.TotalDurationPoWMs
					updConf.TotalDurationGTTAMsec += bundleData.TotalDurationTipselMs

					seq.processConfirmerUpdate(
						updConf, bundleData.Addr, bundleData.Index, bundleHash, bundleData.StartTime)
				}
			}
		}
	}
	// at this point *seq.bundleSource is closed. It can happen when generator closes channel due to API errors
	// The strategy at the moment is to exit the program with errors altogether. It will be restarted by systemd
	// Alternative might be to restart the sequence caused those errors
	// that would be better strategy but most likely restart won't be significant for the metrics calculation

	seq.log.Errorf("---- !!!!! ---- Bundle generation channel was closed for sequence '%v'. Exiting the program", seq.name)
	os.Exit(8)
}

const securityLevel = 2

func (seq *TransferSequence) processStartUpdate(bundleData *bundle_source.FirstBundleData) {
	var updType sender_update.SenderUpdateType
	if bundleData.IsNew {
		updType = sender_update.SENDER_UPD_START_SEND
	} else {
		updType = sender_update.SENDER_UPD_START_CONTINUE
	}
	seq.log.Debugf("Update '%v' for %v index = %v",
		updType, seq.params.GetUID(), bundleData.Index)

	processUpdate(
		"local",
		&sender_update.SenderUpdate{
			SeqUID:                seq.params.GetUID(),
			SeqName:               seq.name,
			UpdType:               updType,
			Index:                 bundleData.Index,
			Addr:                  bundleData.Addr,
			Bundle:                bundleData.Bundle.Hash(),
			StartTs:               lib.UnixMs(bundleData.StartTime),
			UpdateTs:              lib.UnixMs(bundleData.StartTime),
			NumAttaches:           bundleData.NumAttach,
			NumPromotions:         0,
			NodePOW:               seq.params.IOTANodePoW,
			NodeTipsel:            seq.params.IOTANodeTipsel,
			PromoteEverySec:       int64(seq.params.PromoteEverySec),
			ForceReattachAfterMin: int64(seq.params.ForceReattachAfterMin),
			PromoteChain:          seq.params.PromoteChain,
			BundleSize:            securityLevel + 1,
			PromoBundleSize:       1,
			TotalPoWMsec:          bundleData.TotalDurationPoWMs,
			TotalTipselMsec:       bundleData.TotalDurationTipselMs,
		})
}

func (seq *TransferSequence) processConfirmerUpdate(updConf *confirmer.ConfirmerUpdate,
	addr giota.Address, index int, bundleHash giota.Trytes, sendingStarted time.Time) {

	updType := confirmerUpdType2Sender(updConf.UpdateType)
	seq.log.Debugf("Update '%v' for %v index = %v",
		updType, seq.params.GetUID(), index)
	processUpdate(
		"local",
		&sender_update.SenderUpdate{
			SeqUID:                seq.params.GetUID(),
			SeqName:               seq.name,
			UpdType:               updType,
			Index:                 index,
			Addr:                  addr,
			Bundle:                bundleHash,
			StartTs:               lib.UnixMs(sendingStarted),
			UpdateTs:              lib.UnixMs(updConf.UpdateTime),
			NumAttaches:           updConf.NumAttaches,
			NumPromotions:         updConf.NumPromotions,
			NodePOW:               seq.params.IOTANodePoW,
			NodeTipsel:            seq.params.IOTANodeTipsel,
			PromoteEverySec:       int64(seq.params.PromoteEverySec),
			ForceReattachAfterMin: int64(seq.params.ForceReattachAfterMin),
			PromoteChain:          seq.params.PromoteChain,
			BundleSize:            securityLevel + 1,
			PromoBundleSize:       1,
			TotalPoWMsec:          updConf.TotalDurationATTMsec,
			TotalTipselMsec:       updConf.TotalDurationGTTAMsec,
		})
}
