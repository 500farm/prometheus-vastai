package main

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/prometheus/common/log"
)

type OfferCache struct {
	rawOffers             VastAiRawOffers
	wholeMachineRawOffers VastAiRawOffers
	machines              VastAiOffers
	ts                    time.Time
}

var offerCache OfferCache

func (cache *OfferCache) UpdateFrom(apiRes VastAiApiResults) {
	if apiRes.offersVerified != nil && apiRes.offersUnverified != nil {
		cache.rawOffers = mergeRawOffers(*apiRes.offersVerified, *apiRes.offersUnverified).validate()
		cache.wholeMachineRawOffers = cache.rawOffers.validate().filterWholeMachines()
		cache.machines = cache.wholeMachineRawOffers.decode()
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
	var offers *VastAiRawOffers
	url := "/offers"
	if wholeMachines {
		offers = &cache.wholeMachineRawOffers
		url = "/machines"
	} else {
		offers = &cache.rawOffers
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
