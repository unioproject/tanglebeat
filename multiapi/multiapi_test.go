package multiapi

import (
	"fmt"
	. "github.com/iotaledger/iota.go/api"
	. "github.com/iotaledger/iota.go/trinary"
	. "net/http"
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
	_getLatestInclusion(t)
	//for i := 0; i < 3; i++ {
	//	_getLatestInclusion(t)
	//	time.Sleep(5 * time.Second)
	//}
	//time.Sleep(5 * time.Minute)
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

func Test_polyCall(t *testing.T) {
	api, err := ComposeAPI(
		HTTPClientSettings{
			URI: endpoints[0],
			Client: &Client{
				Timeout: time.Duration(10) * time.Second,
			},
		},
	)
	if err != nil {
		t.Error(err)
	}
	gliResp1, err1 := api.GetLatestInclusion(txs)
	fmt.Printf("fun = 'GetLatestInclusion' gliResp = %v err1 = %v\n", gliResp1, err1)

	resp, err2 := __polyCall__(api, "GetLatestInclusion", txs)
	if err2 == nil {
		gliResp2 := resp.([]bool)
		fmt.Printf("fun = 'gli' resp = %v err1 = %v\n", gliResp2, err2)
		if !equalBoolSlice(gliResp1, gliResp2) {
			t.Errorf("assert(gliResp1 == gliResp2)")
		}
	} else {
		fmt.Printf("%v\n", err2)
	}

	gttaResp1, err1 := api.GetTransactionsToApprove(3)
	fmt.Printf("fun = 'GetTransactionsToApprove' resp1 = %v err1 = %v\n", gttaResp1, err1)

	gttaResp2tmp, err2 := __polyCall__(api, "GetTransactionsToApprove", uint64(3))
	if err2 == nil {
		_, ok := gttaResp2tmp.(*TransactionsToApprove)
		if !ok {
			t.Errorf("Wrong return type")
		}
	} else {
		fmt.Printf("%v\n", err2)
	}
	fmt.Printf("fun = 'gtta' resp2 = %v err2 = %v\n", gttaResp2tmp, err2)
}

func Test_GetBalances(t *testing.T) {
	api, err := ComposeAPI(
		HTTPClientSettings{
			URI: endpoints[0],
			Client: &Client{
				Timeout: time.Duration(10) * time.Second,
			},
		},
	)
	if err != nil {
		t.Error(err)
	}
	mapi, err := New(endpoints, 10)
	if err != nil || mapi == nil {
		t.Errorf("Must return correct mapi and err==nil")
	}

	started := time.Now()
	balOrig, errOrig := api.GetBalances(addrs, 100)
	fmt.Printf("Original GetBalances bal = %v err = %v ep = %v duration = %v\n",
		balOrig, errOrig, endpoints[0], time.Since(started))
	var res MultiCallRet
	balMapi, errMapi := mapi.GetBalances(addrs, uint64(100), &res)
	fmt.Printf("MAPI GetBalances bal = %v err = %v res = %v\n", balMapi, errMapi, res)
	balMapi2, errMapi2 := mapi.GetBalances(addrs, uint64(100))
	fmt.Printf("MAPI GetBalances bal = %v err = %v res = ???\n", balMapi2, errMapi2)
	if !equalGetBalanceResponses(balMapi, balOrig) {
		t.Errorf("responses of getBalances not equal")
	}
}

func Test_FindTransactions(t *testing.T) {
	api, err := ComposeAPI(
		HTTPClientSettings{
			URI: endpoints[0],
			Client: &Client{
				Timeout: time.Duration(10) * time.Second,
			},
		},
	)
	if err != nil {
		t.Error(err)
	}
	mapi, err := New(endpoints, 10)
	if err != nil || mapi == nil {
		t.Errorf("Must return correct mapi and err==nil")
	}

	started := time.Now()
	respOrig, errOrig := api.FindTransactions(FindTransactionsQuery{
		Addresses: addrs,
	})
	fmt.Printf("Original FindTransactions resp = %v err = %v ep = %v duration = %v\n",
		respOrig, errOrig, endpoints[0], time.Since(started))
	var res MultiCallRet
	respMapi, errMapi := mapi.FindTransactions(FindTransactionsQuery{
		Addresses: addrs,
	})
	fmt.Printf("Original FindTransactions resp = %v err = %v ep = %v duration = %v\n",
		respMapi, errMapi, nil, nil)
	respMapi, errMapi = mapi.FindTransactions(FindTransactionsQuery{
		Addresses: addrs,
	}, &res)
	fmt.Printf("Original FindTransactions resp = %v err = %v ep = %v duration = %v\n",
		respMapi, errMapi, res.Endpoint, res.Duration)

	if !equalHashes(respOrig, respMapi) {
		t.Errorf("responses of FindTransactions not equal")
	}
}

func equalHashes(h1, h2 Hashes) bool {
	if len(h1) != len(h2) {
		return false
	}
	for i := range h1 {
		if h1[i] != h2[i] {
			return false
		}
	}
	return true
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

func equalUint64Slice(ba1, ba2 []uint64) bool {
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

func equalGetBalanceResponses(bal1, bal2 *Balances) bool {
	return bal1.MilestoneIndex == bal2.MilestoneIndex &&
		bal1.Milestone == bal2.Milestone &&
		equalUint64Slice(bal1.Balances, bal2.Balances)
}
