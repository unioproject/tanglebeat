package lib

import "github.com/iotaledger/iota.go/api"

type ErrorCounter interface {
	CheckError(api *api.API, err error) bool
	RegisterAPI(api *api.API, endpoint string)
}
