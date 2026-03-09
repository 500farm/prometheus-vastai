package main

import (
	"cmp"
	"encoding/json"
	"math"
	"slices"

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

func boolCmp(a, b bool) int {
	if a == b {
		return 0
	}
	if !a {
		return -1
	}
	return 1
}

func compareCategorizedStats(a, b CategorizedStatsEntry) int {
	if c := cmp.Compare(a.GpuName, b.GpuName); c != 0 {
		return c
	}
	if c := boolCmp(a.Datacenter, b.Datacenter); c != 0 {
		return c
	}
	if c := cmp.Compare(string(a.GpuCountRange), string(b.GpuCountRange)); c != 0 {
		return c
	}
	if c := boolCmp(a.Verified, b.Verified); c != 0 {
		return c
	}
	return boolCmp(a.Rented, b.Rented)
}

func (machines VastAiMachineOffers) categorizedStats() []CategorizedStatsEntry {
	// collect per-GPU prices into buckets for each category combination.
	buckets := make(map[categoryKey][]float64)

	for _, m := range machines {
		if m.GpuName == "" {
			continue
		}

		pricePerGpu := float64(m.PricePerGpu)

		// gpu_count_range is based on the machine's total GPU count
		countRange := gpuCountRange(m.NumGpus)

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
				gpuCountRange: countRange,
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

	slices.SortFunc(result, compareCategorizedStats)

	return result
}

type CategorizedStatsGroup struct {
	GpuName    string
	Categories []CategorizedStatsEntry
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
		g.TotalCount += e.Count
	}

	result := make([]CategorizedStatsGroup, 0, len(groups))
	for _, g := range groups {
		result = append(result, *g)
	}

	// sort by total GPU count descending
	slices.SortFunc(result, func(a, b CategorizedStatsGroup) int {
		return cmp.Compare(b.TotalCount, a.TotalCount)
	})

	return result
}

// custom MarshalJSON to avoid "unsupported value: NaN" and convert cents to dollars
func (e CategorizedStatsEntry) MarshalJSON() ([]byte, error) {
	type jsonEntry struct {
		Datacenter    bool                   `json:"datacenter"`
		GpuCountRange Category_GpuCountRange `json:"gpu_count_range"`
		Verified      bool                   `json:"verified"`
		Rented        bool                   `json:"rented"`
		Count         int                    `json:"count"`
		PriceMedian   *float64               `json:"price_median,omitempty"`
		Price10th     *float64               `json:"price_10th_percentile,omitempty"`
		Price90th     *float64               `json:"price_90th_percentile,omitempty"`
	}
	out := jsonEntry{
		Datacenter:    e.Datacenter,
		GpuCountRange: e.GpuCountRange,
		Verified:      e.Verified,
		Rented:        e.Rented,
		Count:         e.Count,
	}
	if !math.IsNaN(e.Median) {
		v := e.Median / 100
		out.PriceMedian = &v
	}
	if !math.IsNaN(e.PercentileLow) && !math.IsNaN(e.PercentileHigh) {
		v1 := e.PercentileLow / 100
		out.Price10th = &v1
		v2 := e.PercentileHigh / 100
		out.Price90th = &v2
	}
	return json.Marshal(out)
}
