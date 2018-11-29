package multiapi

import (
	"testing"
)

func Test_NewMultiapi(t *testing.T) {
	var mapi MultiAPI
	var err error
	mapi, err = New(endpoints, 0)
	if err == nil {
		t.Errorf("Must return an error")
	}

	mapi, err = New(nil, 10)
	if err == nil {
		t.Errorf("Must return an error")
	}

	mapi, err = New(endpoints[:1], 10)
	if err != nil || mapi == nil {
		t.Errorf("Must return correct mapi and err==nil")
	}
}
