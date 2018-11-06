package lib

import "github.com/lunfardo314/giota"

type ErrorCounter interface {
	IncErrorCount(api *giota.API)
}
