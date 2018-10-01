package main

import (
	"github.com/lunfardo314/giota"
	"net/http"
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
}

func NewSequence(name string) (*Sequence, error) {
	params, err := GetSeqParams(name)
	if err != nil {
		return nil, err
	}
	var ret = Sequence{Name: name, Params: params, SecurityLevel: 2}
	ret.IotaAPI = giota.NewAPI(
		ret.Params.IOTANode[0],
		&http.Client{
			Timeout: time.Duration(ret.Params.TimeoutAPI) * time.Second,
		},
	)
	ret.IotaAPIgTTA = giota.NewAPI(
		ret.Params.IOTANodeGTTA[0],
		&http.Client{
			Timeout: time.Duration(ret.Params.TimeoutGTTA) * time.Second,
		},
	)
	ret.IotaAPIaTT = giota.NewAPI(
		ret.Params.IOTANodeATT[0],
		&http.Client{
			Timeout: time.Duration(ret.Params.TimeoutATT) * time.Second,
		},
	)
	ret.Seed, _ = giota.ToTrytes(ret.Params.Seed)
	ret.TxTag, _ = giota.ToTrytes(ret.Params.TxTag)
	ret.TxTagPromote, _ = giota.ToTrytes(ret.Params.TxTagPromote)

	log.Infof("Created sequence object '%v'", name)
	return &ret, nil
}

func (seq *Sequence) Run() {
	log.Infof("Start running sequence '%v'", seq.Name)
	if addr, err1 := seq.GetAddress(0); err1 == nil {
		log.Infof("%v Address %v", seq.Name, addr)
		log.Infof("%v skaiciuojame balansa", seq.Name)
		if bal, err2 := seq.GetBalanceAddr([]giota.Address{addr}); err2 == nil {
			log.Infof("%v Balance: %v", seq.Name, bal[0])
		} else {
			log.Errorf("%v: ", seq.Name, err2)
		}
	} else {
		log.Error(err1)
	}
}

func (seq *Sequence) GetAddress(index int) (giota.Address, error) {
	ret, err := giota.NewAddress(seq.Seed, index, seq.SecurityLevel)
	if err != nil {
		return "", err
	}
	return ret, nil
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
