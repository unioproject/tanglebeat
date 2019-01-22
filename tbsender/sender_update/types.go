package sender_update

// defines update's structure.
// It is published as JSON

import (
	. "github.com/iotaledger/iota.go/trinary"
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
	Version   string           `json:"ver"`       // version of the originator
	SeqUID    string           `json:"seqid"`     // unique id of the sequences. Parte of seed's hash
	SeqName   string           `json:"seqname"`   // name of the sequence as specified in the config
	UpdType   SenderUpdateType `json:"updtype"`   // update type
	Index     uint64           `json:"addridx"`   // address index
	Balance   uint64           `json:"balance"`   // balance is being sent
	Addr      Hash             `json:"addr"`      // address
	Bundle    Hash             `json:"bundle"`    // bundle hash
	PromoTail Hash             `json:"promoTail"` // tail hash
	StartTs   uint64           `json:"start"`     // unix time miliseconds when first bundle was creates.
	// If bundle was already in the tangle (after restart)
	// it is equal to timestamp of the tail
	UpdateTs uint64 `json:"ts"` // unix time miliseconds when update was created. Based on the
	// same clock as StartedTs
	NumAttaches           uint64 `json:"numattach"`  // number of out bundles in tha tangle
	NumPromotions         uint64 `json:"numpromote"` // number of promotions in the current session (starts with 0 after restart)
	TotalPoWMsec          uint64 `json:"powms"`      // total milliseconds spent on PoW (attachToTangle calls)
	TotalTipselMsec       uint64 `json:"tipselms"`   // total milliseconds spent on tipsel (getTransactionsToApproves calls)
	NodePOW               string `json:"nodepow"`    // node used for attachToTangle calls
	NodeTipsel            string `json:"nodetipsel"` // node used for getTransactionToApprove cals
	BundleSize            uint64 `json:"bsize"`      // number of tx in the spending bundle
	PromoBundleSize       uint64 `json:"pbsize"`     // number of tx in the promo bundle
	PromoteEverySec       uint64 `json:"promosec"`   // sleep time after each promoton
	ForceReattachAfterMin uint64 `json:"reattmin"`   // force reattach after minutes
	PromoteChain          bool   `json:"chain"`      // promotion strategy. "chain" vs 'blowball'
}
