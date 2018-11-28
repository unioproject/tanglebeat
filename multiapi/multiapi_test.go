package multiapi

import (
	"fmt"
	. "github.com/iotaledger/iota.go/trinary"
	"testing"
)

var endpoints = []string{
	"http://node.iotalt.com:14600",
	"https://field.deviota.com:443",
	"https://nodes.thetangle.org:443",
}

var tx = Trytes("YOL9RXYRQFGGPNOOMRXVJVGQCOKFWZCGUKHCZBZ9POSIWDPEI9IOSTNOZKKTWOGIOPMH9NAQVJXKZ9999")

func TestMultiAPI_GetLatestInclusion(t *testing.T) {
	var mapi MultiAPI
	var err error
	mapi, err = New(endpoints, 0)
	if err == nil {
		t.Fail()
	}
	fmt.Printf("err = %v\n", err)

	mapi, err = New(nil, 10)
	if err == nil {
		t.Fail()
	}
	fmt.Printf("err = %v\n", err)

	mapi, err = New(endpoints[:1], 10)
	if err != nil || mapi == nil {
		t.Fail()
	}
	fmt.Printf("err = %v\n", err)

	resMapi, errMapi, idx := mapi.GetLatestInclusion(Hashes{tx})
	resOrig, errOrig := mapi[0].GetLatestInclusion(Hashes{tx})
	if !equalBoolSlice(resMapi, resOrig) {
		t.Fail()
	}
	if errMapi != errOrig {
		t.Fail()
	}
	if idx != 0 {
		t.Fail()
	}

	mapi, err = New(endpoints, 10)
	for i := 0; i < 10; i++ {
		resMapi, errMapi, idx = mapi.GetLatestInclusion(Hashes{tx})
		fmt.Printf("Idx = %d endpoint = %v\n", idx, endpoints[idx])
		if !equalBoolSlice(resMapi, resOrig) {
			t.Fail()
		}
		if errMapi != errOrig {
			t.Fail()
		}
	}
}

func equalBoolSlice(ba1, ba2 []bool) bool {
	if len(ba1) != len(ba2) {
		return false
	}
	for i := range ba1 {
		if ba1[i] != ba2[i] {
			return false
		}
	}
	return true
}
