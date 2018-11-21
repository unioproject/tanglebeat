package main

import (
	"fmt"
	. "github.com/iotaledger/iota.go/api"
	. "github.com/iotaledger/iota.go/consts"
	. "github.com/iotaledger/iota.go/guards/validators"
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/lunfardo314/tanglebeat/bundle_source"
	"github.com/lunfardo314/tanglebeat/confirmer"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/lunfardo314/tanglebeat/sender_update"
	"github.com/op/go-logging"
	"os"
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
	bundleSource, err := NewTransferBundleGenerator(name, params, logger)
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
		ret.log.Infof("Exit sequence '%v' after %d consecutive API errors", ret.GetLongName(), params.SeqRestartAfterErr)
	}
	ret.log.Infof("Created instance of the sequence %v", ret.GetLongName())
	return &ret, nil
}

func (seq *TransferSequence) GetLongName() string {
	return fmt.Sprintf("%v(%v)", seq.params.GetUID(), seq.name)
}

func createConfirmer(params *senderParamsYAML, logger *logging.Logger) (*confirmer.Confirmer, error) {
	// TODO add timeouts
	iotaAPI, err := ComposeAPI(
		HTTPClientSettings{URI: params.IOTANode},
	)
	if err != nil {
		return nil, err
	}
	AEC.registerAPI(iotaAPI, params.IOTANode)

	iotaAPIgTTA, err := ComposeAPI(
		HTTPClientSettings{URI: params.IOTANodeTipsel},
	)
	if err != nil {
		return nil, err
	}
	AEC.registerAPI(iotaAPIgTTA, params.IOTANodeTipsel)

	iotaAPIaTT, err := ComposeAPI(
		HTTPClientSettings{URI: params.IOTANodePoW},
	)
	if err != nil {
		return nil, err
	}
	AEC.registerAPI(iotaAPIaTT, params.IOTANodePoW)

	txTagPromote := Pad(Trytes(params.TxTagPromote), TagTrinarySize/3)
	err = Validate(ValidateTags(txTagPromote))
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
		PromoteEverySec:       params.PromoteEverySec,
		Log:                   logger,
		AEC:                   AEC,
	}
	return &ret, nil
}

func (seq *TransferSequence) Run() {
	seq.log.Infof("Start running sequence '%v'", seq.name)
	var bundleHash Trytes

	for bundleData := range *seq.bundleSource {
		seq.processStartUpdate(bundleData)

		//run confirmed task and listen to updates
		if chUpdate, err := seq.confirmer.RunConfirm(bundleData.BundleTrytes); err != nil {
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
				seq.log.Debug("Trace 1")
			}
			seq.log.Debug("Trace 2")
		}
		seq.log.Debug("Trace 3")
	}
	// at this point *seq.bundleSource is closed. It can happen when generator closes channel due to API errors
	// The strategy at the moment is to exit the program with errors altogether. It will be restarted by systemd
	// Alternative might be to restart the sequence caused those errors
	// that would be better strategy but most likely restart won't be significant for the metrics calculation

	seq.log.Errorf("---- !!!!! ---- BundleTrytes generation channel was closed for sequence '%v'. Exiting the program", seq.name)
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
		updType, seq.GetLongName(), bundleData.Index)

	processUpdate(
		"local",
		&sender_update.SenderUpdate{
			Version:               Version,
			SeqUID:                seq.params.GetUID(),
			SeqName:               seq.name,
			UpdType:               updType,
			Index:                 bundleData.Index,
			Addr:                  bundleData.Addr,
			Bundle:                bundleData.BundleHash,
			StartTs:               bundleData.StartTime,
			UpdateTs:              bundleData.StartTime,
			NumAttaches:           bundleData.NumAttach,
			NumPromotions:         0,
			NodePOW:               seq.params.IOTANodePoW,
			NodeTipsel:            seq.params.IOTANodeTipsel,
			PromoteEverySec:       seq.params.PromoteEverySec,
			ForceReattachAfterMin: seq.params.ForceReattachAfterMin,
			PromoteChain:          seq.params.PromoteChain,
			BundleSize:            securityLevel + 1,
			PromoBundleSize:       1,
			TotalPoWMsec:          bundleData.TotalDurationPoWMs,
			TotalTipselMsec:       bundleData.TotalDurationTipselMs,
		})
}

func (seq *TransferSequence) processConfirmerUpdate(updConf *confirmer.ConfirmerUpdate,
	addr Hash, index uint64, bundleHash Hash, sendingStarted uint64) {

	updType := confirmerUpdType2Sender(updConf.UpdateType)
	seq.log.Debugf("Update '%v' for %v index = %v",
		updType, seq.GetLongName(), index)
	processUpdate(
		"local",
		&sender_update.SenderUpdate{
			Version:               Version,
			SeqUID:                seq.params.GetUID(),
			SeqName:               seq.name,
			UpdType:               updType,
			Index:                 index,
			Addr:                  addr,
			Bundle:                bundleHash,
			StartTs:               sendingStarted,
			UpdateTs:              lib.UnixMs(updConf.UpdateTime),
			NumAttaches:           updConf.NumAttaches,
			NumPromotions:         updConf.NumPromotions,
			NodePOW:               seq.params.IOTANodePoW,
			NodeTipsel:            seq.params.IOTANodeTipsel,
			PromoteEverySec:       seq.params.PromoteEverySec,
			ForceReattachAfterMin: seq.params.ForceReattachAfterMin,
			PromoteChain:          seq.params.PromoteChain,
			BundleSize:            securityLevel + 1,
			PromoBundleSize:       1,
			TotalPoWMsec:          updConf.TotalDurationATTMsec,
			TotalTipselMsec:       updConf.TotalDurationGTTAMsec,
		})
}
