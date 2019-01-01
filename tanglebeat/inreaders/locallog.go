package inreaders

import (
	"fmt"
	"github.com/op/go-logging"
)

var (
	localLog   *logging.Logger
	localDebug bool
)

func SetLog(log *logging.Logger, debug bool) {
	localLog = log
	localDebug = debug
}

func errorf(format string, args ...interface{}) {
	if localLog != nil {
		localLog.Errorf(format, args...)
	} else {
		fmt.Printf("ERRO "+format+"\n", args...)
	}
}

func debugf(format string, args ...interface{}) {
	if !localDebug {
		return
	}
	if localLog != nil {
		localLog.Debugf(format, args...)
	} else {
		fmt.Printf("DEBU "+format+"\n", args...)
	}
}

func infof(format string, args ...interface{}) {
	if localLog != nil {
		localLog.Infof(format, args...)
	} else {
		fmt.Printf("INFO "+format+"\n", args...)
	}
}
