package main

import (
	"math"
	"net/url"

	"github.com/montanaflynn/stats"
)

type VastAiOffer struct {
	MachineId int     `json:"machine_id"`
	GpuName   string  `json:"gpu_name"`
	NumGpus   int     `json:"num_gpus"`
	GpuFrac   float64 `json:"gpu_frac"`
	DphBase   float64 `json:"dph_base"`
	Verified  bool
}

type GroupedOffers map[string][]VastAiOffer

type OfferStats struct {
	Count                                 int
	Median, PercentileLow, PercentileHigh float64
}

type OfferStats2 struct {
	Verified, Unverified, All OfferStats
}

func loadOffers(result *VastAiApiResults) error {
	var verified, unverified struct {
		Offers []VastAiOffer `json:"offers"`
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
	offers := mergeOffers(verified.Offers, unverified.Offers)
	result.offers = &offers
	return nil
}

func mergeOffers(verified []VastAiOffer, unverified []VastAiOffer) []VastAiOffer {
	result := []VastAiOffer{}
	for _, offer := range verified {
		offer.Verified = true
		result = append(result, offer)
	}
	for _, offer := range unverified {
		offer.Verified = false
		result = append(result, offer)
	}
	return result
}

func groupOffersByGpuName(offers []VastAiOffer) GroupedOffers {
	offersByGpu := make(map[string][]VastAiOffer)
	for _, offer := range offers {
		name := offer.GpuName
		offersByGpu[name] = append(offersByGpu[name], offer)
	}
	return offersByGpu
}

func filterOffers(offers []VastAiOffer, filter func(*VastAiOffer) bool) []VastAiOffer {
	result := []VastAiOffer{}
	for _, offer := range offers {
		if filter(&offer) {
			result = append(result, offer)
		}
	}
	return result
}

func offerStats(offers []VastAiOffer) OfferStats {
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

func offerStats2(offers []VastAiOffer) OfferStats2 {
	return OfferStats2{
		Verified:   offerStats(filterOffers(offers, func(offer *VastAiOffer) bool { return offer.Verified })),
		Unverified: offerStats(filterOffers(offers, func(offer *VastAiOffer) bool { return !offer.Verified })),
		All:        offerStats(offers),
	}
}
