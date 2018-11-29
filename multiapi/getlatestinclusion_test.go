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
	fmt.Printf("Err = %v\n", err)

	mapi, err = New(nil, 10)
	if err == nil {
		t.Fail()
	}
	fmt.Printf("Err = %v\n", err)

	mapi, err = New(endpoints[:1], 10)
	if err != nil || mapi == nil {
		t.Fail()
	}
	fmt.Printf("Err = %v\n", err)

	resMapi, _ := mapi.GetLatestInclusion(Hashes{tx})
	resOrig, _ := mapi[0].api.GetLatestInclusion(Hashes{tx})
	if !equalBoolSlice(resMapi, resOrig) {
		t.Fail()
	}
	var res MultiAPIGetLatestInclusionResult
	resMapi, _ = mapi.GetLatestInclusion(Hashes{tx}, &res)
	if res.Endpoint != mapi[0].endpoint {
		t.Errorf("Wrong value returned")
	}

	mapi, err = New(endpoints, 10)
	resMapi, _ = mapi[:1].GetLatestInclusion(Hashes{tx})
	for i := 0; i < 3; i++ {
		resMapiTmp, err := mapi[:i].GetLatestInclusion(Hashes{tx}, &res)
		fmt.Printf("err = %v endpoint = %v\n", err, res.Endpoint)
		if !equalBoolSlice(resMapiTmp, resMapi) {
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
