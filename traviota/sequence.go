package main

import (
	"errors"
	"fmt"
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/lib"
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

	uid, err := ret.GetUID()
	if err != nil {
		return nil, err
	}
	log.Infof("Created sequence instance: '%v'. UID = %v", name, uid)
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
	return path.Join(Config.SiteDataDir, uid+".last"), err
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
	_, err = fout.WriteString(string(index))
	return err
}

func (seq *Sequence) ReadLastIndex() int {
	fname, err := seq.getLastIndexFname()
	if err != nil {
		return 0
	}
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return 0
	}
	ret, err := strconv.Atoi(string(b))
	if err != nil {
		return 0
	}
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
