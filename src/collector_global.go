package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

type VastAiGlobalCollector struct {
	VastAiPriceStatsCollectorV1
	VastAiPriceStatsCollectorV2

	gpu_vram_gigabytes *prometheus.GaugeVec
	gpu_teraflops      *prometheus.GaugeVec
	gpu_dlperf_score   *prometheus.GaugeVec
}

func newVastAiGlobalCollector() *VastAiGlobalCollector {
	namespace := "vastai"

	return &VastAiGlobalCollector{
		VastAiPriceStatsCollectorV1: newVastAiPriceStatsCollectorV1(),
		VastAiPriceStatsCollectorV2: newVastAiPriceStatsCollectorV2(),

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
	e.VastAiPriceStatsCollectorV1.Describe(ch)
	e.VastAiPriceStatsCollectorV2.Describe(ch)

	e.gpu_vram_gigabytes.Describe(ch)
	e.gpu_teraflops.Describe(ch)
	e.gpu_dlperf_score.Describe(ch)
}

func (e *VastAiGlobalCollector) Collect(ch chan<- prometheus.Metric) {
	e.VastAiPriceStatsCollectorV1.Collect(ch)
	e.VastAiPriceStatsCollectorV2.Collect(ch)

	e.gpu_vram_gigabytes.Collect(ch)
	e.gpu_teraflops.Collect(ch)
	e.gpu_dlperf_score.Collect(ch)
}

func (e *VastAiGlobalCollector) UpdateFrom(offerCache *OfferCacheSnapshot) {
	defer timeStage("metrics_global")()

	e.VastAiPriceStatsCollectorV1.UpdateFrom(offerCache, nil)
	e.VastAiPriceStatsCollectorV2.UpdateFrom(offerCache, nil)

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
