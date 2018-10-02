package main

import (
	"errors"
	"fmt"
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/op/go-logging"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"
)

type Sequence struct {
	Name          string
	Params        SenderParams
	IotaAPI       *giota.API
	IotaAPIgTTA   *giota.API
	IotaAPIaTT    *giota.API
	Seed          giota.Trytes
	TxTag         giota.Trytes
	TxTagPromote  giota.Trytes
	SecurityLevel int
	log           *logging.Logger
}

func NewSequence(name string) (*Sequence, error) {
	params, err := GetSeqParams(name)
	if err != nil {
		return nil, err
	}
	logger, err := createSeqLogger(name)
	if err != nil {
		return nil, err
	}
	var ret = Sequence{
		Name:          name,
		Params:        params,
		SecurityLevel: 2,
		log:           logger,
	}
	ret.IotaAPI = giota.NewAPI(
		ret.Params.IOTANode[0],
		&http.Client{
			Timeout: time.Duration(ret.Params.TimeoutAPI) * time.Second,
		},
	)
	ret.log.Infof("IOTA node: %v, Timeout: %v sec", ret.Params.IOTANode[0], ret.Params.TimeoutAPI)

	ret.IotaAPIgTTA = giota.NewAPI(
		ret.Params.IOTANodeGTTA[0],
		&http.Client{
			Timeout: time.Duration(ret.Params.TimeoutGTTA) * time.Second,
		},
	)
	ret.log.Infof("IOTA node for gTTA: %v, Timeout: %v sec", ret.Params.IOTANodeGTTA[0], ret.Params.TimeoutGTTA)

	ret.IotaAPIaTT = giota.NewAPI(
		ret.Params.IOTANodeATT[0],
		&http.Client{
			Timeout: time.Duration(ret.Params.TimeoutATT) * time.Second,
		},
	)
	ret.log.Infof("IOTA node for ATT: %v, Timeout: %v sec", ret.Params.IOTANodeATT[0], ret.Params.TimeoutATT)

	ret.Seed, _ = giota.ToTrytes(ret.Params.Seed)
	ret.TxTag, _ = giota.ToTrytes(ret.Params.TxTag)
	ret.TxTagPromote, _ = giota.ToTrytes(ret.Params.TxTagPromote)

	uid, err := ret.GetUID()
	if err != nil {
		return nil, err
	}
	ret.log.Infof("Created sequence instance. UID = %v", uid)
	return &ret, nil
}

func createSeqLogger(name string) (*logging.Logger, error) {
	var logger *logging.Logger
	if Config.Sender.LogConsoleOnly {
		logger = log
		log.Infof("Separate logger for the sequence won't be created")
	} else {
		logFname := path.Join(Config.SiteDataDir, Config.Sender.LogDir, PREFIX_MODULE+"."+name+".log")
		fout, err := os.OpenFile(logFname, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
		if err != nil {
			return nil, err
		} else {
			logWriter := io.Writer(fout)
			log.Infof("Created separate logger for the sequence %v", logFname)

			logBackend := logging.NewLogBackend(logWriter, "", 0)
			var formatter logging.Formatter
			if len(Config.Sender.LogFormat) == 0 {
				formatter = logFormatDefault
			} else {
				formatter = logging.MustStringFormatter(Config.Sender.LogFormat)
			}
			logBackendFormatter := logging.NewBackendFormatter(logBackend, formatter)
			seqLoggingBackend := logging.AddModuleLevel(logBackendFormatter)
			if Config.Sender.Globals.Nodebug {
				masterLoggingBackend.SetLevel(logging.INFO, name)
			} else {
				masterLoggingBackend.SetLevel(logging.DEBUG, name)
			}
			logger = logging.MustGetLogger(name)
			logger.SetBackend(logging.MultiLogger(masterLoggingBackend, seqLoggingBackend))
		}
	}
	return logger, nil
}

func (seq *Sequence) Run() {
	seq.log.Infof("Start running sequence")
	if addr, err1 := seq.GetAddress(0); err1 == nil {
		seq.log.Infof("Address %v", addr)
		seq.log.Infof("skaiciuojame balansa")
		if bal, err2 := seq.GetBalanceAddr([]giota.Address{addr}); err2 == nil {
			seq.log.Infof("Balance: %v", bal[0])
		} else {
			seq.log.Error(err2)
		}
	} else {
		seq.log.Error(err1)
	}
}

func (seq *Sequence) GetAddress(index int) (giota.Address, error) {
	ret, err := giota.NewAddress(seq.Seed, index, seq.SecurityLevel)
	if err != nil {
		return "", err
	}
	return ret, nil
}

// returns last 12 trytes of the hash of the seed
func (seq *Sequence) GetUID() (string, error) {
	hash, err := lib.KerlTrytes(seq.Seed)
	if err != nil {
		return "", errors.New(fmt.Sprintf("%v: %v", seq.Name, err))
	}
	ret := string(hash)
	return ret[len(ret)-12:], nil
}

func (seq *Sequence) getLastIndexFname() (string, error) {
	uid, err := seq.GetUID()
	return path.Join(Config.SiteDataDir, uid), err
}

// TODO
func (seq *Sequence) SaveIndex(index int) error {
	fname, err := seq.getLastIndexFname()
	if err != nil {
		return err
	}
	fout, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer fout.Close()
	if _, err = fout.WriteString(fmt.Sprintf("%v", index)); err == nil {
		seq.log.Debugf("Last idx %v saved to %v", index, fname)
	}

	return err
}

func (seq *Sequence) GetLastIndex() int {
	fname, err := seq.getLastIndexFname()
	if err != nil {
		return seq.Params.Index0
	}
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return seq.Params.Index0
	}
	ret, err := strconv.Atoi(string(b))
	if err != nil {
		return seq.Params.Index0
	}

	ret = lib.Max(ret, seq.Params.Index0)
	seq.log.Debugf("Last idx %v read from %v", ret, fname)
	return ret
}

func (seq *Sequence) IsSpentAddr(address giota.Address) (bool, error) {
	if resp, err := seq.IotaAPI.WereAddressesSpentFrom([]giota.Address{address}); err != nil {
		return false, err
	} else {
		return resp.States[0], nil
	}
}

func (seq *Sequence) GetBalanceAddr(addresses []giota.Address) ([]int64, error) {
	if gbResp, err := seq.IotaAPI.GetBalances(addresses, 100); err != nil {
		return nil, err
	} else {
		return gbResp.Balances, nil
	}
}
