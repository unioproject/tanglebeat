package bundle_source

import (
	"github.com/lunfardo314/giota"
	"time"
)

// the idea is abstract source (channel) of bundles to be confirmed.
// Channel produces records of type FirstBundleData to confirm.
// It can be newly created (e.g. new transfer), or it can be already read from the tangle.
// 'StartTime' and 'IsNew' is set accordingly
// 'address' is an input  address of the transfer (if it is a transfer)
//
// Traveling IOTA -style of Bundle source produces sequences of transfer bundles, each
// spends whole palances from the previous address to the next
// It produces new budnles only upon and immediately after confirmation of the previous transfer.
//
// Bundle source in principle it can be any Bundle generator,
// for example sequence of MAM bundles to confirm

// structure produced by Bundle generator
type FirstBundleData struct {
	Addr      giota.Address
	Index     int
	Bundle    giota.Bundle // Bundle to confirm
	IsNew     bool         // new Bundle created or existing one found
	StartTime time.Time    // 	for new Bundle, when Bundle attach,
	// 	for old Bundle timestamp of oldest of all tails
	TotalDurationPoWMs    int64 // > 0 if new Bundle, ==0 if existing Bundle
	TotalDurationTipselMs int64 // > 0 if new Bundle, ==0 if existing Bundle
	NumAttach             int64 // number of tails with the same Bundle hash at the start
}

type BundleSourceChan chan *FirstBundleData
