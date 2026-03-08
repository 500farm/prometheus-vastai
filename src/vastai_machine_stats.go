package main

import (
	"encoding/json"
	"math"

	"github.com/montanaflynn/stats"
)

type GroupedMachineOffers map[string]VastAiMachineOffers

type MachineStats struct {
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

type MachineStats2 struct {
	Verified   MachineStats `json:"verified"`
	Unverified MachineStats `json:"unverified"`
	All        MachineStats `json:"all"`
}

type MachineStats3 struct {
	Rented    MachineStats2 `json:"rented"`
	Available MachineStats2 `json:"available"`
	All       MachineStats2 `json:"all"`
}

func (machines VastAiMachineOffers) groupByGpu() GroupedMachineOffers {
	result := make(GroupedMachineOffers)
	for _, m := range machines {
		if m.GpuName != "" {
			result[m.GpuName] = append(result[m.GpuName], m)
		}
	}
	return result
}

func (machines VastAiMachineOffers) filter(fn func(VastAiMachineOffer) bool) VastAiMachineOffers {
	return machines.filter2(fn, nil)
}

func (machines VastAiMachineOffers) filter2(fn func(VastAiMachineOffer) bool, post func(VastAiMachineOffer) VastAiMachineOffer) VastAiMachineOffers {
	result := make(VastAiMachineOffers, 0, len(machines))
	for _, m := range machines {
		if fn(m) {
			if post != nil {
				result = append(result, post(m))
			} else {
				result = append(result, m)
			}
		}
	}
	return result
}

func (machines VastAiMachineOffers) filterVerified() VastAiMachineOffers {
	return machines.filter(func(m VastAiMachineOffer) bool { return m.Verified })
}

func (machines VastAiMachineOffers) filterUnverified() VastAiMachineOffers {
	return machines.filter(func(m VastAiMachineOffer) bool { return !m.Verified })
}

func (machines VastAiMachineOffers) filterRented() VastAiMachineOffers {
	return machines.filter2(
		func(m VastAiMachineOffer) bool { return m.NumGpusRented > 0 },
		func(m VastAiMachineOffer) VastAiMachineOffer {
			out := m
			out.NumGpus = m.NumGpusRented
			return out
		},
	)
}

func (machines VastAiMachineOffers) filterAvailable() VastAiMachineOffers {
	return machines.filter2(
		func(m VastAiMachineOffer) bool { return m.NumGpusRented < m.NumGpus },
		func(m VastAiMachineOffer) VastAiMachineOffer {
			out := m
			out.NumGpus -= m.NumGpusRented
			return out
		},
	)
}

func (machines VastAiMachineOffers) stats(perDlPerf bool) MachineStats {
	prices := []float64{}
	for _, m := range machines {
		pricePerGpu := float64(m.PricePerGpu)
		if perDlPerf {
			pricePerGpu = math.Floor(pricePerGpu * 100.0 / m.DlperfPerGpuChunk)
		}
		for range m.NumGpus {
			prices = append(prices, pricePerGpu)
		}
	}

	result := MachineStats{
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

func (machines VastAiMachineOffers) gpuInfo() *GpuInfo {
	if len(machines) == 0 {
		return nil
	}

	vramVals := []float64{}
	dlperfVals := []float64{}
	tflopsVals := []float64{}
	for _, m := range machines {
		vramVals = append(vramVals, m.Vram)
		dlperfVals = append(dlperfVals, m.DlperfPerGpuChunk)
		tflopsVals = append(tflopsVals, m.TflopsPerGpu)
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

func (machines VastAiMachineOffers) stats2(perDlPerf bool) MachineStats2 {
	return MachineStats2{
		Verified:   machines.filterVerified().stats(perDlPerf),
		Unverified: machines.filterUnverified().stats(perDlPerf),
		All:        machines.stats(perDlPerf),
	}
}

func (machines VastAiMachineOffers) stats3(perDlPerf bool) MachineStats3 {
	return MachineStats3{
		Rented:    machines.filterRented().stats2(perDlPerf),
		Available: machines.filterAvailable().stats2(perDlPerf),
		All:       machines.stats2(perDlPerf),
	}
}

// Custom Marshaler to avoid "unsupported value: NaN"
func (t MachineStats) MarshalJSON() ([]byte, error) {
	type MachineStatsJson struct {
		Count          int      `json:"count"`
		Median         *float64 `json:"price_median,omitempty"`
		PercentileLow  *float64 `json:"price_10th_percentile,omitempty"`
		PercentileHigh *float64 `json:"price_90th_percentile,omitempty"`
	}
	u := MachineStatsJson{
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
