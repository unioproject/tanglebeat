package main

import (
	"fmt"
	. "github.com/iotaledger/iota.go/consts"
	. "github.com/iotaledger/iota.go/guards/validators"
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/lunfardo314/tanglebeat/bundle_source"
	"github.com/lunfardo314/tanglebeat/confirmer"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/lunfardo314/tanglebeat/multiapi"
	"github.com/lunfardo314/tanglebeat/sender_update"
	"github.com/lunfardo314/tanglebeat/stopwatch"
	"github.com/op/go-logging"
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
	longName := fmt.Sprintf("%v(%v)", params.GetUID(), name)
	bundleSource, err := NewTransferBundleGenerator(longName, params, logger)
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
	ret.log.Infof("Created instance of the sequence %v", ret.GetLongName())
	return &ret, nil
}

func (seq *TransferSequence) GetLongName() string {
	return fmt.Sprintf("%v(%v)", seq.params.GetUID(), seq.name)
}

func createConfirmer(params *senderParamsYAML, logger *logging.Logger) (*confirmer.Confirmer, error) {
	iotaMultiAPI, err := multiapi.New(params.IOTANode, params.TimeoutAPI)
	if err != nil {
		return nil, err
	}
	iotaMultiAPIgTTA, err := multiapi.New(params.IOTANodeTipsel, params.TimeoutTipsel)
	if err != nil {
		return nil, err
	}

	iotaMultiAPIaTT, err := multiapi.New([]string{params.IOTANodePoW}, params.TimeoutPoW)
	if err != nil {
		return nil, err
	}

	txTagPromote := Pad(Trytes(params.TxTagPromote), TagTrinarySize/3)
	err = Validate(ValidateTags(txTagPromote))
	if err != nil {
		return nil, err
	}
	ret := confirmer.Confirmer{
		IotaMultiAPI:          iotaMultiAPI,
		IotaMultiAPIaTT:       iotaMultiAPIaTT,
		IotaMultiAPIgTTA:      iotaMultiAPIgTTA,
		TxTagPromote:          txTagPromote,
		ForceReattachAfterMin: params.ForceReattachAfterMin,
		PromoteChain:          params.PromoteChain,
		PromoteEverySec:       params.PromoteEverySec,
		PromoteDisable:        params.PromoteDisable,
		Log:                   logger,
		AEC:                   AEC,
	}
	return &ret, nil
}

func (seq *TransferSequence) Run() {
	seq.log.Infof("Start running sequence '%v'", seq.name)
	var bundleHash Trytes

	for bundleData := range *seq.bundleSource {
		tail, err := lib.TailFromBundleTrytes(bundleData.BundleTrytes)
		if err != nil {
			seq.log.Errorf("RunConfirm for '%v' returned: %v", seq.GetLongName(), err)
			time.Sleep(5 * time.Second)
			continue
		}
		bundleHash = tail.Bundle

		seq.log.Debugf("Run sequence '%v': start confirming bundle %v", seq.name, bundleHash)
		seq.processStartUpdate(bundleData, bundleHash)

		//run confirmed task and listen to updates
		chUpdate, err := seq.confirmer.StartConfirmerTask(bundleData.BundleTrytes)
		if err != nil {
			seq.log.Errorf("Run sequence '%v': RunConfirm returned: %v", seq.name, err)
			continue
		}
		// read and process updated from confirmer until task is closed
		for updConf := range chUpdate {
			// summing up with stats collected during findOrCreateBundleToConfirm
			if updConf.Err != nil {
				seq.log.Errorf("TransferSequence '%v': confirmer reported an error: %v", seq.GetLongName(), updConf.Err)
			} else {
				updConf.NumAttaches += bundleData.NumAttach
				updConf.TotalDurationATTMsec += bundleData.TotalDurationPoWMs
				updConf.TotalDurationGTTAMsec += bundleData.TotalDurationTipselMs

				seq.processConfirmerUpdate(updConf, bundleData.Addr, bundleData.Index, bundleHash)
			}
		}
		seq.log.Debugf("TransferSequence '%v': finished processing updates for bundle %v", seq.GetLongName(), bundleHash)
	}
	// at this point *seq.bundleSource is closed. It can happen when generator closes channel due to API errors
	// The strategy at the moment is to exit the program with errors altogether. It will be restarted by systemd
	// Alternative might be to restart the sequence caused those errors
	// that would be better strategy but most likely restart won't be significant for the metrics calculation

	seq.log.Errorf("---- !!!!! ---- BundleTrytes generation channel was closed for sequence '%v'. Exiting the program", seq.name)
	os.Exit(8)
}

const securityLevel = 2

func (seq *TransferSequence) processStartUpdate(bundleData *bundle_source.FirstBundleData, bundleHash Hash) {
	var updType sender_update.SenderUpdateType
	if bundleData.IsNew {
		updType = sender_update.SENDER_UPD_START_SEND
	} else {
		updType = sender_update.SENDER_UPD_START_CONTINUE
	}
	seq.log.Debugf("Update '%v' for %v index = %v",
		updType, seq.name, bundleData.Index)

	startTs, updateTs, ok := stopwatch.Get(bundleHash)
	if !ok {
		seq.log.Errorf("No stopwatch entry for bundle hash %v", bundleHash)
	}
	processUpdate(
		"local",
		&sender_update.SenderUpdate{
			Version:               Version,
			SeqUID:                seq.params.GetUID(),
			SeqName:               seq.name,
			UpdType:               updType,
			Index:                 bundleData.Index,
			Addr:                  bundleData.Addr,
			Bundle:                bundleHash,
			StartTs:               startTs,
			UpdateTs:              updateTs,
			NumAttaches:           bundleData.NumAttach,
			NumPromotions:         0,
			NodePOW:               seq.params.IOTANodePoW,
			NodeTipsel:            seq.params.IOTANodeTipsel[0],
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
	addr Hash, index uint64, bundleHash Hash) {

	updType := confirmerUpdType2Sender(updConf.UpdateType)
	seq.log.Debugf("Update '%v' for %v index = %v",
		updType, seq.GetLongName(), index)

	var started, end uint64
	var ok bool
	if updConf.UpdateType == confirmer.UPD_CONFIRM {
		started, end, ok = stopwatch.GetAndRemove(bundleHash)
	} else {
		started, end, ok = stopwatch.Get(bundleHash)
	}
	if !ok {
		seq.log.Errorf("processConfirmerUpdate: No stopwatch entry for %v", bundleHash)
	}
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
			StartTs:               started,
			UpdateTs:              end,
			NumAttaches:           updConf.NumAttaches,
			NumPromotions:         updConf.NumPromotions,
			NodePOW:               seq.params.IOTANodePoW,
			NodeTipsel:            seq.params.IOTANodeTipsel[0],
			PromoteEverySec:       seq.params.PromoteEverySec,
			ForceReattachAfterMin: seq.params.ForceReattachAfterMin,
			PromoteChain:          seq.params.PromoteChain,
			BundleSize:            securityLevel + 1,
			PromoBundleSize:       1,
			TotalPoWMsec:          updConf.TotalDurationATTMsec,
			TotalTipselMsec:       updConf.TotalDurationGTTAMsec,
		})
}
