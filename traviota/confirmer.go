package main

import (
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/confirmer"
)

func (seq *Sequence) NewConfirmerChan(bundle giota.Bundle) chan *confirmer.ConfirmerUpdate {
	ret := confirmer.Confirmer{
		IOTANode:              seq.Params.IOTANode[0],
		IOTANodeGTTA:          seq.Params.IOTANodeGTTA[0],
		IOTANodeATT:           seq.Params.IOTANodeATT[0],
		TimeoutAPI:            seq.Params.TimeoutAPI,
		TimeoutGTTA:           seq.Params.TimeoutGTTA,
		TimeoutATT:            seq.Params.TimeoutATT,
		TxTagPromote:          seq.TxTagPromote,
		ForceReattachAfterMin: seq.Params.ForceReattachAfterMin,
		PromoteNoChain:        seq.Params.PromoteNoChain,
		PromoteEverySec:       seq.Params.PromoteEverySec,
	}
	return ret.Run(bundle)
}
