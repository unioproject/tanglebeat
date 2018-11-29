package multiapi

import (
	"fmt"
	. "github.com/iotaledger/iota.go/api"
	. "net/http"
	"testing"
	"time"
)

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
