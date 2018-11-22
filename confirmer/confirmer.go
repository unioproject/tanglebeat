package confirmer

import (
	. "github.com/iotaledger/iota.go/api"
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/op/go-logging"
	"strings"
	"sync"
	"time"
)

type UpdateType int

const (
	UPD_NO_ACTION UpdateType = 0
	UPD_REATTACH  UpdateType = 1
	UPD_PROMOTE   UpdateType = 2
	UPD_CONFIRM   UpdateType = 3
)

type Confirmer struct {
	IotaAPI               *API
	IotaAPIgTTA           *API
	IotaAPIaTT            *API
	TxTagPromote          Trytes
	ForceReattachAfterMin uint64
	PromoteChain          bool
	PromoteEverySec       uint64
	Log                   *logging.Logger
	AEC                   lib.ErrorCounter
	// internal
	chanUpdate chan *ConfirmerUpdate
	mutex      sync.Mutex //task state access sync
	// confirmer task state
	lastBundleTrytes      []Trytes
	nextForceReattachTime time.Time
	numAttach             uint64
	nextPromoTime         time.Time
	nextTailHashToPromote Hash
	numPromote            uint64
	totalDurationATTMsec  uint64
	totalDurationGTTAMsec uint64
	isNotPromotable       bool
}

type ConfirmerUpdate struct {
	NumAttaches           uint64
	NumPromotions         uint64
	TotalDurationATTMsec  uint64
	TotalDurationGTTAMsec uint64
	UpdateTime            time.Time
	UpdateType            UpdateType
	Err                   error
}

func (conf *Confirmer) debugf(f string, p ...interface{}) {
	if conf.Log != nil {
		conf.Log.Debugf(f, p...)
	}
}

func (conf *Confirmer) errorf(f string, p ...interface{}) {
	if conf.Log != nil {
		conf.Log.Errorf(f, p...)
	}
}

func (conf *Confirmer) warningf(f string, p ...interface{}) {
	if conf.Log != nil {
		conf.Log.Warningf(f, p...)
	}
}

type dummy struct{}

func (*dummy) IncErrorCount(api *API) {}

func (conf *Confirmer) StartConfirmerTask(bundleTrytes []Trytes) (Hash, chan *ConfirmerUpdate, func(), error) {
	//if err := lib.CheckBundle(bundle); err != nil {
	//	return nil, nil, errors.New(fmt.Sprintf("Attempt to run confirmer with wrong bundle: %v", err))
	//}

	tail, err := lib.TailFromBundleTrytes(bundleTrytes)
	if err != nil {
		return "", nil, nil, err
	}
	nowis := time.Now()
	conf.lastBundleTrytes = bundleTrytes
	conf.nextForceReattachTime = nowis.Add(time.Duration(conf.ForceReattachAfterMin) * time.Minute)
	conf.nextPromoTime = nowis
	conf.nextTailHashToPromote = tail.Hash
	conf.isNotPromotable = false
	conf.chanUpdate = make(chan *ConfirmerUpdate)
	conf.numAttach = 0
	conf.numPromote = 0
	conf.totalDurationGTTAMsec = 0
	conf.totalDurationATTMsec = 0
	if conf.AEC == nil {
		conf.AEC = &dummy{}
	}

	cancelPromo := conf.goPromote()
	cancelReattach := conf.goReattach()

	return tail.Bundle, conf.chanUpdate, func() {
		conf.Log.Debugf("CONFIRMER: canceling confirmer task for %v", tail.Bundle)
		cancelPromo()
		cancelReattach()
		close(conf.chanUpdate)
	}, nil
}

func (conf *Confirmer) RunConfirm(bundleTrytes []Trytes) (chan *ConfirmerUpdate, error) {
	// start promote and reattach routines
	bhash, chUpd, cancelFun, err := conf.StartConfirmerTask(bundleTrytes)
	if err != nil {
		return nil, err
	}

	// wait until any bundle with the hash is confirmed
	go func() {
		started := time.Now()
		conf.debugf("CONFIRMER: confirmer task started")
		defer conf.debugf("CONFIRMER: confirmer task ended")
		defer cancelFun()

		for {
			since := time.Now().Sub(started)
			if since > 15*time.Minute {
				conf.warningf("----- CONFIRMER: it takes longer than %v to confirm the bundle", since)
				time.Sleep(5 * time.Second)
			}
			if confirmed, err := conf.isBundleHashConfirmed(bhash); err != nil {
				conf.errorf("CONFIRMER:isBundleHashConfirmed: %v", err)
			} else {
				if confirmed {
					conf.sendConfirmerUpdate(UPD_CONFIRM, nil)
					return
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()
	return chUpd, nil
}

func (conf *Confirmer) isBundleHashConfirmed(bundleHash Trytes) (bool, error) {
	respHashes, err := conf.IotaAPI.FindTransactions(FindTransactionsQuery{
		Bundles: Hashes{bundleHash},
	})
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPI)
		return false, err
	}

	states, err := conf.IotaAPI.GetLatestInclusion(respHashes)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPI)
		return false, err
	}
	for _, conf := range states {
		if conf {
			return true, nil
		}
	}
	return false, nil
}

func (conf *Confirmer) sendConfirmerUpdate(updType UpdateType, err error) {
	conf.mutex.Lock()
	upd := &ConfirmerUpdate{
		NumAttaches:           conf.numAttach,
		NumPromotions:         conf.numPromote,
		TotalDurationATTMsec:  conf.totalDurationATTMsec,
		TotalDurationGTTAMsec: conf.totalDurationATTMsec,
		UpdateTime:            time.Now(),
		UpdateType:            updType,
		Err:                   err,
	}
	conf.mutex.Unlock()
	conf.chanUpdate <- upd
}

func (conf *Confirmer) checkConsistency(tailHash Hash) (bool, error) {
	consistent, info, err := conf.IotaAPI.CheckConsistency(tailHash)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPI)
		return false, err
	}
	if !consistent && strings.Contains(info, "not solid") {
		consistent = true
	}
	if !consistent {
		conf.debugf("CONFIRMER: inconsistent tail. Reason: %v", info)
	}
	return consistent, nil
}

func (conf *Confirmer) promoteIfNeeded() (bool, error) {
	conf.mutex.Lock()
	defer conf.mutex.Unlock()

	toPromote, err := conf.checkIfToPromote()
	if err != nil {
		return false, err
	}
	if !toPromote {
		return false, nil
	}
	err = conf.promote()
	return err == nil, err
}

func (conf *Confirmer) checkIfToPromote() (bool, error) {

	if conf.isNotPromotable || time.Now().Before(conf.nextPromoTime) {
		// if not promotable, routine will be idle until reattached
		return false, nil
	}
	// check if next tail to promote is consistent. If not, promote will be idle
	consistent, err := conf.checkConsistency(conf.nextTailHashToPromote)
	if err != nil {
		return false, err
	}
	conf.isNotPromotable = !consistent
	return consistent, nil
}

func (conf *Confirmer) goPromote() func() {
	chCancel := make(chan struct{})

	var wg sync.WaitGroup
	go func() {
		conf.debugf("CONFIRMER: started promoter routine. Tail to promote = %v", conf.nextTailHashToPromote)
		defer conf.debugf("CONFIRMER: ended promoter routine for bundle hash %v", conf.nextTailHashToPromote)

		wg.Add(1)
		defer wg.Done()

		var err error
		var promoted bool
		for {
			promoted, err = conf.promoteIfNeeded()
			if err != nil {
				conf.sendConfirmerUpdate(UPD_NO_ACTION, err)
			} else {
				if promoted {
					conf.sendConfirmerUpdate(UPD_PROMOTE, nil)
				}
			}
			if err != nil {
				conf.errorf("CONFIRMER: promotion routine: %v. Sleep 5 sec: ", err)
				time.Sleep(5 * time.Second)
			}
			select {
			case <-chCancel:
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
	}()
	return func() {
		close(chCancel)
		wg.Wait()
	}
}

func (conf *Confirmer) goReattach() func() {
	chCancel := make(chan struct{})

	var wg sync.WaitGroup
	go func() {
		conf.debugf("CONFIRMER: started reattacher routine")
		defer conf.debugf("CONFIRMER: ended reattacher routine")

		wg.Add(1)
		defer wg.Done()

		var err error
		var sendUpdate bool
		for {
			conf.mutex.Lock()
			if conf.isNotPromotable || time.Now().After(conf.nextForceReattachTime) {
				err = conf.reattach()
				sendUpdate = true
			}
			conf.mutex.Unlock()

			if sendUpdate {
				if err != nil {
					conf.sendConfirmerUpdate(UPD_NO_ACTION, err)
				} else {
					conf.sendConfirmerUpdate(UPD_REATTACH, nil)
				}
			}
			if err != nil {
				conf.errorf("promotion routine: %v", err)
			}
			if err != nil {
				time.Sleep(5 * time.Second)
			}
			sendUpdate = false
			err = nil
			select {
			case <-chCancel:
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}()
	return func() {
		close(chCancel)
		wg.Wait()
	}
}
