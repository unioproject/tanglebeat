package sender_update

// defines update's structure.
// It is published as JSON

import (
	"github.com/lunfardo314/giota"
)

type SenderUpdateType string

const (
	SENDER_UPD_UNDEF          SenderUpdateType = "undef"
	SENDER_UPD_NO_ACTION      SenderUpdateType = "no action"
	SENDER_UPD_START_SEND     SenderUpdateType = "send"
	SENDER_UPD_START_CONTINUE SenderUpdateType = "continue"
	SENDER_UPD_REATTACH       SenderUpdateType = "reattach"
	SENDER_UPD_PROMOTE        SenderUpdateType = "promote"
	SENDER_UPD_CONFIRM        SenderUpdateType = "confirm"
)

type SenderUpdate struct {
	SeqUID  string           `json:"seqid"`   // unique id of the sequences. Parte of seed's hash
	SeqName string           `json:"seqname"` // name of the sequence as specified in the config
	UpdType SenderUpdateType `json:"updtype"` // update type
	Index   int              `json:"addridx"` // address index
	Addr    giota.Address    `json:"addr"`    // address
	Bundle  giota.Trytes     `json:"bundle"`  // bundle hash
	StartTs int64            `json:"start"`   // unix time miliseconds when first bundle was creates.
	// If bundle was already in the tangle (after restart)
	// it is equal to timestamp of the tail
	UpdateTs int64 `json:"ts"` // unix time miliseconds when update was created. Based on the
	// same clock as StartedTs
	NumAttaches           int64  `json:"numattach"`  // number of out bundles in tha tangle
	NumPromotions         int64  `json:"numpromote"` // number of promotions in the current session (starts with 0 after restart)
	TotalPoWMsec          int64  `json:"powms"`      // total milliseconds spent on PoW (attachToTangle calls)
	TotalTipselMsec       int64  `json:"tipselms"`   // total milliseconds spent on tipsel (getTransactionsToApproves calls)
	NodeATT               string `json:"nodepow"`    // node used for PoW
	NodeGTTA              string `json:"nodetipsel"` // node used for tipsel
	BundleSize            int64  `json:"bsize"`      // number of tx in the spending bundle
	PromoBundleSize       int64  `json:"pbsize"`     // number of tx in the promo bundle
	PromoteEveryNumSec    int64  `json:"promosec"`   // sleep time after each promoton
	ForceReattachAfterMin int64  `json:"reattmin"`   // force reattach after minutes
	PromoteChain          bool   `json:"chain"`      // promotion strategy. "chain" vs 'blowball'
}
