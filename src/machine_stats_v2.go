package main

import (
	"math"

	"github.com/montanaflynn/stats"
)

type Category_GpuCountRange string

const (
	GpuCount_1_to_3 Category_GpuCountRange = "1-3"
	GpuCount_4_to_7 Category_GpuCountRange = "4-7"
	GpuCount_8_plus Category_GpuCountRange = "8+"
)

type CategorizedStatsEntry struct {
	GpuName string

	Rented        bool
	Verified      bool
	Datacenter    bool
	GpuCountRange Category_GpuCountRange

	Count          int
	Median         float64
	PercentileLow  float64
	PercentileHigh float64
}

type categoryKey struct {
	gpuName       string
	rented        bool
	verified      bool
	datacenter    bool
	gpuCountRange Category_GpuCountRange
}

func gpuCountRange(numGpus int) Category_GpuCountRange {
	switch {
	case numGpus >= 8:
		return GpuCount_8_plus
	case numGpus >= 4:
		return GpuCount_4_to_7
	default:
		return GpuCount_1_to_3
	}
}

func (machines VastAiMachineOffers) categorizedStats() []CategorizedStatsEntry {
	// collect per-GPU prices into buckets for each category combination.
	buckets := make(map[categoryKey][]float64)

	for _, m := range machines {
		if m.GpuName == "" {
			continue
		}

		pricePerGpu := float64(m.PricePerGpu)

		// a machine contributes to both rented and available buckets proportionally to its rented/available GPU counts
		type portion struct {
			rented  bool
			numGpus int
		}
		portions := []portion{}
		if m.NumGpusRented > 0 {
			portions = append(portions, portion{rented: true, numGpus: m.NumGpusRented})
		}
		if m.NumGpus-m.NumGpusRented > 0 {
			portions = append(portions, portion{rented: false, numGpus: m.NumGpus - m.NumGpusRented})
		}

		for _, p := range portions {
			key := categoryKey{
				gpuName:       m.GpuName,
				rented:        p.rented,
				verified:      m.Verified,
				datacenter:    m.Datacenter,
				gpuCountRange: gpuCountRange(p.numGpus),
			}
			prices := buckets[key]
			for range p.numGpus {
				prices = append(prices, pricePerGpu)
			}
			buckets[key] = prices
		}
	}

	// convert buckets to result entries
	result := make([]CategorizedStatsEntry, 0, len(buckets))
	for key, prices := range buckets {
		entry := CategorizedStatsEntry{
			GpuName:        key.gpuName,
			Rented:         key.rented,
			Verified:       key.verified,
			Datacenter:     key.datacenter,
			GpuCountRange:  key.gpuCountRange,
			Count:          len(prices),
			Median:         math.NaN(),
			PercentileLow:  math.NaN(),
			PercentileHigh: math.NaN(),
		}
		if len(prices) > 0 {
			entry.Median, _ = stats.Median(prices)
			entry.PercentileLow, _ = stats.Percentile(prices, 10)
			entry.PercentileHigh, _ = stats.Percentile(prices, 90)
		}
		result = append(result, entry)
	}

	return result
}
