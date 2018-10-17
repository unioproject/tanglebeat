package main

import (
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/confirmer"
	"github.com/op/go-logging"
	"net/http"
	"path"
	"time"
)

type Sequence struct {
	name         string
	params       *SenderParams
	bundleSource chan *firstBundleData
	confirmer    *confirmer.Confirmer
	log          *logging.Logger
}

func NewSequence(name string) (*Sequence, error) {
	params, err := getSeqParams(name)
	if err != nil {
		return nil, err
	}
	var logger *logging.Logger
	if Config.Sender.LogConsoleOnly {
		logger = log
		log.Infof("Separate logger for the sequence won't be created")
	} else {
		var level logging.Level
		if Config.Debug {
			level = logging.DEBUG
		} else {
			level = logging.INFO
		}
		formatter := logging.MustStringFormatter(Config.Publisher.LogFormat)
		logger, err = createChildLogger(
			name,
			path.Join(Config.SiteDataDir, Config.Sender.LogDir),
			&masterLoggingBackend,
			&formatter,
			level)
		if err != nil {
			return nil, err
		}
	}
	bundleSource, err := NewBundleSource(params, logger)
	if err != nil {
		return nil, err
	}
	conf, err := createConfirmer(params, logger)
	if err != nil {
		return nil, err
	}
	ret := Sequence{
		name:         name,
		params:       params,
		bundleSource: bundleSource,
		confirmer:    conf,
		log:          logger,
	}
	ret.log.Infof("Created instance of the sequence UID = %v, name = %v", params.GetUID(), name)
	return &ret, nil
}

func createConfirmer(params *SenderParams, logger *logging.Logger) (*confirmer.Confirmer, error) {
	iotaAPI := giota.NewAPI(
		params.IOTANode[0],
		&http.Client{
			Timeout: time.Duration(params.TimeoutAPI) * time.Second,
		},
	)
	iotaAPIgTTA := giota.NewAPI(
		params.IOTANodeGTTA[0],
		&http.Client{
			Timeout: time.Duration(params.TimeoutGTTA) * time.Second,
		},
	)
	iotaAPIaTT := giota.NewAPI(
		params.IOTANodeATT[0],
		&http.Client{
			Timeout: time.Duration(params.TimeoutATT) * time.Second,
		},
	)
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
	}
	return &ret, nil
}

func (seq *Sequence) Run() {
	seq.log.Info("Start running sequence")
	var bundleHash giota.Trytes

	for bundleData := range seq.bundleSource {
		bundleHash = bundleData.bundle.Hash()
		if chUpdate, err := seq.confirmer.RunConfirm(bundleData.bundle); err != nil {
			seq.log.Errorf("RunConfirm returned: %v", err)
		} else {
			for updConf := range chUpdate {
				// summing up with stats collected during findOrCreateBundleToConfirm
				if updConf.Err != nil {
					seq.log.Errorf("Sequence: confirmer reported an error: %v", updConf.Err)
				} else {
					updConf.NumAttaches += bundleData.numAttach
					updConf.TotalDurationATTMsec += bundleData.totalDurationATTMsec
					updConf.TotalDurationGTTAMsec += bundleData.totalDurationGTTAMsec

					seq.confirmerUpdateToPub(
						updConf, bundleData.addr, bundleData.index, bundleHash, bundleData.startTime)
				}
			}

		}
	}
}
