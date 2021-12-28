package main

import (
	"fmt"
	"math"

	"github.com/prometheus/client_golang/prometheus"
)

type VastAiPriceStatsCollector struct {
	ondemand_price_median_dollars          *prometheus.GaugeVec
	ondemand_price_10th_percentile_dollars *prometheus.GaugeVec
	ondemand_price_90th_percentile_dollars *prometheus.GaugeVec
	gpu_count                              *prometheus.GaugeVec
	gpu_count_by_ondemand_price            *prometheus.GaugeVec
}

func newVastAiPriceStatsCollector() VastAiPriceStatsCollector {
	namespace := "vastai"

	labelNames := []string{"gpu_name", "verified", "rented"}
	labelNamesWithRange := []string{"gpu_name", "verified", "rented", "upper"}

	return VastAiPriceStatsCollector{
		ondemand_price_median_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_median_dollars",
			Help:      "Median on-demand price per GPU model",
		}, labelNames),
		ondemand_price_10th_percentile_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_10th_percentile_dollars",
			Help:      "10th percentile of on-demand prices per GPU model",
		}, labelNames),
		ondemand_price_90th_percentile_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_90th_percentile_dollars",
			Help:      "90th percentile of on-demand prices per GPU model",
		}, labelNames),
		gpu_count: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "gpu_count",
			Help:      "Number of GPUs offered on site",
		}, labelNames),
		gpu_count_by_ondemand_price: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "gpu_count_by_ondemand_price",
			Help:      "Number of GPUs offered on site, grouped by price ranges",
		}, labelNamesWithRange),
	}
}

func (e *VastAiPriceStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	e.ondemand_price_median_dollars.Describe(ch)
	e.ondemand_price_10th_percentile_dollars.Describe(ch)
	e.ondemand_price_90th_percentile_dollars.Describe(ch)
	e.gpu_count.Describe(ch)
	e.gpu_count_by_ondemand_price.Describe(ch)
}

func (e *VastAiPriceStatsCollector) Collect(ch chan<- prometheus.Metric) {
	e.ondemand_price_median_dollars.Collect(ch)
	e.ondemand_price_10th_percentile_dollars.Collect(ch)
	e.ondemand_price_90th_percentile_dollars.Collect(ch)
	e.gpu_count.Collect(ch)
	e.gpu_count_by_ondemand_price.Collect(ch)
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
			e.ondemand_price_10th_percentile_dollars.With(labels).Set(stats.PercentileLow / 100)
			e.ondemand_price_90th_percentile_dollars.With(labels).Set(stats.PercentileHigh / 100)
		} else {
			e.ondemand_price_10th_percentile_dollars.Delete(labels)
			e.ondemand_price_90th_percentile_dollars.Delete(labels)
		}
		// gpu counts by price ranges
		if needCount {
			minUpper := 1000000
			maxUpper := 0
			for upper := range stats.CountByPriceRange {
				if upper < minUpper {
					minUpper = upper
				}
				if upper > maxUpper {
					maxUpper = upper
				}
			}
			t := e.gpu_count_by_ondemand_price.MustCurryWith(labels)
			for upper := 5; upper < 200; upper += 5 {
				labels := prometheus.Labels{"upper": fmt.Sprintf("%.2f", float64(upper)/100)}
				c := stats.CountByPriceRange[upper]
				if upper >= minUpper && upper <= maxUpper {
					t.With(labels).Set(float64(c))
				} else {
					t.Delete(labels)
				}
			}
		}
	}

	filterByGpuName := gpuNames != nil
	isMyGpu := map[string]bool{}
	if filterByGpuName {
		for _, name := range gpuNames {
			isMyGpu[name] = true
		}
	}

	for gpuName, offers := range groupedOffers {
		if filterByGpuName && !isMyGpu[gpuName] {
			continue
		}
		stats := offers.stats3()
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
}
