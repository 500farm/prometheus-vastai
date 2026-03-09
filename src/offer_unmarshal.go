package main

import (
	"fmt"
	"sync"

	jsonv2 "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

func (s *VastAiRawOffers) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	if tok.Kind() != '[' {
		return fmt.Errorf("expected '[', got %q", tok.Kind())
	}

	var rawElements []jsontext.Value
	for dec.PeekKind() != ']' {
		val, err := dec.ReadValue()
		if err != nil {
			return err
		}
		rawElements = append(rawElements, val)
	}

	if _, err := dec.ReadToken(); err != nil {
		return err
	}

	offers := make(VastAiRawOffers, len(rawElements))

	workers := 1
	if len(rawElements) >= 100 {
		workers = numWorkers()
	}

	var firstErr error
	var errOnce sync.Once

	parallelDo(len(rawElements), workers, func(w, start, end int) {
		for i := start; i < end; i++ {
			var m VastAiRawOffer
			if err := jsonv2.Unmarshal(rawElements[i], &m,
				jsontext.AllowDuplicateNames(true),
			); err != nil {
				errOnce.Do(func() { firstErr = err })
				return
			}
			offers[i] = m
		}
	})

	if firstErr != nil {
		return firstErr
	}

	*s = offers
	return nil
}
