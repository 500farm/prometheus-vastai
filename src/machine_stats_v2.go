package main

import (
	"cmp"
	"encoding/json"
	"math"
	"slices"

	"github.com/montanaflynn/stats"
)

type CategorizedStats_GpuCountRange string

const (
	GpuCount_1_to_3 CategorizedStats_GpuCountRange = "1-3"
	GpuCount_4_to_7 CategorizedStats_GpuCountRange = "4-7"
	GpuCount_8_plus CategorizedStats_GpuCountRange = "8+"
)

type CategorizedStats_CategoryStats struct {
	Rented    MachineStats `json:"rented"`
	Available MachineStats `json:"available"`
	All       MachineStats `json:"all"`
}

type CategorizedStats_Category struct {
	GpuName string

	Verified      bool
	Datacenter    bool
	GpuCountRange CategorizedStats_GpuCountRange

	Stats CategorizedStats_CategoryStats
}

type categoryKey struct {
	gpuName       string
	verified      bool
	datacenter    bool
	gpuCountRange CategorizedStats_GpuCountRange
}

type categoryPrices struct {
	rented    []float64
	available []float64
}

func gpuCountRange(numGpus int) CategorizedStats_GpuCountRange {
	switch {
	case numGpus >= 8:
		return GpuCount_8_plus
	case numGpus >= 4:
		return GpuCount_4_to_7
	default:
		return GpuCount_1_to_3
	}
}

func compareBool(a, b bool) int {
	if a == b {
		return 0
	}
	if !a {
		return -1
	}
	return 1
}

func compareCategories(a, b CategorizedStats_Category) int {
	if c := cmp.Compare(a.GpuName, b.GpuName); c != 0 {
		return c
	}
	if c := compareBool(a.Datacenter, b.Datacenter); c != 0 {
		return c
	}
	if c := cmp.Compare(string(a.GpuCountRange), string(b.GpuCountRange)); c != 0 {
		return c
	}
	return compareBool(a.Verified, b.Verified)
}

func computeMachineStats(prices []float64) MachineStats {
	result := MachineStats{
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

func (machines VastAiMachineOffers) categorizedStats() []CategorizedStats_Category {
	buckets := make(map[categoryKey]*categoryPrices)

	for _, m := range machines {
		if m.GpuName == "" {
			continue
		}

		key := categoryKey{
			gpuName:       m.GpuName,
			verified:      m.Verified,
			datacenter:    m.Datacenter,
			gpuCountRange: gpuCountRange(m.NumGpus),
		}

		bucket, ok := buckets[key]
		if !ok {
			bucket = &categoryPrices{}
			buckets[key] = bucket
		}

		pricePerGpu := float64(m.PricePerGpu)

		for range m.NumGpusRented {
			bucket.rented = append(bucket.rented, pricePerGpu)
		}
		for range m.NumGpus - m.NumGpusRented {
			bucket.available = append(bucket.available, pricePerGpu)
		}
	}

	result := make([]CategorizedStats_Category, 0, len(buckets))
	for key, bucket := range buckets {
		allPrices := append(bucket.rented, bucket.available...)

		entry := CategorizedStats_Category{
			GpuName:       key.gpuName,
			Verified:      key.verified,
			Datacenter:    key.datacenter,
			GpuCountRange: key.gpuCountRange,
			Stats: CategorizedStats_CategoryStats{
				Rented:    computeMachineStats(bucket.rented),
				Available: computeMachineStats(bucket.available),
				All:       computeMachineStats(allPrices),
			},
		}

		result = append(result, entry)
	}

	slices.SortFunc(result, compareCategories)

	return result
}

type CategorizedStatsGroup struct {
	GpuName    string
	Categories []CategorizedStats_Category
	TotalCount int
}

func (machines VastAiMachineOffers) categorizedStatsByGpu() []CategorizedStatsGroup {
	entries := machines.categorizedStats()

	groups := make(map[string]*CategorizedStatsGroup)
	for _, e := range entries {
		g, ok := groups[e.GpuName]

		if !ok {
			g = &CategorizedStatsGroup{GpuName: e.GpuName}
			groups[e.GpuName] = g
		}

		g.Categories = append(g.Categories, e)
		g.TotalCount += e.Stats.All.Count
	}

	result := make([]CategorizedStatsGroup, 0, len(groups))
	for _, g := range groups {
		result = append(result, *g)
	}

	slices.SortFunc(result, func(a, b CategorizedStatsGroup) int {
		return cmp.Compare(b.TotalCount, a.TotalCount)
	})

	return result
}

func (e CategorizedStats_Category) MarshalJSON() ([]byte, error) {
	type jsonEntry struct {
		Datacenter    bool                           `json:"datacenter"`
		GpuCountRange CategorizedStats_GpuCountRange `json:"gpu_count_range"`
		Verified      bool                           `json:"verified"`
		Stats         CategorizedStats_CategoryStats `json:"stats"`
	}
	return json.Marshal(jsonEntry{
		Datacenter:    e.Datacenter,
		GpuCountRange: e.GpuCountRange,
		Verified:      e.Verified,
		Stats:         e.Stats,
	})
}
