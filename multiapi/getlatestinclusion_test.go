package multiapi

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

func TestMultiAPI_GetLatestInclusion(t *testing.T) {
	go func() {
		for {
			fmt.Printf("--------- Num goroutines = %d\n", runtime.NumGoroutine())
			time.Sleep(1 * time.Second)
		}
	}()
	for i := 0; i < 3; i++ {
		_getLatestInclusion(t)
		time.Sleep(5 * time.Second)
	}
	time.Sleep(5 * time.Minute)
}

func _getLatestInclusion(t *testing.T) {
	mapi, err := New(endpoints, 10)
	if err != nil || mapi == nil {
		t.Errorf("Must return correct mapi and err==nil")
	}

	for i := range mapi {
		res, err := mapi[i].api.GetLatestInclusion(txs)
		fmt.Printf("Original: Result = %v err = %v from %v\n", res, err, mapi[i].endpoint)
	}
	fmt.Println("----- Testing MultiAPI")
	resOrig, err2 := mapi[0].api.GetLatestInclusion(txs)
	resMapi, err1 := mapi.GetLatestInclusion(txs)
	fmt.Printf("res1 = %v err1 =%v res2 = %v err2 = %v\n", resMapi, err1, resOrig, err2)
	if err1 == nil && err2 == nil && !equalBoolSlice(resMapi, resOrig) {
		t.Errorf("Must be equal resuls. Res1 = %v Res2 = %v", resMapi, resOrig)
	}
	var res MultiCallRet
	resMapi, err1 = mapi.GetLatestInclusion(txs, &res)
	resOrig, err2 = mapi[0].api.GetLatestInclusion(txs)
	fmt.Printf("res1 = %v err1 =%v res2 = %v err2 = %v ep = %v\n", resMapi, err1, resOrig, err2, res.Endpoint)
	if err1 == nil && err2 == nil && !equalBoolSlice(resMapi, resOrig) {
		t.Errorf("Must be equal resuls. Res1 = %v Res2 = %v", resMapi, resOrig)
	}

	resMapi, _ = mapi[:1].GetLatestInclusion(txs, &res)
	if res.Endpoint != mapi[0].endpoint {
		t.Errorf("Wrong value of endpoint returned. ep1 = %v ep2 = %v", res.Endpoint, mapi[0].endpoint)
	}

	mapi, _ = New(endpoints, 10)
	resMapi, _ = mapi[:1].GetLatestInclusion(txs, &res)
	fmt.Printf("First result = %v from = %v\n", resMapi, res.Endpoint)
	if !equalBoolSlice(resOrig, resMapi) {
		t.Errorf("%v != %v : results must be equal", resOrig, resMapi)
	}

	for i := 1; i < len(endpoints); i++ {
		resMapiTmp, _ := mapi[:i].GetLatestInclusion(txs, &res)
		fmt.Printf("res1 = %v res2 = %v ep = %v\n", resMapi, resMapiTmp, res.Endpoint)
		if !equalBoolSlice(resMapiTmp, resMapi) {
			t.Errorf("%v != %v : must be equal to first result", resMapiTmp, resMapi)
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
