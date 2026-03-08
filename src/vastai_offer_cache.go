package main

import (
	"errors"
	"runtime"
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
	mu         sync.RWMutex
	offerCount int
	machines   VastAiOffers
	responses  SerializedResponses
	ts         time.Time
}

var offerCache OfferCache

func (cache *OfferCache) UpdateFrom(apiRes VastAiApiResults) {
	if apiRes.offers != nil {
		done := timeStage("validate_dedupe")
		rawOffers := (*apiRes.offers).validate().dedupe()
		done()

		done = timeStage("collect_whole_machines")
		wholeMachineRawOffers := rawOffers.collectWholeMachines()
		done()

		done = timeStage("decode")
		machines := wholeMachineRawOffers.decode()
		done()

		responses := NewSerializedResponses(rawOffers, wholeMachineRawOffers, machines, apiRes.ts)

		cache.mu.Lock()
		cache.offerCount = len(rawOffers)
		cache.machines = machines
		cache.responses = responses
		cache.ts = apiRes.ts
		cache.mu.Unlock()

		runtime.GC()

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
