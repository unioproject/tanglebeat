package zmqpart

import "github.com/lunfardo314/tanglebeat/tanglebeat/cfg"

func toOutput(msgData []byte, msgSplit []string, repeatedTimes int) {
	// check if message was seen exactly number of times as configured (usually 2)
	if repeatedTimes == cfg.Config.RepeatToAcceptTX {
		// publish message to output Nanomsg channel exactly as reaceived from ZeroMQ. For others to consume
		if err := compoundOutPublisher.PublishData(msgData); err != nil {
			errorf("Error while publishing data: %v", err)
		}
		// update metrics based on compound (resulting) message stream (TPS, CTPS etc)
		updateCompoundMetrics(msgSplit[0])
		// analyze if this is value transaction. Process to collect necessary metrics
		processValueTxMsg(msgSplit)
	}
}
