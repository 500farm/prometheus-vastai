package main

import (
	"math"

	"github.com/prometheus/client_golang/prometheus"
)

type VastAiPriceStatsCollector struct {
	ondemand_price_median_dollars *prometheus.GaugeVec
	ondemand_price_p10_dollars    *prometheus.GaugeVec
	ondemand_price_p90_dollars    *prometheus.GaugeVec

	ondemand_price_per_100dlperf_median_dollars *prometheus.GaugeVec
	ondemand_price_per_100dlperf_p10_dollars    *prometheus.GaugeVec
	ondemand_price_per_100dlperf_p90_dollars    *prometheus.GaugeVec

	gpu_count *prometheus.GaugeVec
}

func newVastAiPriceStatsCollector() VastAiPriceStatsCollector {
	namespace := "vastai"

	labelNames := []string{"verified", "rented"}
	labelNamesWithGpu := []string{"gpu_name", "verified", "rented"}

	return VastAiPriceStatsCollector{
		ondemand_price_median_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_median_dollars",
			Help:      "Median on-demand price per GPU model",
		}, labelNamesWithGpu),
		ondemand_price_p10_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_10th_percentile_dollars",
			Help:      "10th percentile of on-demand prices per GPU model",
		}, labelNamesWithGpu),
		ondemand_price_p90_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_90th_percentile_dollars",
			Help:      "90th percentile of on-demand prices per GPU model",
		}, labelNamesWithGpu),

		ondemand_price_per_100dlperf_median_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_per_100dlperf_median_dollars",
			Help:      "Median on-demand price per 100 DLPerf points among all GPU models",
		}, labelNames),
		ondemand_price_per_100dlperf_p10_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_per_100dlperf_p10_dollars",
			Help:      "10th percentile of on-demand price per 100 DLPerf point among all GPU models",
		}, labelNames),
		ondemand_price_per_100dlperf_p90_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_per_100dlperf_p90_dollars",
			Help:      "90th percentile of on-demand prices per 100 DLPerf points among all GPU models",
		}, labelNames),

		gpu_count: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "gpu_count",
			Help:      "Number of GPUs offered on site",
		}, labelNamesWithGpu),
	}
}

func (e *VastAiPriceStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	e.ondemand_price_median_dollars.Describe(ch)
	e.ondemand_price_p10_dollars.Describe(ch)
	e.ondemand_price_p90_dollars.Describe(ch)

	e.ondemand_price_per_100dlperf_median_dollars.Describe(ch)
	e.ondemand_price_per_100dlperf_p10_dollars.Describe(ch)
	e.ondemand_price_per_100dlperf_p90_dollars.Describe(ch)

	e.gpu_count.Describe(ch)
}

func (e *VastAiPriceStatsCollector) Collect(ch chan<- prometheus.Metric) {
	e.ondemand_price_median_dollars.Collect(ch)
	e.ondemand_price_p10_dollars.Collect(ch)
	e.ondemand_price_p90_dollars.Collect(ch)

	e.ondemand_price_per_100dlperf_median_dollars.Collect(ch)
	e.ondemand_price_per_100dlperf_p10_dollars.Collect(ch)
	e.ondemand_price_per_100dlperf_p90_dollars.Collect(ch)

	e.gpu_count.Collect(ch)
}

func (e *VastAiPriceStatsCollector) UpdateFrom(offerCache *OfferCache, gpuNames []string) {
	groupedOffers := offerCache.machines.groupByGpu()

	updateMetrics := func(labels prometheus.Labels, stats OfferStats, needCount bool) {
		if needCount {
			e.gpu_count.With(labels).Set(float64(stats.Count))
		}
		if !math.IsNaN(stats.Median) {
			e.ondemand_price_median_dollars.With(labels).Set(stats.Median / 100)
		} else {
			e.ondemand_price_median_dollars.Delete(labels)
		}
		if !math.IsNaN(stats.PercentileLow) && !math.IsNaN(stats.PercentileHigh) {
			e.ondemand_price_p10_dollars.With(labels).Set(stats.PercentileLow / 100)
			e.ondemand_price_p90_dollars.With(labels).Set(stats.PercentileHigh / 100)
		} else {
			e.ondemand_price_p10_dollars.Delete(labels)
			e.ondemand_price_p90_dollars.Delete(labels)
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

	for gpuName, offers := range groupedOffers {
		if filterByGpuName && !isMyGpu[gpuName] {
			continue
		}
		stats := offers.stats3(false)
		updateMetrics(prometheus.Labels{"gpu_name": gpuName, "verified": "yes", "rented": "yes"}, stats.Rented.Verified, true)
		updateMetrics(prometheus.Labels{"gpu_name": gpuName, "verified": "no", "rented": "yes"}, stats.Rented.Unverified, true)
		updateMetrics(prometheus.Labels{"gpu_name": gpuName, "verified": "any", "rented": "yes"}, stats.Rented.All, false)
		updateMetrics(prometheus.Labels{"gpu_name": gpuName, "verified": "yes", "rented": "no"}, stats.Available.Verified, true)
		updateMetrics(prometheus.Labels{"gpu_name": gpuName, "verified": "no", "rented": "no"}, stats.Available.Unverified, true)
		updateMetrics(prometheus.Labels{"gpu_name": gpuName, "verified": "any", "rented": "no"}, stats.Available.All, false)
		updateMetrics(prometheus.Labels{"gpu_name": gpuName, "verified": "yes", "rented": "any"}, stats.All.Verified, false)
		updateMetrics(prometheus.Labels{"gpu_name": gpuName, "verified": "no", "rented": "any"}, stats.All.Unverified, false)
		updateMetrics(prometheus.Labels{"gpu_name": gpuName, "verified": "any", "rented": "any"}, stats.All.All, false)
	}

	// per-100-dlperf stats
	if !filterByGpuName {
		updateMetrics2 := func(labels prometheus.Labels, stats OfferStats) {
			if !math.IsNaN(stats.Median) {
				e.ondemand_price_per_100dlperf_median_dollars.With(labels).Set(stats.Median / 100)
			} else {
				e.ondemand_price_per_100dlperf_median_dollars.Delete(labels)
			}
			if !math.IsNaN(stats.PercentileLow) && !math.IsNaN(stats.PercentileHigh) {
				e.ondemand_price_per_100dlperf_p10_dollars.With(labels).Set(stats.PercentileLow / 100)
				e.ondemand_price_per_100dlperf_p90_dollars.With(labels).Set(stats.PercentileHigh / 100)
			} else {
				e.ondemand_price_per_100dlperf_p10_dollars.Delete(labels)
				e.ondemand_price_per_100dlperf_p90_dollars.Delete(labels)
			}
		}
		stats := offerCache.machines.stats3(true)
		updateMetrics2(prometheus.Labels{"verified": "yes", "rented": "yes"}, stats.Rented.Verified)
		updateMetrics2(prometheus.Labels{"verified": "no", "rented": "yes"}, stats.Rented.Unverified)
		updateMetrics2(prometheus.Labels{"verified": "any", "rented": "yes"}, stats.Rented.All)
		updateMetrics2(prometheus.Labels{"verified": "yes", "rented": "no"}, stats.Available.Verified)
		updateMetrics2(prometheus.Labels{"verified": "no", "rented": "no"}, stats.Available.Unverified)
		updateMetrics2(prometheus.Labels{"verified": "any", "rented": "no"}, stats.Available.All)
		updateMetrics2(prometheus.Labels{"verified": "yes", "rented": "any"}, stats.All.Verified)
		updateMetrics2(prometheus.Labels{"verified": "no", "rented": "any"}, stats.All.Unverified)
		updateMetrics2(prometheus.Labels{"verified": "any", "rented": "any"}, stats.All.All)
	}
}
