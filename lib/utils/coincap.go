package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

type jsonReadStruct struct {
	Data map[string]string `json:"data"`
}

func GetMiotaPriceUSD() (float64, error) {
	var data []byte
	var unm jsonReadStruct

	response, err := http.Get("https://api.coincap.io/v2/assets/iota")
	if err != nil {
		return 0, fmt.Errorf("GetMiotaPriceUSD: %v", err)
	}
	data, _ = ioutil.ReadAll(response.Body)
	err = json.Unmarshal(data, &unm)
	if err != nil {
		return 0, fmt.Errorf("GetMiotaPriceUSD: %v", err)
	}
	pstr, ok := unm.Data["priceUsd"]
	if !ok {
		return 0, fmt.Errorf("GetMiotaPriceUSD: json parse error")

	}
	f, err := strconv.ParseFloat(pstr, 64)
	if err != nil {
		return 0, fmt.Errorf("GetMiotaPriceUSD: %v", err)
	}
	return f, nil
}
