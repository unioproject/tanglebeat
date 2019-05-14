package inputpart

import (
	"fmt"
	"github.com/unioproject/tanglebeat/tanglebeat/cfg"
)

func toOutput(msgData []byte, msgSplit []string) {
	// publish message to output Nanomsg channel exactly as received from ZeroMQ. For others to consume
	if err := compoundOutPublisher.PublishData(msgData); err != nil {
		errorf("Error while publishing data: %v", err)
	}
	// update metrics based on compound (resulting) message stream (TPS, CTPS etc)
	updateCompoundMetrics(msgSplit[0])
	// analyze if this is value transaction. Process to collect necessary metrics
	processValueTxMsg(msgSplit)
}

// forming new message type
// 'seen <tx_hash> <quorum filter level passed>'

func publishQuorumUpdate(txHash string, timesSeen int) {
	if !cfg.Config.QuorumUpdatesEnabled {
		return
	}
	if timesSeen < cfg.Config.QuorumUpdatesFrom || timesSeen > cfg.Config.QuorumUpdatesTo {
		return
	}
	msgData := fmt.Sprintf("seen %s %d", txHash, timesSeen)

	if err := compoundOutPublisher.PublishData([]byte(msgData)); err != nil {
		errorf("Error while publishing data: %v", err)
	}
}
