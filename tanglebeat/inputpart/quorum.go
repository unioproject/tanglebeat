package inputpart

import "github.com/unioproject/tanglebeat/tanglebeat/cfg"

func GetTxQuorum() int {
	return cfg.Config.QuorumTxToPass
}

func GetSnQuorum() int {
	return 3
}

func GetLmiQuorum() int {
	return 3
}
