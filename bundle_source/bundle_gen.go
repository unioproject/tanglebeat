package bundle_source

import (
	"github.com/iotaledger/iota.go/trinary"
)

// the idea is abstract source (channel) of bundles to be confirmed.
// Channel produces records of type FirstBundleData to confirm.
// It can be newly created (e.g. new transfer), or it can be already read from the tangle.
// 'StartTime' and 'IsNew' is set accordingly
// 'address' is an input  address of the transfer (if it is a transfer)
//
// Traveling IOTA -style of BundleTrytes source produces sequences of transfer bundles, each
// spends whole palances from the previous address to the next
// It produces new budnles only upon and immediately after confirmation of the previous transfer.
//
// BundleTrytes source in principle it can be any BundleTrytes generator,
// for example sequence of MAM bundles to confirm

// structure produced by BundleTrytes generator
type FirstBundleData struct {
	Addr         trinary.Hash
	Index        uint64
	BundleTrytes []trinary.Trytes // raw bundle trytes to start with confirmation.
	BundleHash   trinary.Hash     // bundle hash, never changes
	IsNew        bool             // new BundleTrytes created or existing one found
	StartTime    uint64           // unix milisec	for new BundleTrytes, when BundleTrytes attach,
	// 	for old BundleTrytes timestamp of oldest of all tails
	TotalDurationPoWMs    uint64 // > 0 if new BundleTrytes, ==0 if existing BundleTrytes
	TotalDurationTipselMs uint64 // > 0 if new BundleTrytes, ==0 if existing BundleTrytes
	NumAttach             uint64 // number of tails with the same BundleTrytes hash at the start
}

type BundleSourceChan chan *FirstBundleData
