package main

import (
	"encoding/json"
	"math"

	"github.com/montanaflynn/stats"
)

type VastAiOffer struct {
	MachineId         int
	GpuName           string
	NumGpus           int
	NumGpusRented     int
	PricePerGpu       int // in cents
	Verified          bool
	Datacenter        bool
	StaticIp          bool
	Vram              float64
	DlperfPerGpuChunk float64
	DlperfPerGpuWhole float64
	TflopsPerGpu      float64
}
type VastAiOffers []VastAiOffer

type GroupedOffers map[string]VastAiOffers

type OfferStats struct {
	Count             int
	Median            float64
	PercentileLow     float64
	PercentileHigh    float64
	CountByPriceRange map[int]int // price in cents, in $0.05 increments
}

type GpuInfo struct {
	Vram   float64 `json:"vram"`
	Dlperf float64 `json:"dlperf"`
	Tflops float64 `json:"tflops"`
}

type OfferStats2 struct {
	Verified   OfferStats `json:"verified"`
	Unverified OfferStats `json:"unverified"`
	All        OfferStats `json:"all"`
}

type OfferStats3 struct {
	Rented    OfferStats2 `json:"rented"`
	Available OfferStats2 `json:"available"`
	All       OfferStats2 `json:"all"`
}

func (offers VastAiRawOffers) decode() VastAiOffers {
	result := make(VastAiOffers, 0, len(offers))
	for _, offer := range offers {
		numGpus := offer.numGpus()
		decoded := VastAiOffer{
			MachineId:     offer.machineId(),
			GpuName:       offer.gpuName(),
			NumGpus:       numGpus,
			NumGpusRented: offer.numGpusRented(),
			PricePerGpu:   offer.pricePerGpu(),
			Verified:      offer.verified(),
			Datacenter:    offer.datacenter(),
			StaticIp:      offer.staticIp(),
		}
		vram, _ := offer["gpu_ram"].(float64)
		dlperf, _ := offer["dlperf"].(float64)
		dlperfChunk, _ := offer["dlperf_chunk"].(float64)
		tflops, _ := offer["total_flops"].(float64)
		decoded.Vram = math.Ceil(vram / 1024)
		decoded.DlperfPerGpuWhole = dlperf / float64(numGpus)
		decoded.DlperfPerGpuChunk = dlperfChunk / float64(numGpus)
		decoded.TflopsPerGpu = tflops / float64(numGpus)
		result = append(result, decoded)
	}
	return result
}

func (offers VastAiOffers) groupByGpu() GroupedOffers {
	offersByGpu := make(GroupedOffers)
	for _, offer := range offers {
		name := offer.GpuName
		if name != "" {
			offersByGpu[name] = append(offersByGpu[name], offer)
		}
	}
	return offersByGpu
}

func (offers VastAiOffers) filter(filter func(VastAiOffer) bool) VastAiOffers {
	return offers.filter2(filter, nil)
}

func (offers VastAiOffers) filter2(filter func(VastAiOffer) bool, postProcess func(VastAiOffer) VastAiOffer) VastAiOffers {
	result := make(VastAiOffers, 0, len(offers))
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

func (offers VastAiOffers) stats(perDlPerf bool) OfferStats {
	prices := []float64{}
	for _, offer := range offers {
		pricePerGpu := float64(offer.PricePerGpu)
		if perDlPerf {
			pricePerGpu = math.Floor(pricePerGpu * 100.0 / offer.DlperfPerGpuChunk)
		}
		for i := 0; i < offer.NumGpus; i++ {
			prices = append(prices, pricePerGpu)
		}
	}

	result := OfferStats{
		Count:             len(prices),
		Median:            math.NaN(),
		PercentileLow:     math.NaN(),
		PercentileHigh:    math.NaN(),
		CountByPriceRange: make(map[int]int),
	}
	if len(prices) > 0 {
		result.Median, _ = stats.Median(prices)
		result.PercentileLow, _ = stats.Percentile(prices, 10)
		result.PercentileHigh, _ = stats.Percentile(prices, 90)
		for _, price := range prices {
			r := int(math.Ceil(price/5) * 5)
			result.CountByPriceRange[r]++
		}
	}
	return result
}

func (offers VastAiOffers) gpuInfo() *GpuInfo {
	if len(offers) == 0 {
		return nil
	}

	vramVals := []float64{}
	dlperfVals := []float64{}
	tflopsVals := []float64{}
	for _, offer := range offers {
		vramVals = append(vramVals, offer.Vram)
		dlperfVals = append(dlperfVals, offer.DlperfPerGpuChunk)
		tflopsVals = append(tflopsVals, offer.TflopsPerGpu)
	}

	vram, _ := stats.Max(vramVals)
	dlperf, err := stats.Percentile(dlperfVals, 90)
	if err != nil {
		dlperf, _ = stats.Max(dlperfVals)
	}
	tflops, err := stats.Percentile(tflopsVals, 90)
	if err != nil {
		tflops, _ = stats.Max(tflopsVals)
	}

	return &GpuInfo{
		Vram:   vram,
		Dlperf: dlperf,
		Tflops: tflops,
	}
}

func (offers VastAiOffers) stats2(perDlPerf bool) OfferStats2 {
	return OfferStats2{
		Verified:   offers.filterVerified().stats(perDlPerf),
		Unverified: offers.filterUnverified().stats(perDlPerf),
		All:        offers.stats(perDlPerf),
	}
}

func (offers VastAiOffers) stats3(perDlPerf bool) OfferStats3 {
	return OfferStats3{
		Rented:    offers.filterRented().stats2(perDlPerf),
		Available: offers.filterAvailable().stats2(perDlPerf),
		All:       offers.stats2(perDlPerf),
	}
}

// Custom Marshaler to avoid "unsupported value: NaN"
func (t OfferStats) MarshalJSON() ([]byte, error) {
	type OfferStatsJson struct {
		Count          int      `json:"count"`
		Median         *float64 `json:"price_median,omitempty"`
		PercentileLow  *float64 `json:"price_10th_percentile,omitempty"`
		PercentileHigh *float64 `json:"price_90th_percentile,omitempty"`
	}
	u := OfferStatsJson{
		Count: t.Count,
	}
	if !math.IsNaN(t.Median) {
		v := t.Median / 100
		u.Median = &v
	}
	if !math.IsNaN(t.PercentileLow) && !math.IsNaN(t.PercentileHigh) {
		v1 := t.PercentileLow / 100
		u.PercentileLow = &v1
		v2 := t.PercentileHigh / 100
		u.PercentileHigh = &v2
	}
	j, err := json.Marshal(u)
	if err != nil {
		return nil, err
	}
	return []byte("[" + string(j) + "]"), nil
}
