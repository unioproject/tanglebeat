package pubupdate

import (
	"github.com/unioproject/tanglebeat/lib/nanomsg"
	"github.com/unioproject/tanglebeat/tbsender/sender_update"
)

func PublishSenderUpdate(publisher *nanomsg.Publisher, upd *sender_update.SenderUpdate) error {
	err := publisher.PublishAsJSON(upd)
	if err != nil {
		return err
	}
	publisher.Debugf("Published sender update '%v' from '%v'", upd.UpdType, upd.SeqName)
	return nil
}
