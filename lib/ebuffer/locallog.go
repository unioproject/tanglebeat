package ebuffer

import (
	"fmt"
	"github.com/op/go-logging"
)

var (
	localLog   *logging.Logger
	localTrace bool
)

func SetLog(log *logging.Logger, trace bool) {
	localLog = log
	localTrace = trace
}

func errorf(format string, args ...interface{}) {
	if localLog != nil {
		localLog.Errorf(format, args...)
	} else {
		fmt.Printf("ERRO "+format+"\n", args...)
	}
}

func debugf(format string, args ...interface{}) {
	if localLog != nil {
		localLog.Debugf(format, args...)
	} else {
		fmt.Printf("DEBU "+format+"\n", args...)
	}
}

func tracef(format string, args ...interface{}) {
	if !localTrace {
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
