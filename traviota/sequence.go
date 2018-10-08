package main

import (
	"errors"
	"fmt"
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/comm"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/op/go-logging"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

type Sequence struct {
	Name          string
	UID           string
	Params        SenderParams
	IotaAPI       *giota.API
	IotaAPIgTTA   *giota.API
	IotaAPIaTT    *giota.API
	Seed          giota.Trytes
	TxTag         giota.Trytes
	TxTagPromote  giota.Trytes
	SecurityLevel int
	// addr cache
	addrIdxCache   int
	addrCache      giota.Address
	addrCacheMutex sync.Mutex
	log            *logging.Logger
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
	var ret = Sequence{
		Name:          name,
		Params:        params,
		SecurityLevel: 2,
		log:           logger,
		addrIdxCache:  -1,
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

	ret.Seed, err = giota.ToTrytes(ret.Params.Seed)
	if err != nil {
		return nil, err
	}
	ret.UID, err = ret.GetUID()
	if err != nil {
		return nil, err
	}
	ret.TxTag, err = giota.ToTrytes(ret.Params.TxTag)
	if err != nil {
		return nil, err
	}
	ret.TxTagPromote, err = giota.ToTrytes(ret.Params.TxTagPromote)
	if err != nil {
		return nil, err
	}
	ret.log.Infof("Created sequence instance. UID = %v", ret.UID)
	return &ret, nil
}

func (seq *Sequence) Run() {
	index0 := seq.getLastIndex()
	seq.log.Infof("Start running sequence from index0 = %v", index0)

	for index := index0; ; index++ {
		seq.processAddrWithIndex(index)
		seq.log.Debugf("Going to the next index. %v --> %v", index, index+1)
		seq.saveIndex(index)
	}
}

func (seq *Sequence) processAddrWithIndex(index int) {
	addr, err := seq.GetAddress(index)
	if err != nil {
		seq.log.Errorf("Can't get address for idx=%v", index)
	}
	seq.log.Infof("Start processing idx=%v, %v", index, addr)
	inCh, cancelBalanceChan := seq.NewAddrBalanceChan(index)
	defer cancelBalanceChan()

	count := 0
	// processAddrWithIndex only finishes if balance is 0 and address is spent
	// if balance = 0 and address is not spent, loop is waiting for the iotas
	sendingStarted := false
	waitSendingToStop := func() {}
	for s := <-inCh; !(s.balance == 0 && s.isSpent); s = <-inCh {
		if !sendingStarted {
			waitSendingToStop = seq.startSending(addr, index)
			sendingStarted = true
		}
		if count%12 == 0 {
			seq.log.Infof("CURRENT address idx=%v, %v. Balance = %v",
				index, addr, s.balance)
		}
		time.Sleep(5 * time.Second)
		count++
	}
	waitSendingToStop()
	seq.log.Infof("Finished processing: balance == 0 and isSpent. idx=%v, %v", index, addr)
}

func (seq *Sequence) GetAddress(index int) (giota.Address, error) {
	seq.addrCacheMutex.Lock()
	defer seq.addrCacheMutex.Unlock()

	if seq.addrIdxCache == index {
		return seq.addrCache, nil
	}
	var err error
	seq.addrCache, err = giota.NewAddress(seq.Seed, index, seq.SecurityLevel)
	if err == nil {
		seq.addrIdxCache = index
	} else {
		seq.addrIdxCache = -1
	}
	return seq.addrCache, nil
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

func (seq *Sequence) getLastIndexFname() string {
	return path.Join(Config.SiteDataDir, seq.UID)
}

func (seq *Sequence) saveIndex(index int) error {
	fname := seq.getLastIndexFname()
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

func (seq *Sequence) getLastIndex() int {
	fname := seq.getLastIndexFname()
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

func (seq *Sequence) isConfirmed(txHash giota.Trytes) (bool, error) {
	incl, err := seq.IotaAPI.GetLatestInclusion([]giota.Trytes{txHash})
	if err != nil {
		return false, err
	}
	return incl[0], nil
}

func (seq *Sequence) attachToTangle(trunkHash, branchHash giota.Trytes, trytes []giota.Transaction) (*giota.AttachToTangleResponse, error) {
	return seq.IotaAPIaTT.AttachToTangle(&giota.AttachToTangleRequest{
		TrunkTransaction:   trunkHash,
		BranchTransaction:  branchHash,
		Trytes:             trytes,
		MinWeightMagnitude: 14,
	})
}

func (seq *Sequence) sendToNext(addr giota.Address, index int, sendingStats *comm.SendingStats) (giota.Bundle, error) {
	nextAddr, err := seq.GetAddress(index + 1)
	if err != nil {
		return nil, err
	}
	seq.log.Infof("Send. idx=%v. %v --> %v", index, addr, nextAddr)

	gbResp, err := seq.IotaAPI.GetBalances([]giota.Address{addr}, 100)
	if err != nil {
		return nil, err
	}
	balance := gbResp.Balances[0]
	if balance == 0 {
		return nil, errors.New(fmt.Sprintf("Address %v has 0 balance, can't sent to the next.", addr))
	}
	bundle, err := seq.sendBalance(addr, nextAddr, balance, seq.Seed, index, sendingStats)
	if err != nil {
		return nil, err
	}
	// wait until address will acquire isSpent status
	spent, err := seq.IsSpentAddr(addr)
	timeout := 10
	for count := 0; !spent && err == nil; count++ {
		if count > timeout {
			return nil, errors.New("!!!!!!!! Didn't get 'spent' state in 10 seconds")
		}
		time.Sleep(1 * time.Second)
		spent, err = seq.IsSpentAddr(addr)
	}
	return bundle, err
}
