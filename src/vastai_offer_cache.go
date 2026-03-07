package main

import (
	"errors"
	"sync"
	"time"
)

func timeStage(stage string) func() {
	if metrics != nil {
		return metrics.TimeProcessing(stage)
	}
	return func() {}
}

type OfferCache struct {
	mu                    sync.RWMutex
	rawOffers             VastAiRawOffers
	wholeMachineRawOffers VastAiRawOffers
	machines              VastAiOffers
	ts                    time.Time
}

var offerCache OfferCache

func (cache *OfferCache) UpdateFrom(apiRes VastAiApiResults) {
	if apiRes.offers != nil {
		prev := cache.Snapshot()

		done := timeStage("validate_dedupe")
		rawOffers := (*apiRes.offers).validate().dedupe()
		done()

		done = timeStage("collect_whole_machines")
		wholeMachineRawOffers := rawOffers.collectWholeMachines(prev.wholeMachineRawOffers)
		done()

		done = timeStage("decode")
		machines := wholeMachineRawOffers.decode()
		done()

		cache.mu.Lock()

		cache.rawOffers = rawOffers
		cache.wholeMachineRawOffers = wholeMachineRawOffers
		cache.machines = machines
		cache.ts = apiRes.ts

		cache.mu.Unlock()

		if metrics != nil {
			metrics.UpdateCounts(len(rawOffers), len(wholeMachineRawOffers))
		}

		if geoCache != nil {
			geoCache.save()
		}
	}
}

func (cache *OfferCache) InitialUpdateFrom(apiRes VastAiApiResults) error {
	if apiRes.offers == nil {
		return errors.New("could not read offer data from Vast.ai")
	}
	cache.UpdateFrom(apiRes)
	return nil
}

type RawOffersResponse struct {
	Url       string           `json:"url"`
	Timestamp time.Time        `json:"timestamp"`
	Count     int              `json:"count"`
	Note      string           `json:"note,omitempty"`
	Offers    *VastAiRawOffers `json:"offers"`
}
