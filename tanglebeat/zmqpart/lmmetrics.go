package zmqpart

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
)

type jsonReadStruct struct {
	Data map[string]map[string]string `json:"data"`
}

const endpoint1 = "http://88.99.60.78:15265"

// const endpoint2 = "http://159.69.9.6:15265"

var fields = []string{"avg_5", "avg_10", "avg_15", "avg_30"}

func GetLMConfRate() (map[string]float64, error) {
	var data []byte
	var unm interface{}

	ret := make(map[string]float64)

	response, err := http.Get(endpoint1)
	if err != nil {
		return nil, fmt.Errorf("GetLMConfRate: %v", err)
	}
	defer response.Body.Close() // https://golang.org/pkg/net/http/

	data, _ = ioutil.ReadAll(response.Body)
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
	return ret, nil
}
