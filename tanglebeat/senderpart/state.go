package senderpart

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tbsender/sender_update"
	"sync"
)

type senderState struct {
	id            string
	name          string
	index         uint64
	value         uint64
	fromAddr      string
	toAddr        string
	bundle        string
	startedTs     uint64
	updateTs      uint64
	state         string
	numPromo      uint64
	numAttach     uint64
	lastHeartbeat uint64
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
			name: upd.SeqName,
		}
		senders[upd.SeqUID] = state
	}

	state.index = upd.Index
	state.value = upd.Balance
	state.fromAddr = upd.Addr
	state.toAddr = ""
	state.bundle = upd.Bundle
	state.startedTs = upd.StartTs
	state.updateTs = upd.UpdateTs
	state.state = string(upd.UpdType)
	state.numPromo = upd.NumPromotions
	state.numAttach = upd.NumAttaches
	state.lastHeartbeat = utils.UnixMsNow()
}
