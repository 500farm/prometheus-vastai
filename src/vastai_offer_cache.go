package main

import (
	"errors"
	"log"
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
	machines   VastAiMachineOffers
	responses  SerializedResponses
	ts         time.Time
}

var offerCache OfferCache

func (cache *OfferCache) UpdateFrom(apiRes VastAiApiResults) {
	if apiRes.offers != nil {
		done := timeStage("decode")
		offers := (*apiRes.offers).decode()
		done()

		done = timeStage("collect_machines")
		machines := offers.collectMachineOffers()
		done()

		log.Println("INFO:", len(offers), "offers,", len(machines), "machines")

		responses := NewSerializedResponses(offers, machines, apiRes.ts)

		cache.mu.Lock()
		cache.offerCount = len(offers)
		cache.machines = machines
		cache.responses = responses
		cache.ts = apiRes.ts
		cache.mu.Unlock()

		runtime.GC()

		if metrics != nil {
			metrics.UpdateCounts(len(offers), len(machines))
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
