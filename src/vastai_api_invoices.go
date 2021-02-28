package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
)

type VastAiInvoices struct {
	Current struct {
		Total float64 `json:"total"`
	} `json:"current"`
	Invoices []struct {
		Type   string `json:"type"`
		Amount string `json:"amount"`
	} `json:"invoices"`
}

type PayoutInfo struct {
	paidOut       float64
	pendingPayout float64
}

func getPayouts() (*PayoutInfo, error) {
	var data VastAiInvoices
	resp, err := http.Get("https://vast.ai/api/v0/users/current/invoices/?api_key=" + *apiKey)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}
	paidOut := float64(0)
	for _, invoice := range data.Invoices {
		if invoice.Type == "payment" {
			amount, _ := strconv.ParseFloat(invoice.Amount, 64)
			paidOut += amount
		}
	}
	return &PayoutInfo{paidOut, data.Current.Total}, nil
}
