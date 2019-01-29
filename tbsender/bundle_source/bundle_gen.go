package bundle_source

import (
	"fmt"
	. "github.com/iotaledger/iota.go/trinary"
	"sync"
)

// the idea is abstract source (channel) of bundles to be confirmed.
// Channel produces records of type FirstBundleData to confirm.
// It can be newly created (e.g. new transfer), or it can be already read from the tangle.
// 'StartTime' and 'IsNew' is set accordingly
// 'address' is an input  address of the transfer (if it is a transfer)
//
// Traveling IOTA-style of BundleTrytes source produces sequences of transfer bundles, each
// spends whole balances from the previous address to the next
// It produces new bundles only upon and immediately after confirmation of the previous transfer.
//
// BundleTrytes source in principle it can be any BundleTrytes generator,
// for example sequence of MAM bundles to confirm

// structure produced by BundleTrytes generator
type FirstBundleData struct {
	Addr         Hash
	Index        uint64
	Balance      uint64
	BundleTrytes []Trytes // raw bundle trytes to start with confirmation. Bundle must be on the tangle
	BundleHash   Hash
	IsNew        bool // new BundleTrytes created or existing one found TODO deprecate
	//StartTime             uint64           // unix milisec	set when bundle was read from tangle. Tx timestamps not used
	TotalDurationPoWMs    uint64 // > 0 if new BundleTrytes, ==0 if existing BundleTrytes
	TotalDurationTipselMs uint64 // > 0 if new BundleTrytes, ==0 if existing BundleTrytes
	NumAttach             uint64 // number of tails with the same BundleTrytes hash at the start
}

type ConfirmationResult struct {
	BundleHash Hash // hash of the bundle, for control
	success    bool // if confirmed succesfully
}

type BundleSource struct {
	theSource         chan *FirstBundleData
	results           chan *ConfirmationResult
	bundlesInProgress map[Hash]func(bool)
	mapMutex          *sync.Mutex
}

func NewBundleSource() *BundleSource {
	ret := &BundleSource{
		theSource:         make(chan *FirstBundleData),
		results:           make(chan *ConfirmationResult),
		bundlesInProgress: make(map[Hash]func(bool)),
		mapMutex:          &sync.Mutex{},
	}
	go ret.confirmationProcessingLoop()
	return ret
}

// used by confirmer. Retrieves next bundle from the source
func (src *BundleSource) GetNextBundleToConfirm() *FirstBundleData {
	ret, ok := <-src.theSource
	if ok {
		return ret
	}
	return nil
}

func (src *BundleSource) RegisterBundleToConfirm__(bundleData *FirstBundleData, onConfirm func(bool)) error {
	src.mapMutex.Lock()
	defer src.mapMutex.Unlock()

	if _, ok := src.bundlesInProgress[bundleData.BundleHash]; ok {
		return fmt.Errorf("bundle %v is already being confirmed", bundleData.BundleHash)
	}
	src.bundlesInProgress[bundleData.BundleHash] = onConfirm
	return nil
}

// used by generator. Generated bundle is put to the source.
// it is retrieved by GetNextBundleToConfirm
func (src *BundleSource) PutNextBundleToConfirm(bundleData *FirstBundleData, onConfirm func(bool)) error {
	if err := src.RegisterBundleToConfirm__(bundleData, onConfirm); err != nil {
		return err
	}
	src.theSource <- bundleData
	return nil
}

// used by source consumer to return confirmation result
func (src *BundleSource) PutConfirmationResult(bundleHash Hash, success bool) {
	src.results <- &ConfirmationResult{
		BundleHash: bundleHash,
		success:    success,
	}
}

func (src *BundleSource) PutAndWaitForResult(bundleData *FirstBundleData) (bool, error) {
	var ret bool
	var wg sync.WaitGroup
	wg.Add(1)
	err := src.PutNextBundleToConfirm(bundleData, func(success bool) {
		ret = success
		wg.Done()
	})
	if err != nil {
		wg.Done()
		return false, err
	}
	wg.Wait()
	return ret, nil
}

func (src *BundleSource) confirmationProcessingLoop() {
	for r := range src.results {
		src.processConfirmation(r)
	}
}

// on confirmation calls callback and deletes it from the map
func (src *BundleSource) processConfirmation(r *ConfirmationResult) {
	src.mapMutex.Lock()
	defer src.mapMutex.Unlock()
	src.bundlesInProgress[r.BundleHash](r.success)
	delete(src.bundlesInProgress, r.BundleHash)
}
