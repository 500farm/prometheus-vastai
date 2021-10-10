package main

import (
	"math"

	"github.com/montanaflynn/stats"
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

	return &VastAiCollectorAllGpus{
		ondemand_price_median_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_median_dollars",
			Help:      "Median on-demand price among verified GPUs",
		}, []string{"gpu_name"}),
		ondemand_price_10th_percentile_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_10th_percentile_dollars",
			Help:      "10th percentile of on-demand prices among verified GPUs",
		}, []string{"gpu_name"}),
		ondemand_price_90th_percentile_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_90th_percentile_dollars",
			Help:      "90th percentile of on-demand prices among verified GPUs",
		}, []string{"gpu_name"}),
		gpu_count: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "gpu_count",
			Help:      "Number of GPUs offered on site",
		}, []string{"gpu_name"}),
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

func (e *VastAiCollectorAllGpus) UpdateFrom(info *VastAiApiResults) {
	if info.offers == nil {
		return
	}

	// group offers by gpu name
	offersByGpu := make(map[string][]VastAiOffer)
	for _, offer := range *info.offers {
		name := offer.GpuName
		offersByGpu[name] = append(offersByGpu[name], offer)
	}

	// process offers
	for gpuName, offers := range offersByGpu {
		prices := []float64{}
		for _, offer := range offers {
			if offer.GpuFrac == 1 {
				pricePerGpu := offer.DphBase / float64(offer.NumGpus)
				for i := 0; i < offer.NumGpus; i++ {
					prices = append(prices, pricePerGpu)
				}
			}
		}

		labels := prometheus.Labels{
			"gpu_name": gpuName,
		}
		e.gpu_count.With(labels).Set(float64(len(prices)))
		median := math.NaN()
		percentileLow := math.NaN()
		percentileHigh := math.NaN()
		if len(prices) > 0 {
			median, _ = stats.Median(prices)
			percentileLow, _ = stats.Percentile(prices, 10)
			percentileHigh, _ = stats.Percentile(prices, 90)
		}
		if !math.IsNaN(median) {
			e.ondemand_price_median_dollars.With(labels).Set(median)
		} else {
			e.ondemand_price_median_dollars.Delete(labels)
		}
		if !math.IsNaN(percentileLow) && !math.IsNaN(percentileHigh) {
			e.ondemand_price_10th_percentile_dollars.With(labels).Set(percentileLow)
			e.ondemand_price_90th_percentile_dollars.With(labels).Set(percentileHigh)
		} else {
			e.ondemand_price_10th_percentile_dollars.Delete(labels)
			e.ondemand_price_90th_percentile_dollars.Delete(labels)
		}
	}
}
