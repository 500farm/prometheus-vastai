package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

type VastAiCollector struct {
	apiKey string

	ondemand_price_dollars prometheus.Histogram
}

func newVastAiCollector(apiKey string) (*VastAiCollector, error) {
	namespace := "vastai"

	return &VastAiCollector{
		apiKey: apiKey,

		ondemand_price_dollars: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "ondemand_price_dollars",
			Help:      "Distribution of on-demand prices",
			Buckets:   []float64{},
		}),
	}, nil
}

func (e *VastAiCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.ondemand_price_dollars.Desc()
}

func (e *VastAiCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- e.ondemand_price_dollars
}

func (e *VastAiCollector) Update(info *VastAiInfo) {
}
