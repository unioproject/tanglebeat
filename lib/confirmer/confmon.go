package confirmer

import (
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/op/go-logging"
	"github.com/unioproject/tanglebeat/lib/multiapi"
	"github.com/unioproject/tanglebeat/lib/utils"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/tcp"
	"strings"
	"sync"
	"time"
)

type bundleState struct {
	callbacks []func(time.Time)
}

type ConfirmationMonitor struct {
	sync.Mutex
	bundles       map[Hash]*bundleState
	mapi          multiapi.MultiAPI
	log           *logging.Logger
	aec           utils.ErrorCounter
	loopSleepTime time.Duration
}

const (
	pollingSleepWithoutNanozmq = 30 * time.Second
	pollingSleepWithNanozmq    = 3 * time.Minute
)

func NewConfirmationMonitor(mapi multiapi.MultiAPI, nanozmq string, log *logging.Logger, aec utils.ErrorCounter) *ConfirmationMonitor {
	ret := &ConfirmationMonitor{
		bundles:       make(map[Hash]*bundleState),
		mapi:          mapi,
		log:           log,
		aec:           aec,
		loopSleepTime: pollingSleepWithoutNanozmq,
	}
	if nanozmq == "" {
		log.Infof("Confirmation monitor: Will be polling only")
		return ret
	}
	log.Infof("Confirmation monitor: will be listening to '%s'", nanozmq)

	go ret.nanomsgRoutine(nanozmq)

	return ret
}

func (cmon *ConfirmationMonitor) errorf(format string, args ...interface{}) {
	if cmon.log != nil {
		cmon.log.Errorf(format, args...)
	}
}

func (cmon *ConfirmationMonitor) debugf(format string, args ...interface{}) {
	if cmon.log != nil {
		cmon.log.Debugf(format, args...)
	}
}

func (cmon *ConfirmationMonitor) checkError(endpoint string, err error) bool {
	if cmon.aec != nil {
		return cmon.aec.CheckError(endpoint, err)
	}
	return err != nil
}

func (cmon *ConfirmationMonitor) OnConfirmation(bundleHash Hash, callback func(time.Time)) {
	cmon.Lock()
	defer cmon.Unlock()

	_, ok := cmon.bundles[bundleHash]
	if !ok {
		cmon.bundles[bundleHash] = &bundleState{
			callbacks: make([]func(time.Time), 0, 2),
		}
		go cmon.pollConfirmed(bundleHash)
	}
	cmon.bundles[bundleHash].callbacks = append(cmon.bundles[bundleHash].callbacks, callback)
}

// can't be called from within OnConfirmation callback
func (cmon *ConfirmationMonitor) CancelConfirmationPolling(bundleHash Hash) {
	cmon.Lock()
	defer cmon.Unlock()
	delete(cmon.bundles, bundleHash)
}

func (cmon *ConfirmationMonitor) pollConfirmed(bundleHash Hash) {
	count := 0
	var exit bool

	startWaiting := time.Now()
	for !exit {
		cmon.Lock()
		st := cmon.loopSleepTime
		cmon.Unlock()
		time.Sleep(st)

		count++
		if count%5 == 0 {
			cmon.debugf("Confirmation polling for %v. Time since waiting: %v", bundleHash, time.Since(startWaiting))
		}
		exit = cmon.checkBundle(bundleHash)
	}
}

func (cmon *ConfirmationMonitor) checkBundle(bundleHash Hash) bool {
	var apiret multiapi.MultiCallRet
	var err error
	var confirmed bool
	var bs *bundleState
	var ok bool

	cmon.Lock()
	defer cmon.Unlock()

	if bs, ok = cmon.bundles[bundleHash]; !ok {
		return true // not in map, was cancelled or never started
	}

	confirmed, err = utils.IsBundleHashConfirmedMulti(bundleHash, cmon.mapi, &apiret)

	if cmon.checkError(apiret.Endpoint, err) {
		cmon.errorf("Confirmation polling for %v: '%v' from %v ", bundleHash, err, apiret.Endpoint)
		time.Sleep(sleepAfterError)
		return false
	}
	if confirmed {
		nowis := time.Now()

		// call all callbacks asynchronously
		for _, cb := range bs.callbacks {
			go cb(nowis)
		}
		delete(cmon.bundles, bundleHash) // delete from map

		// stop the stopwatch for the bundle
		StopStopwatch(bundleHash)
		return true // confirmed: stop polling
	}
	return false
}

func (cmon *ConfirmationMonitor) nanomsgRoutine(nanozmq string) {
	for {
		sock := cmon.openNanozmq(nanozmq)

		cmon.Lock()
		cmon.loopSleepTime = pollingSleepWithNanozmq
		cmon.Unlock()

		cmon.log.Errorf("Confirmation monitor: started listening to '%v'", nanozmq)

		cmon.nanozmqLoop(sock)
		cmon.Lock()
		cmon.loopSleepTime = pollingSleepWithoutNanozmq
		cmon.Unlock()
	}
}

func (cmon *ConfirmationMonitor) openNanozmq(nanozmq string) mangos.Socket {
	var ret mangos.Socket
	var err error
	first := true
	for {
		if !first {
			time.Sleep(30 * time.Second)
			first = false
		}
		if ret, err = sub.NewSocket(); err != nil {
			cmon.log.Errorf("Confirmation monitor: can't create new sub socket '%v'. Will be polling only", err)
			continue
		}
		ret.AddTransport(tcp.NewTransport())
		if err = ret.Dial(nanozmq); err != nil {
			cmon.log.Errorf("Confirmation monitor: can't dial sub socket at %v: %v.  Will be polling only", nanozmq, err)
			continue
		}
		err = ret.SetOption(mangos.OptionSubscribe, []byte("sn"))
		if err != nil {
			cmon.log.Errorf("Confirmation monitor: sub socket error %v: %v.  Will be polling only", nanozmq, err)
			continue
		}
		return ret
	}
}

func (cmon *ConfirmationMonitor) nanozmqLoop(sock mangos.Socket) {
	var msg []byte
	var err error
	var msgSplit []string
	var bundle Hash

	for {
		msg, err = sock.Recv()
		if err != nil {
			cmon.log.Errorf("Confirmation monitor: '%v'. Will be polling only")
			return
		}
		msgSplit = strings.Split(string(msg), " ")
		if len(msgSplit) < 7 {
			cmon.log.Errorf("Confirmation monitor: wrong msg format")
			continue
		}
		bundle = Hash(msgSplit[6])

		cmon.Lock()
		for b, bs := range cmon.bundles {
			if b == bundle {
				nowis := time.Now()
				for _, cb := range bs.callbacks {
					go cb(nowis)
				}
				delete(cmon.bundles, bundle)
				StopStopwatch(bundle)
				cmon.log.Infof("Confirmation monitor: received confirmation msg from Nanozmq for bundle %s", bundle)
				break
			}
		}
		cmon.Unlock()
	}
}
