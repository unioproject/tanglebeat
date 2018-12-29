package main

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/nanomsg"
	"github.com/lunfardo314/tanglebeat/tanglebeat/inreaders"
	"github.com/lunfardo314/tanglebeat/tbsender/sender_update"
)

type updateSource struct {
	inreaders.InputReaderBase
	uri string
}

func createUpdateSource(uri string) {
	ret := &updateSource{
		InputReaderBase: *inreaders.NewInoutReaderBase("TBSENDER--" + uri),
		uri:             uri,
	}
	senderUpdateSources.AddInputReader(ret)
}

var (
	senderUpdateSources *inreaders.InputReaderSet
	senderOutPublisher  *nanomsg.Publisher
	publishedUpdates    *hashCacheBase
)

func mustInitSenderDataCollector() {
	publishedUpdates = newHashCacheBase(0, 10*60*1000, 60*60*1000)
	senderUpdateSources = inreaders.NewInputReaderSet()

	if Config.SenderMsgStream.OutputEnabled {
		var err error
		senderOutPublisher, err = nanomsg.NewPublisher(Config.SenderMsgStream.OutputPort, 0, nil)
		if err != nil {
			errorf("Failed to create sender output publishing channel: %v", err)
			panic(err)
		}
		infof("Publisher for sender output initialized successfully on port %v",
			Config.SenderMsgStream.OutputPort)
	} else {
		infof("Publisher for sender output is disabled")
	}
	for _, uri := range Config.SenderMsgStream.Inputs {
		createUpdateSource(uri)
	}
}

func (r *updateSource) GetUri() string {
	r.Lock()
	defer r.Unlock()
	return r.uri
}

func (r *updateSource) Run() {
	uri := r.GetUri()
	chIn, err := sender_update.NewUpdateChan(uri)
	if err != nil {
		errorf("failed to initialize sender update source for %v: %v", uri, err)
		return
	}
	r.SetReading(true)
	infof("Start reading external sender update source at %v", uri)
	for upd := range chIn {
		r.SetLastHeartbeatNow()
		err = r.processUpdate(upd)
		if err != nil {
			r.SetLastErr(fmt.Sprintf("Error while processing update: %v", err))
		}
	}
}

func (r *updateSource) processUpdate(upd *sender_update.SenderUpdate) error {
	infof("Processing update from '%v', source: %v, seq: %v(%v), index: %v",
		upd.UpdType, r.GetUri(), upd.SeqUID, upd.SeqName, upd.Index)

	hash := upd.SeqUID + fmt.Sprintf("%v", upd.UpdateTs)
	if publishedUpdates.seenHash(hash, nil, nil) {
		return nil
	}

	updateSenderMetrics(upd)

	if Config.SenderMsgStream.OutputEnabled {
		infof("Publish update '%v' received from %v, seq: %v(%v), index: %v",
			upd.UpdType, r.GetUri(), upd.SeqUID, upd.SeqName, upd.Index)
		if err := senderOutPublisher.PublishAsJSON(upd); err != nil {
			log.Errorf("Process update: %v", err)
			return err
		}
	}
	return nil
}
