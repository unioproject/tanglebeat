package senderpart

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/nanomsg"
	"github.com/lunfardo314/tanglebeat/tanglebeat/hashcache"
	"github.com/lunfardo314/tanglebeat/tanglebeat/inputpart"
	"github.com/lunfardo314/tanglebeat/tanglebeat/inreaders"
	"github.com/lunfardo314/tanglebeat/tanglebeat/pubupdate"
	"github.com/lunfardo314/tanglebeat/tbsender/sender_update"
)

type updateSource struct {
	inreaders.InputReaderBase
	uri string
}

func createUpdateSource(uri string) {
	ret := &updateSource{
		InputReaderBase: *inreaders.NewInputReaderBase(),
		uri:             uri,
	}
	senderUpdateSources.AddInputReader(uri, ret)
}

var (
	senderUpdateSources *inreaders.InputReaderSet
	senderOutPublisher  *nanomsg.Publisher
	publishedUpdates    *hashcache.HashCacheBase
)

func MustInitSenderDataCollector(outEnabled bool, outPort int, inputs []string) {
	publishedUpdates = hashcache.NewHashCacheBase(
		"publishedUpdates", 0, 10*60, 60*60)
	senderUpdateSources = inreaders.NewInputReaderSet("sender update routine set")

	if outEnabled {
		var err error
		senderOutPublisher, err = nanomsg.NewPublisher(outEnabled, outPort, 0, localLog)
		if err != nil {
			errorf("Failed to create sender output publishing channel: %v", err)
			panic(err)
		}
		infof("Publisher for sender output initialized successfully on port %v", outPort)
	} else {
		infof("Publisher for sender output is disabled")
	}
	for _, uri := range inputs {
		createUpdateSource(uri)
	}
}

func (r *updateSource) GetUri() string {
	r.Lock()
	defer r.Unlock()
	return r.uri
}

func (r *updateSource) Run(name string) inreaders.ReasonNotRunning {
	uri := r.GetUri()
	infof("Starting sender update source '%v' at '%v'", name, uri)
	defer errorf("Leaving sender update source '%v' at '%v'", name, uri)

	chIn, err := sender_update.NewUpdateChan(uri)
	if err != nil {
		errorf("failed to initialize sender update source for %v: %v", uri, err)
		return inreaders.REASON_NORUN_ERROR
	}
	r.SetReading(true)
	infof("Successfully started sender update source at %v", uri)
	for upd := range chIn {
		r.SetLastHeartbeatNow()
		err = r.processUpdate(upd)
		if err != nil {
			r.SetLastErr(fmt.Sprintf("Error while processing update: %v", err))
			return inreaders.REASON_NORUN_ONHOLD_10MIN
		}
	}
	return inreaders.REASON_NORUN_ONHOLD_10MIN
}

func (r *updateSource) processUpdate(upd *sender_update.SenderUpdate) error {
	tracef("Processing update from '%v', source: %v, seq: %v(%v), Index: %v",
		upd.UpdType, r.GetUri(), upd.SeqUID, upd.SeqName, upd.Index)

	hash := upd.SeqUID + fmt.Sprintf("%v", upd.UpdateTs)
	if publishedUpdates.SeenHashBy(hash, 0, nil, nil) {
		return nil
	}

	senderUpdateToStats(upd)
	updateSenderMetrics(upd)
	updateLastState(upd)

	// sending promotes for echo tracking
	if upd.UpdType == sender_update.SENDER_UPD_PROMOTE && len(upd.PromoTail) != 0 {
		inputpart.TxSentForEcho(upd.PromoTail, upd.UpdateTs)
	}

	if senderOutPublisher != nil {
		if upd.UpdType == sender_update.SENDER_UPD_CONFIRM {
			debugf("Publish update '%v' received from %v, seq: %v(%v), Index: %v",
				upd.UpdType, r.GetUri(), upd.SeqUID, upd.SeqName, upd.Index)
		} else {
			tracef("Publish update '%v' received from %v, seq: %v(%v), Index: %v",
				upd.UpdType, r.GetUri(), upd.SeqUID, upd.SeqName, upd.Index)
		}
		if err := pubupdate.PublishSenderUpdate(senderOutPublisher, upd); err != nil {
			errorf("Process update: %v", err)
			return err
		}
	}
	return nil
}
