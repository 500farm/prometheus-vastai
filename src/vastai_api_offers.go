package main

import (
	"math"

	"github.com/montanaflynn/stats"
)

type VastAiOffer struct {
	MachineId int     `json:"machine_id"`
	GpuName   string  `json:"gpu_name"`
	NumGpus   int     `json:"num_gpus"`
	GpuFrac   float64 `json:"gpu_frac"`
	DlPerf    float64 `json:"dlperf"`
	DphBase   float64 `json:"dph_base"`
}

type GroupedOffers map[string][]VastAiOffer

type OfferStats struct {
	Count                                 int
	Median, PercentileLow, PercentileHigh float64
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
