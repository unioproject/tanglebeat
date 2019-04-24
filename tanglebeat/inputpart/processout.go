package inputpart

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
