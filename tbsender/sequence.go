package main

import (
	"fmt"
	. "github.com/iotaledger/iota.go/consts"
	. "github.com/iotaledger/iota.go/guards/validators"
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/op/go-logging"
	"github.com/unioproject/tanglebeat/lib/confirmer"
	"github.com/unioproject/tanglebeat/lib/multiapi"
	"github.com/unioproject/tanglebeat/lib/utils"
	"github.com/unioproject/tanglebeat/tanglebeat/pubupdate"
	"github.com/unioproject/tanglebeat/tbsender/bundle_source"
	"github.com/unioproject/tanglebeat/tbsender/sender_update"
	"os"
	"time"
)

type TransferSequence struct {
	// common for all sequences
	bundleSource *bundle_source.BundleSource
	confirmer    *confirmer.Confirmer
	log          *logging.Logger
	name         string
	iotaMultiAPI multiapi.MultiAPI
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
	// this is needed for the sequence to check balance
	iotaMultiAPI, err := multiapi.New(params.IOTANode, params.TimeoutAPI)
	if err != nil {
		return nil, err
	}

	conf, err := createConfirmer(params, logger)
	if err != nil {
		return nil, err
	}
	ret := TransferSequence{
		name:         name,
		iotaMultiAPI: iotaMultiAPI,
		params:       params,
		bundleSource: bundleSource,
		confirmer:    conf,
		log:          logger,
	}
	ret.log.Infof("Created instance of the sequence %v. Promo tag: %v Promo address: %v",
		ret.GetLongName(), ret.confirmer.TxTagPromote, ret.confirmer.AddressPromote)
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

	var iotaMultiAPIaTT multiapi.MultiAPI
	iotaMultiAPIaTT, err = createPoWAPI(params)
	if err != nil {
		return nil, err
	}

	txTagPromote := Pad(Trytes(params.TxTagPromote), TagTrinarySize/3)
	addressPromote := Trytes(params.AddressPromote)
	err = Validate(ValidateTags(txTagPromote), ValidateHashes(addressPromote))
	if err != nil {
		return nil, err
	}
	return confirmer.NewConfirmer(confirmer.ConfirmerParams{
		IotaMultiAPI:          iotaMultiAPI,
		IotaMultiAPIaTT:       iotaMultiAPIaTT,
		IotaMultiAPIgTTA:      iotaMultiAPIgTTA,
		TxTagPromote:          txTagPromote,
		AddressPromote:        addressPromote,
		ForceReattachAfterMin: params.ForceReattachAfterMin,
		PromoteChain:          params.PromoteChain,
		PromoteEverySec:       params.PromoteEverySec,
		PromoteDisable:        params.PromoteDisable,
		Log:                   logger,
		AEC:                   AEC,
		SlowDownThreshold:     len(Config.Sender.Sequences) * 40,
		ConfmonPollingOnly:    Config.ConfirmationMonitor.UsePollingOnly,
		ConfmonNanozmq:        Config.ConfirmationMonitor.NanoZmq,
	}, nil), nil
}

// checks if balance in the address is enough to confirm a transfer
func (seq *TransferSequence) EnoughBalance(addr Hash, balance uint64) bool {
	var apiret multiapi.MultiCallRet
	bals, err := seq.iotaMultiAPI.GetBalances([]Hash{addr}, 100, &apiret)

	if AEC.CheckError(apiret.Endpoint, err) {
		return true
	}
	return bals.Balances[0] >= balance
}

func (seq *TransferSequence) Run() {
	seq.log.Infof("Start running sequence '%v'", seq.name)
	var bundleHash Trytes
	var bundleData *bundle_source.FirstBundleData
	var finishedOk bool
	for {
		bundleData = seq.bundleSource.GetNextBundleToConfirm()
		if bundleData == nil {
			break // exiting loop, bundle source just closed. It's an error
		}
		bundleHash = bundleData.BundleHash

		seq.log.Debugf("Run sequence '%v': start confirming bundle %v", seq.name, bundleHash)
		seq.processStartUpdate(bundleData, bundleHash)

		//run confirmed task and listen to updates
		chUpdate, cancelConfirmerTask, err := seq.confirmer.StartConfirmerTask(bundleData.BundleTrytes)
		if err != nil {
			seq.log.Errorf("Run sequence '%v': RunConfirm returned: %v", seq.name, err)
			continue
		}
		// read and process updated from confirmer until channel is closed
		finishedOk = false
		balanceCheckedLastTime := time.Now()
		for updConf := range chUpdate {
			if updConf.Err != nil {
				seq.log.Errorf("TransferSequence '%v': confirmer reported an error: %v", seq.GetLongName(), updConf.Err)
			} else {
				updConf.NumAttaches += bundleData.NumAttach
				updConf.TotalDurationATTMsec += bundleData.TotalDurationPoWMs
				updConf.TotalDurationGTTAMsec += bundleData.TotalDurationTipselMs

				if !finishedOk {
					// no updating after confirm
					seq.processConfirmerUpdate(updConf, bundleData.Addr, bundleData.Index, bundleData.Balance, bundleHash)
					if updConf.UpdateType == confirmer.UPD_CONFIRM {
						finishedOk = true
						seq.log.Debugf("TransferSequence '%v': confirmation received for %v. Finish confirmer task",
							seq.GetLongName(), bundleHash)
						cancelConfirmerTask() // confirmer will close the channel
					}
				}
			}
			if !finishedOk {
				// just in case checking if balances is enough.
				// If not, canceling job and sending 'failure' back to the source
				// Checking every 90 sec and it is not enough, canceling the task
				if time.Since(balanceCheckedLastTime) > 90*time.Second {
					if !seq.EnoughBalance(bundleData.Addr, bundleData.Balance) {
						seq.log.Errorf("TransferSequence '%v': not enough balance to confirm %v. cancel confirmer task",
							seq.GetLongName(), bundleHash)
						cancelConfirmerTask()
					}
					balanceCheckedLastTime = time.Now()
				}
			}
		}
		seq.log.Debugf("TransferSequence '%v': finished processing updates for bundle %v. Success = %v",
			seq.GetLongName(), bundleHash, finishedOk)

		// returning result to the bundle source
		seq.bundleSource.PutConfirmationResult(bundleHash, finishedOk)
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

	startTs, updateTs, ok := confirmer.GetStopwatch(bundleHash)
	if !ok {
		seq.log.Errorf("No stopwatch entry for bundle hash %v", bundleHash)
	}
	_ = pubupdate.PublishSenderUpdate(updatePublisher, &sender_update.SenderUpdate{
		Version:               Version,
		SeqUID:                seq.params.GetUID(),
		SeqName:               seq.name,
		UpdType:               updType,
		Index:                 bundleData.Index,
		Balance:               bundleData.Balance,
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
	addr Hash, index uint64, balance uint64, bundleHash Hash) {

	updType := confirmerUpdType2Sender(updConf.UpdateType)
	seq.log.Debugf("Update '%v' for %v index = %v",
		updType, seq.GetLongName(), index)

	var started, end uint64
	var ok bool
	if updConf.UpdateType == confirmer.UPD_CONFIRM {
		started, end, ok = confirmer.GetAndRemoveStopwatch(bundleHash)
	} else {
		started, _, ok = confirmer.GetStopwatch(bundleHash)
		end = utils.UnixMs(updConf.UpdateTime)
	}
	if !ok {
		seq.log.Errorf("processConfirmerUpdate: No stopwatch entry for %v", bundleHash)
	}
	promoTail := Hash("")
	if updConf.UpdateType == confirmer.UPD_PROMOTE {
		promoTail = updConf.PromoteTailHash
	}
	_ = pubupdate.PublishSenderUpdate(updatePublisher,
		&sender_update.SenderUpdate{
			Version:               Version,
			SeqUID:                seq.params.GetUID(),
			SeqName:               seq.name,
			UpdType:               updType,
			Index:                 index,
			Balance:               balance,
			Addr:                  addr,
			Bundle:                bundleHash,
			PromoTail:             promoTail,
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
