package inputpart

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"sync"
)

const endpoint1 = "http://88.99.60.78:15265"
const endpoint2 = "http://159.69.9.6:15265"

var fields = []string{"avg_5", "avg_10", "avg_15", "avg_30"}

type respStruct struct {
	data []byte
	err  error
}

func callEndpoint(endp string, result chan *respStruct) {
	response, err := http.Get(endp)
	if err != nil {
		result <- &respStruct{
			data: nil,
			err:  err,
		}
		return
	}
	defer response.Body.Close() // https://golang.org/pkg/net/http/

	var data []byte
	data, err = ioutil.ReadAll(response.Body)
	result <- &respStruct{
		data: data,
		err:  err,
	}
}

func callMultiEnpoints(args ...string) ([]byte, error) {
	resultCh := make(chan *respStruct, len(args)+1)
	var wg sync.WaitGroup

	wg.Add(len(args))
	for _, ep := range args {
		epcopy := ep
		go func() {
			callEndpoint(epcopy, resultCh)
			wg.Done()
		}()
	}
	go func() {
		// close after all async calls done
		wg.Wait()
		close(resultCh)
	}()

	var err error
	for r := range resultCh {
		err = r.err
		if err == nil {
			go func() {
				// drain the rest of the channel until closed
				for range resultCh {
				}
			}()
			return r.data, nil
		}
	}
	return nil, err // last error
}

// deprecated, 2020.03.05 not deleted yet

func GetLMConfRate() (map[string]float64, error) {
	var unm interface{}

	ret := make(map[string]float64)

	data, err := callMultiEnpoints(endpoint1, endpoint2)
	if err != nil {
		return nil, fmt.Errorf("GetLMConfRate: %v", err)
	}
	err = json.Unmarshal(data, &unm)
	if err != nil {
		return nil, fmt.Errorf("GetLMConfRate: %v", err)
	}

	datamap, ok := unm.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("GetLMConfRate: json parsing error")
	}

	datamap1, ok := datamap["results"]
	if !ok {
		return nil, fmt.Errorf("GetLMConfRate: json parse error 1")
	}
	res, ok := datamap1.(map[string]interface{})

	//var sv string
	var iv interface{}
	var fv float64
	for _, f := range fields {
		ret[f] = 0
		if iv, ok = res[f]; ok {
			if fv, ok = iv.(float64); ok {
				ret[f] = math.Round(fv * 100)
			}
		}
	}
	for k := range ret {
		if ret[k] < 0 {
			ret[k] = 0
		}
	}
	return ret, nil
}
