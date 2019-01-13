package senderpart

import (
	"encoding/json"
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tbsender/sender_update"
	"net/http"
	"sort"
	"sync"
)

type senderState struct {
	id              string
	Name            string `json:"seqName"`
	Index           uint64 `json:"index"`
	Balance         uint64 `json:"balance"`
	fromAddr        string
	Bundle          string `json:"bundle"`
	StartedTs       uint64 `json:"startedTs"`
	updateTs        uint64
	State           string `json:"state"`
	NumPromo        uint64 `json:"numPromo"`
	NumAttach       uint64 `json:"numAttach"`
	PromoteEverySec uint64 `json:"promoteEverySec"`
	PromoteChain    bool   `json:"promoteChain"`
	LastHeartbeat   uint64 `json:"lastHeartbeat"`
}

var (
	senders      = make(map[string]*senderState)
	sendersMutex = &sync.RWMutex{}
)

func updateLastState(upd *sender_update.SenderUpdate) {
	sendersMutex.Lock()
	defer sendersMutex.Unlock()

	state, ok := senders[upd.SeqUID]
	if !ok {
		state = &senderState{
			id:   upd.SeqUID,
			Name: upd.SeqName,
		}
		senders[upd.SeqUID] = state
	}

	state.Index = upd.Index
	state.Balance = upd.Balance
	state.fromAddr = upd.Addr
	state.Bundle = upd.Bundle
	state.StartedTs = upd.StartTs
	state.updateTs = upd.UpdateTs
	state.State = string(upd.UpdType)
	state.NumPromo = upd.NumPromotions
	state.NumAttach = upd.NumAttaches
	state.PromoteEverySec = upd.PromoteEverySec
	state.PromoteChain = upd.PromoteChain
	state.LastHeartbeat = utils.UnixMsNow()
}

func HandlerSenderStates(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write(getSenderStatesJSON())
}

func getSenderStatesJSON() []byte {
	sendersMutex.RLock()
	defer sendersMutex.RUnlock()

	sl := make(SenderStateSlice, 0, len(senders))
	for _, st := range senders {
		sl = append(sl, st)
	}
	sort.Sort(sl)
	ret, err := json.MarshalIndent(sl, "", "  ")
	if err != nil {
		ret = []byte(fmt.Sprintf("getSenderStatesJSON: %v", err))
	}
	return ret
}

type SenderStateSlice []*senderState

func (a SenderStateSlice) Len() int {
	return len(a)
}

func (a SenderStateSlice) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a SenderStateSlice) Less(i, j int) bool {
	return a[i].Name < a[j].Name
}
