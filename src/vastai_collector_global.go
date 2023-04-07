package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

type VastAiGlobalCollector struct {
	VastAiPriceStatsCollector
	gpu_vram_gigabytes *prometheus.GaugeVec
	gpu_teraflops      *prometheus.GaugeVec
	gpu_dlperf_score   *prometheus.GaugeVec
}

func newVastAiGlobalCollector() *VastAiGlobalCollector {
	namespace := "vastai"

	return &VastAiGlobalCollector{
		VastAiPriceStatsCollector: newVastAiPriceStatsCollector(),

		gpu_vram_gigabytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "gpu_vram_gigabytes",
			Help:      "VRAM amount of the GPU model",
		}, []string{"gpu_name"}),
		gpu_teraflops: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "gpu_teraflops",
			Help:      "TFLOPS performance of the GPU model",
		}, []string{"gpu_name"}),
		gpu_dlperf_score: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "gpu_dlperf_score",
			Help:      "DLPerf score of the GPU model",
		}, []string{"gpu_name"}),
	}
}

func (e *VastAiGlobalCollector) Describe(ch chan<- *prometheus.Desc) {
	e.VastAiPriceStatsCollector.Describe(ch)

	e.gpu_vram_gigabytes.Describe(ch)
	e.gpu_teraflops.Describe(ch)
	e.gpu_dlperf_score.Describe(ch)
}

func (e *VastAiGlobalCollector) Collect(ch chan<- prometheus.Metric) {
	e.VastAiPriceStatsCollector.Collect(ch)

	e.gpu_vram_gigabytes.Collect(ch)
	e.gpu_teraflops.Collect(ch)
	e.gpu_dlperf_score.Collect(ch)
}

func (e *VastAiGlobalCollector) UpdateFrom(offerCache *OfferCache) {
	e.VastAiPriceStatsCollector.UpdateFrom(offerCache, nil)

	groupedOffers := offerCache.machines.groupByGpu()
	for gpuName, offers := range groupedOffers {
		info := offers.gpuInfo()
		if info != nil {
			labels := prometheus.Labels{"gpu_name": gpuName}
			e.gpu_vram_gigabytes.With(labels).Set(info.Vram)
			e.gpu_dlperf_score.With(labels).Set(info.Dlperf)
			e.gpu_teraflops.With(labels).Set(info.Tflops)
		}
	}
}
