package main

import (
	"fmt"
	"math"

	"github.com/prometheus/client_golang/prometheus"
)

type VastAiGlobalCollector struct {
	ondemand_price_median_dollars          *prometheus.GaugeVec
	ondemand_price_10th_percentile_dollars *prometheus.GaugeVec
	ondemand_price_90th_percentile_dollars *prometheus.GaugeVec
	gpu_count                              *prometheus.GaugeVec
	gpu_by_ondemand_price_range_count      *prometheus.GaugeVec
	gpu_vram_gigabytes                     *prometheus.GaugeVec
	gpu_teraflops                          *prometheus.GaugeVec
	gpu_dlperf_score                       *prometheus.GaugeVec
	gpu_eth_hashrate_ghs                   *prometheus.GaugeVec
}

func newVastAiGlobalCollector() *VastAiGlobalCollector {
	namespace := "vastai"
	labelNames := []string{"gpu_name", "verified", "rented"}
	labelNamesWithRange := []string{"gpu_name", "verified", "rented", "upper"}
	labelNames2 := []string{"gpu_name"}

	return &VastAiGlobalCollector{
		ondemand_price_median_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_median_dollars",
			Help:      "Median on-demand price among same-type GPUs",
		}, labelNames),
		ondemand_price_10th_percentile_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_10th_percentile_dollars",
			Help:      "10th percentile of on-demand prices among same-type GPUs",
		}, labelNames),
		ondemand_price_90th_percentile_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_90th_percentile_dollars",
			Help:      "90th percentile of on-demand prices among same-type GPUs",
		}, labelNames),
		gpu_count: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "gpu_count",
			Help:      "Number of GPUs offered on site",
		}, labelNames),
		gpu_by_ondemand_price_range_count: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "gpu_by_ondemand_price_range_count",
			Help:      "Number of GPUs offered on site, grouped by price ranges",
		}, labelNamesWithRange),

		gpu_vram_gigabytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "gpu_vram_gigabytes",
			Help:      "VRAM amount of the GPU model",
		}, labelNames2),
		gpu_teraflops: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "gpu_teraflops",
			Help:      "TFLOPS performance of the GPU model",
		}, labelNames2),
		gpu_dlperf_score: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "gpu_dlperf_score",
			Help:      "DLPerf score of the GPU model",
		}, labelNames2),
		gpu_eth_hashrate_ghs: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "gpu_eth_hashrate_ghs",
			Help:      "Approximate ETH hash rate of the GPU model",
		}, labelNames2),
	}
}

func (e *VastAiGlobalCollector) Describe(ch chan<- *prometheus.Desc) {
	e.ondemand_price_median_dollars.Describe(ch)
	e.ondemand_price_10th_percentile_dollars.Describe(ch)
	e.ondemand_price_90th_percentile_dollars.Describe(ch)
	e.gpu_count.Describe(ch)
	e.gpu_by_ondemand_price_range_count.Describe(ch)
	e.gpu_vram_gigabytes.Describe(ch)
	e.gpu_teraflops.Describe(ch)
	e.gpu_dlperf_score.Describe(ch)
	e.gpu_eth_hashrate_ghs.Describe(ch)
}

func (e *VastAiGlobalCollector) Collect(ch chan<- prometheus.Metric) {
	e.ondemand_price_median_dollars.Collect(ch)
	e.ondemand_price_10th_percentile_dollars.Collect(ch)
	e.ondemand_price_90th_percentile_dollars.Collect(ch)
	e.gpu_count.Collect(ch)
	e.gpu_by_ondemand_price_range_count.Collect(ch)
	e.gpu_vram_gigabytes.Collect(ch)
	e.gpu_teraflops.Collect(ch)
	e.gpu_dlperf_score.Collect(ch)
	e.gpu_eth_hashrate_ghs.Collect(ch)
}

func (e *VastAiGlobalCollector) UpdateFrom(offerCache *OfferCache) {
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
			t := e.gpu_by_ondemand_price_range_count.MustCurryWith(labels)
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

	for gpuName, offers := range groupedOffers {
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

		info := offers.gpuInfo()
		if info != nil {
			labels := prometheus.Labels{"gpu_name": gpuName}
			e.gpu_vram_gigabytes.With(labels).Set(info.Vram)
			e.gpu_dlperf_score.With(labels).Set(info.Dlperf)
			e.gpu_teraflops.With(labels).Set(info.Tflops)
			if info.EthHashRate > 0 {
				e.gpu_eth_hashrate_ghs.With(labels).Set(info.EthHashRate)
			}
		}
	}
}
