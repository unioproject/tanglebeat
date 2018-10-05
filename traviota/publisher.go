package main

import "sync"

type publisher struct {
	senders map[string]chan senderCmd // input channels form senders
	mutex   sync.Mutex
}

type senderCmd int

func (pub *publisher) registerSender(id string) chan senderCmd {
	pub.mutex.Lock()
	defer pub.mutex.Unlock()

	if _, ok := pub.senders[id]; ok {
		log.Panicf("Attempt to register sender %v twice", id)
	}
	pub.senders[id] = make(chan senderCmd)
	return pub.senders[id]
}
