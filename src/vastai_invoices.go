package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type VastAiInvoices struct {
	Current struct {
		Total float64 `json:"total"`
	} `json:"current"`
}

func getPendingPayout(userId int) (float64, error) {
	var invoices VastAiInvoices
	resp, err := http.Get(fmt.Sprintf("https://vast.ai/api/v0/users/%d/invoices/?api_key=%s", userId, *apiKey))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	err = json.Unmarshal(body, &invoices)
	if err != nil {
		return 0, err
	}
	return invoices.Current.Total, nil
}
