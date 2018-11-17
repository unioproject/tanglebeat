package lib

import "github.com/iotaledger/iota.go/api"

type ErrorCounter interface {
	IncErrorCount(api *api.API)
}
