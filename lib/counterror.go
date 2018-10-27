package lib

import "github.com/lunfardo314/giota"

type ErrorCounter interface {
	AccountError(api *giota.API)
}
