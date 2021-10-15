package main

import (
	"math"

	"github.com/prometheus/client_golang/prometheus"
)

type VastAiCollectorAllGpus struct {
	ondemand_price_median_dollars          *prometheus.GaugeVec
	ondemand_price_10th_percentile_dollars *prometheus.GaugeVec
	ondemand_price_90th_percentile_dollars *prometheus.GaugeVec
	gpu_count                              *prometheus.GaugeVec
}

func newVastAiCollectorAllGpus() *VastAiCollectorAllGpus {
	namespace := "vastai"
	labelNames := []string{"gpu_name", "verified", "rented"}

	return &VastAiCollectorAllGpus{
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
	}
}

func (e *VastAiCollectorAllGpus) Describe(ch chan<- *prometheus.Desc) {
	e.ondemand_price_median_dollars.Describe(ch)
	e.ondemand_price_10th_percentile_dollars.Describe(ch)
	e.ondemand_price_90th_percentile_dollars.Describe(ch)
	e.gpu_count.Describe(ch)
}

func (e *VastAiCollectorAllGpus) Collect(ch chan<- prometheus.Metric) {
	e.ondemand_price_median_dollars.Collect(ch)
	e.ondemand_price_10th_percentile_dollars.Collect(ch)
	e.ondemand_price_90th_percentile_dollars.Collect(ch)
	e.gpu_count.Collect(ch)
}

func (e *VastAiCollectorAllGpus) UpdateFrom(offerCache *OfferCache) {
	groupedOffers := offerCache.machines.groupByGpu()

	updateMetrics := func(labels prometheus.Labels, stats OfferStats, needCount bool) {
		if needCount {
			e.gpu_count.With(labels).Set(float64(stats.Count))
		}
		if !math.IsNaN(stats.Median) {
			e.ondemand_price_median_dollars.With(labels).Set(stats.Median)
		} else {
			e.ondemand_price_median_dollars.Delete(labels)
		}
		if !math.IsNaN(stats.PercentileLow) && !math.IsNaN(stats.PercentileHigh) {
			e.ondemand_price_10th_percentile_dollars.With(labels).Set(stats.PercentileLow)
			e.ondemand_price_90th_percentile_dollars.With(labels).Set(stats.PercentileHigh)
		} else {
			e.ondemand_price_10th_percentile_dollars.Delete(labels)
			e.ondemand_price_90th_percentile_dollars.Delete(labels)
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
	}
}
