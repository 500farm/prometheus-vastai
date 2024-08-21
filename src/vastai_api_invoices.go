package main

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/prometheus/common/log"
)

type VastAiInvoices struct {
	Current struct {
		Charges float64 `json:"charges"`
	} `json:"current"`
	Invoices []struct {
		Type   string  `json:"type"`
		Amount float64 `json:"amount"`
		Ts     float64 `json:"timestamp"`
	} `json:"invoices"`
}

type PayoutInfo struct {
	PaidOut        float64 `json:"paidOut"`
	PendingPayout  float64 `json:"pendingPayout"`
	LastPayoutTime float64 `json:"lastPayoutTime"`
}

// TODO collect payouts since the beginning of time
func getPayouts() (*PayoutInfo, error) {
	var data VastAiInvoices
	err := vastApiCall(&data, "users/current/invoices", nil, defaultTimeout)
	if err != nil {
		return nil, err
	}
	paidOut := int64(0) // in cents to avoid precision loss when summing
	lastPayoutTime := 0.0
	for _, invoice := range data.Invoices {
		if invoice.Type == "payment" {
			amount := invoice.Amount
			if amount > 0 {
				paidOut += int64(amount * 100)
				if invoice.Ts > lastPayoutTime {
					lastPayoutTime = invoice.Ts
				}
			}
		}
	}
	return &PayoutInfo{float64(paidOut) / 100, data.Current.Charges, lastPayoutTime}, nil
}

func readLastPayouts() *PayoutInfo {
	j, err := ioutil.ReadFile(*stateDir + "/.vastai_last_payouts")
	if err != nil {
		if !os.IsNotExist(err) {
			log.Errorln(err)
		}
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
	err := ioutil.WriteFile(*stateDir+"/.vastai_last_payouts", j, 0600)
	if err != nil {
		log.Errorln(err)
	}
}
