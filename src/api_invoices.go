package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/url"
	"os"
	"time"
)

type VastAiInvoices struct {
	Current struct {
		Charges float64 `json:"charges"` // float in dollars
	} `json:"current"`
	Invoices []struct {
		Type   string  `json:"type"`      // handling "payment" only
		Amount float64 `json:"amount"`    // float in dollars, positive for payouts, negative for charges
		Ts     float64 `json:"timestamp"` // unix timestamp
	} `json:"invoices"`
}

type PayoutInfo struct {
	PaidOut        float64 `json:"paidOut"`
	PendingPayout  float64 `json:"pendingPayout"`
	LastPayoutTime float64 `json:"lastPayoutTime"`
}

type VastAiInvoice2 struct {
	Ts          float64 `json:"when"`         // unix timestamp
	AmountCents int64   `json:"amount_cents"` // in cents, positive for payouts, negative for charges
}

type InvoiceState struct {
	LastInvoice  VastAiInvoice2 `json:"lastInvoice"`
	PaidOutCents int64          `json:"paidOutCents"`
}

func getPayouts() (*PayoutInfo, error) {
	// old api — provides only recent invoices

	var data VastAiInvoices
	err := vastApiCall(&data, "users/current/invoices", nil, defaultTimeout)
	if err != nil {
		return nil, err
	}
	lastPayoutTime := 0.0
	for _, invoice := range data.Invoices {
		if invoice.Type == "payment" && invoice.Amount > 0 && invoice.Ts > lastPayoutTime {
			lastPayoutTime = invoice.Ts
		}
	}

	// new api — provides lifetime invoices but we're trying to request incrementally

	state := readInvoiceState()

	args := url.Values{}
	args.Set("select_cols", jsonArg([]string{"when", "amount_cents"}))

	if state != nil {
		args.Set("select_filters", jsonArg(map[string]any{
			"when": map[string]any{
				"gt": state.LastInvoice.Ts,
			},
		}))
	}

	var data2 []VastAiInvoice2
	err = vastApiCall(&data2, "invoices", args, defaultTimeout)
	if err != nil {
		return nil, err
	}

	if state != nil {
		log.Printf("INFO: received %d new invoices after %s", len(data2),
			time.Unix(int64(state.LastInvoice.Ts), 0).Format(time.RFC3339))
	} else {
		log.Printf("INFO: received %d invoices (initial fetch)", len(data2))
	}

	paidOutCents := int64(0)
	if state != nil {
		paidOutCents = state.PaidOutCents
	}

	for _, invoice := range data2 {
		if invoice.AmountCents > 0 {
			paidOutCents += invoice.AmountCents
		}
	}

	if len(data2) > 0 {
		storeInvoiceState(&InvoiceState{
			LastInvoice:  data2[len(data2)-1],
			PaidOutCents: paidOutCents,
		})
	}

	return &PayoutInfo{
		PaidOut:        float64(paidOutCents) / 100,
		PendingPayout:  data.Current.Charges,
		LastPayoutTime: lastPayoutTime,
	}, nil
}

func jsonArg(v any) string {
	j, _ := json.Marshal(v)
	return string(j)
}

func readLastPayouts() *PayoutInfo {
	j, err := os.ReadFile(*stateDir + "/.vastai_last_payouts")
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Println("ERROR:", err)
		}
		return nil
	}
	var payouts PayoutInfo
	err = json.Unmarshal(j, &payouts)
	if err != nil {
		log.Println("ERROR:", err)
		return nil
	}
	return &payouts
}

func storeLastPayouts(payouts *PayoutInfo) {
	j, err := json.Marshal(payouts)
	if err != nil {
		log.Println("ERROR:", err)
		return
	}
	err = os.WriteFile(*stateDir+"/.vastai_last_payouts", j, 0600)
	if err != nil {
		log.Println("ERROR:", err)
	}
}

func readInvoiceState() *InvoiceState {
	j, err := os.ReadFile(*stateDir + "/.vastai_invoice_state")
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Println("ERROR:", err)
		}
		return nil
	}
	var state InvoiceState
	err = json.Unmarshal(j, &state)
	if err != nil {
		log.Println("ERROR:", err)
		return nil
	}
	return &state
}

func storeInvoiceState(state *InvoiceState) {
	j, err := json.Marshal(state)
	if err != nil {
		log.Println("ERROR:", err)
		return
	}
	err = os.WriteFile(*stateDir+"/.vastai_invoice_state", j, 0600)
	if err != nil {
		log.Println("ERROR:", err)
	}
}
