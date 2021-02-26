package main

import (
	"encoding/json"
	"fmt"
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

func getPayouts(userId int) (float64, float64, error) {
	var data VastAiInvoices
	resp, err := http.Get(fmt.Sprintf("https://vast.ai/api/v0/users/%d/invoices/?api_key=%s", userId, *apiKey))
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, err
	}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return 0, 0, err
	}
	paidOut := float64(0)
	for _, invoice := range data.Invoices {
		if invoice.Type == "payment" {
			amount, _ := strconv.ParseFloat(invoice.Amount, 64)
			paidOut += amount
		}
	}
	return paidOut, data.Current.Total, nil
}
