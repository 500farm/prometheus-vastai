package main

import (
	"math"

	"github.com/montanaflynn/stats"
)

type VastAiOffer struct {
	MachineId     int
	GpuName       string
	NumGpus       int
	NumGpusRented int
	PricePerGpu   int // in cents
	Verified      bool
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

type OfferStats3 struct {
	Rented, Available, All OfferStats2
}

func (offers VastAiRawOffers) decode() VastAiOffers {
	result := VastAiOffers{}
	for _, offer := range offers {
		result = append(result, VastAiOffer{
			MachineId:     offer.machineId(),
			GpuName:       offer.gpuName(),
			NumGpus:       offer.numGpus(),
			NumGpusRented: offer.numGpusRented(),
			PricePerGpu:   offer.pricePerGpu(),
			Verified:      offer.verified(),
		})
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

func (offers VastAiOffers) filter(filter func(VastAiOffer) bool) VastAiOffers {
	return offers.filter2(filter, nil)
}

func (offers VastAiOffers) filter2(filter func(VastAiOffer) bool, postProcess func(VastAiOffer) VastAiOffer) VastAiOffers {
	result := VastAiOffers{}
	for _, offer := range offers {
		if filter(offer) {
			if postProcess != nil {
				result = append(result, postProcess(offer))
			} else {
				result = append(result, offer)
			}
		}
	}
	return result
}

func (offers VastAiOffers) filterVerified() VastAiOffers {
	return offers.filter(func(offer VastAiOffer) bool { return offer.Verified })
}

func (offers VastAiOffers) filterUnverified() VastAiOffers {
	return offers.filter(func(offer VastAiOffer) bool { return !offer.Verified })
}

func (offers VastAiOffers) filterRented() VastAiOffers {
	return offers.filter2(
		func(offer VastAiOffer) bool { return offer.NumGpusRented > 0 },
		func(offer VastAiOffer) VastAiOffer {
			newOffer := offer
			newOffer.NumGpus = offer.NumGpusRented
			return newOffer
		},
	)
}

func (offers VastAiOffers) filterAvailable() VastAiOffers {
	return offers.filter2(
		func(offer VastAiOffer) bool { return offer.NumGpusRented < offer.NumGpus },
		func(offer VastAiOffer) VastAiOffer {
			newOffer := offer
			newOffer.NumGpus -= offer.NumGpusRented
			return newOffer
		},
	)
}

func (offers VastAiOffers) stats() OfferStats {
	prices := []float64{}
	for _, offer := range offers {
		pricePerGpu := offer.PricePerGpu
		for i := 0; i < offer.NumGpus; i++ {
			prices = append(prices, float64(pricePerGpu))
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
		Verified:   offers.filterVerified().stats(),
		Unverified: offers.filterUnverified().stats(),
		All:        offers.stats(),
	}
}

func (offers VastAiOffers) stats3() OfferStats3 {
	return OfferStats3{
		Rented:    offers.filterRented().stats2(),
		Available: offers.filterAvailable().stats2(),
		All:       offers.stats2(),
	}
}
