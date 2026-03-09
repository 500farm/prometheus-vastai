package main

import (
	"maps"
	"math"

	"github.com/prometheus/client_golang/prometheus"
)

type VastAiPriceStatsCollectorV2 struct {
	v2_ondemand_price_median_dollars *prometheus.GaugeVec
	v2_ondemand_price_p10_dollars    *prometheus.GaugeVec
	v2_ondemand_price_p90_dollars    *prometheus.GaugeVec

	v2_gpu_count *prometheus.GaugeVec
}

func newVastAiPriceStatsCollectorV2() VastAiPriceStatsCollectorV2 {
	namespace := "vastai"

	labelNames := []string{"gpu_name", "verified", "rented", "datacenter", "gpu_count_range"}

	return VastAiPriceStatsCollectorV2{
		v2_ondemand_price_median_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "v2_ondemand_price_median_dollars",
			Help:      "Median on-demand price per GPU model (categorized)",
		}, labelNames),
		v2_ondemand_price_p10_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "v2_ondemand_price_10th_percentile_dollars",
			Help:      "10th percentile of on-demand prices per GPU model (categorized)",
		}, labelNames),
		v2_ondemand_price_p90_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "v2_ondemand_price_90th_percentile_dollars",
			Help:      "90th percentile of on-demand prices per GPU model (categorized)",
		}, labelNames),

		v2_gpu_count: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "v2_gpu_count",
			Help:      "Number of GPUs offered on site (categorized)",
		}, labelNames),
	}
}

func (e *VastAiPriceStatsCollectorV2) Describe(ch chan<- *prometheus.Desc) {
	e.v2_ondemand_price_median_dollars.Describe(ch)
	e.v2_ondemand_price_p10_dollars.Describe(ch)
	e.v2_ondemand_price_p90_dollars.Describe(ch)

	e.v2_gpu_count.Describe(ch)
}

func (e *VastAiPriceStatsCollectorV2) Collect(ch chan<- prometheus.Metric) {
	e.v2_ondemand_price_median_dollars.Collect(ch)
	e.v2_ondemand_price_p10_dollars.Collect(ch)
	e.v2_ondemand_price_p90_dollars.Collect(ch)

	e.v2_gpu_count.Collect(ch)
}

func (e *VastAiPriceStatsCollectorV2) UpdateFrom(offerCache *OfferCacheSnapshot, gpuNames []string) {
	updateMetrics := func(labels prometheus.Labels, s MachineStats) {
		e.v2_gpu_count.With(labels).Set(float64(s.Count))

		if !math.IsNaN(s.Median) {
			e.v2_ondemand_price_median_dollars.With(labels).Set(s.Median / 100)
		} else {
			e.v2_ondemand_price_median_dollars.Delete(labels)
		}

		if !math.IsNaN(s.PercentileLow) && !math.IsNaN(s.PercentileHigh) {
			e.v2_ondemand_price_p10_dollars.With(labels).Set(s.PercentileLow / 100)
			e.v2_ondemand_price_p90_dollars.With(labels).Set(s.PercentileHigh / 100)
		} else {
			e.v2_ondemand_price_p10_dollars.Delete(labels)
			e.v2_ondemand_price_p90_dollars.Delete(labels)
		}
	}

	filterByGpuName := gpuNames != nil
	isMyGpu := map[string]bool{}
	if filterByGpuName {
		for _, name := range gpuNames {
			isMyGpu[name] = true
		}
	}

	// always include these GPUs
	isMyGpu["RTX 3090"] = true
	isMyGpu["RTX 4090"] = true
	isMyGpu["RTX 5090"] = true

	for _, entry := range offerCache.machines.categorizedStats() {
		gpuName := entry.GpuName
		if filterByGpuName && !isMyGpu[gpuName] {
			continue
		}

		baseLabels := prometheus.Labels{
			"gpu_name":        gpuName,
			"verified":        boolToYesNo(entry.Verified),
			"datacenter":      boolToYesNo(entry.Datacenter),
			"gpu_count_range": string(entry.GpuCountRange),
		}

		for _, r := range []struct {
			label string
			stats MachineStats
		}{
			{"yes", entry.Stats.Rented},
			{"no", entry.Stats.Available},
			{"any", entry.Stats.All},
		} {
			labels := maps.Clone(baseLabels)
			labels["rented"] = r.label
			updateMetrics(labels, r.stats)
		}
	}
}

func boolToYesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
