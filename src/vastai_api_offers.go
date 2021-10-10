package main

import (
	"math"
	"net/url"

	"github.com/montanaflynn/stats"
	"github.com/prometheus/common/log"
)

type VastAiRawOffer map[string]interface{}
type VastAiRawOffers []VastAiRawOffer

type VastAiOffer struct {
	MachineId int
	GpuName   string
	NumGpus   int
	GpuFrac   float64
	DphBase   float64
	Verified  bool
}
type VastAiOffers []VastAiOffer

type GroupedOffers map[string]VastAiOffers

type OfferStats struct {
	Count                                 int
	Median, PercentileLow, PercentileHigh float64
}

type OfferStats2 struct {
	Verified, Unverified, All OfferStats
}

func loadOffers(result *VastAiApiResults) error {
	var verified, unverified struct {
		Offers VastAiRawOffers `json:"offers"`
	}
	if err := vastApiCall(&verified, "bundles", url.Values{
		"q": {`{"external":{"eq":"false"},"verified":{"eq":"true"},"type":"on-demand","disable_bundling":true}`},
	}); err != nil {
		return err
	}
	if err := vastApiCall(&unverified, "bundles", url.Values{
		"q": {`{"external":{"eq":"false"},"verified":{"eq":"false"},"type":"on-demand","disable_bundling":true}`},
	}); err != nil {
		return err
	}
	rawOffers := mergeRawOffers(verified.Offers, unverified.Offers)
	offers := rawOffers.decode()
	result.rawOffers = &rawOffers
	result.offers = &offers
	return nil
}

func mergeRawOffers(verified VastAiRawOffers, unverified VastAiRawOffers) VastAiRawOffers {
	result := VastAiRawOffers{}
	for _, offer := range verified {
		offer["verified"] = true
		result = append(result, offer)
	}
	for _, offer := range unverified {
		offer["verified"] = false
		result = append(result, offer)
	}
	return result
}

func (offers VastAiRawOffers) filter(filter func(*VastAiRawOffer) bool) VastAiRawOffers {
	result := VastAiRawOffers{}
	for _, offer := range offers {
		if filter(&offer) {
			result = append(result, offer)
		}
	}
	return result
}

func (offers VastAiRawOffers) decode() VastAiOffers {
	result := VastAiOffers{}
	for _, offer := range offers {
		machineId, ok1 := offer["machine_id"].(float64)
		gpuName, ok2 := offer["gpu_name"].(string)
		numGpus, ok3 := offer["num_gpus"].(float64)
		gpuFrac, ok4 := offer["gpu_frac"].(float64)
		dphBase, ok5 := offer["dph_base"].(float64)
		if ok1 && ok2 && ok3 && ok4 && ok5 {
			result = append(result, VastAiOffer{
				MachineId: int(machineId),
				GpuName:   gpuName,
				NumGpus:   int(numGpus),
				GpuFrac:   gpuFrac,
				DphBase:   dphBase,
				Verified:  offer["verified"].(bool),
			})
		} else {
			log.Errorln("Invalid offer record:", offer)
		}
	}
	return result
}

func (offers VastAiOffers) groupByGpu() GroupedOffers {
	offersByGpu := make(GroupedOffers)
	for _, offer := range offers {
		name := offer.GpuName
		offersByGpu[name] = append(offersByGpu[name], offer)
	}
	return offersByGpu
}

func (offers VastAiOffers) filter(filter func(*VastAiOffer) bool) VastAiOffers {
	result := VastAiOffers{}
	for _, offer := range offers {
		if filter(&offer) {
			result = append(result, offer)
		}
	}
	return result
}

func (offers VastAiOffers) stats() OfferStats {
	prices := []float64{}
	for _, offer := range offers {
		pricePerGpu := offer.DphBase / float64(offer.NumGpus)
		for i := 0; i < offer.NumGpus; i++ {
			prices = append(prices, pricePerGpu)
		}
	}

	result := OfferStats{
		Count:          len(prices),
		Median:         math.NaN(),
		PercentileLow:  math.NaN(),
		PercentileHigh: math.NaN(),
	}
	if len(prices) > 0 {
		result.Median, _ = stats.Median(prices)
		result.PercentileLow, _ = stats.Percentile(prices, 10)
		result.PercentileHigh, _ = stats.Percentile(prices, 90)
	}
	return result
}

func (offers VastAiOffers) stats2() OfferStats2 {
	return OfferStats2{
		Verified:   offers.filter(func(offer *VastAiOffer) bool { return offer.Verified }).stats(),
		Unverified: offers.filter(func(offer *VastAiOffer) bool { return !offer.Verified }).stats(),
		All:        offers.stats(),
	}
}
