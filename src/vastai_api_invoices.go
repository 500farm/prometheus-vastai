package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/prometheus/common/log"
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
	PaidOut       float64 `json:"paidOut"`
	PendingPayout float64 `json:"pendingPayout"`
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
	paidOut := int64(0) // in cents to avoid precision loss when summing
	for _, invoice := range data.Invoices {
		if invoice.Type == "payment" {
			amount, _ := strconv.ParseFloat(invoice.Amount, 64)
			paidOut += int64(amount * 100)
		}
	}
	return &PayoutInfo{float64(paidOut) / 100, data.Current.Total}, nil
}

func readLastPayouts() *PayoutInfo {
	j, err := ioutil.ReadFile(os.Getenv("HOME") + "/.vastai_last_payouts")
	if err != nil {
		log.Errorln(err)
		return nil
	}
	var payouts PayoutInfo
	err = json.Unmarshal(j, &payouts)
	if err != nil {
		log.Errorln(err)
		return nil
	}
	return &payouts
}

func storeLastPayouts(payouts *PayoutInfo) {
	j, _ := json.Marshal(payouts)
	err := ioutil.WriteFile(os.Getenv("HOME")+"/.vastai_last_payouts", j, 0600)
	if err != nil {
		log.Errorln(err)
	}
}
