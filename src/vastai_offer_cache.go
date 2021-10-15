package main

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/prometheus/common/log"
)

type OfferCache struct {
	offers    VastAiOffers
	rawOffers VastAiRawOffers
	ts        time.Time
}

var offerCache OfferCache

func (cache *OfferCache) UpdateFrom(apiRes VastAiApiResults) {
	if apiRes.offersVerified != nil && apiRes.offersUnverified != nil {
		cache.rawOffers = mergeRawOffers(*apiRes.offersVerified, *apiRes.offersUnverified)
		cache.offers = cache.rawOffers.filterWholeMachines().decode()
		cache.ts = apiRes.ts
	}
}

func (cache *OfferCache) InitialUpdateFrom(apiRes VastAiApiResults) error {
	if apiRes.offersVerified == nil || apiRes.offersUnverified == nil {
		return errors.New("Could not read offer data from Vast.ai")
	}
	cache.UpdateFrom(apiRes)
	return nil
}

type RawOffersResponse struct {
	Url       string           `json:"url"`
	Timestamp time.Time        `json:"timestamp"`
	Count     int              `json:"count"`
	Offers    *VastAiRawOffers `json:"offers"`
}

func (cache *OfferCache) rawOffersJson(wholeMachines bool) []byte {
	offers := &cache.rawOffers
	url := "/raw-offers"
	if wholeMachines {
		filtered := offers.filterWholeMachines()
		offers = &filtered
		url += "/whole-machines"
	}
	result, err := json.MarshalIndent(RawOffersResponse{
		Url:       url,
		Timestamp: cache.ts.UTC(),
		Count:     len(*offers),
		Offers:    offers,
	}, "", "    ")
	if err != nil {
		log.Errorln(err)
		return nil
	}
	return result
}
