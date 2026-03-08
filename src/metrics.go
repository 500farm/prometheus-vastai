package main

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var metrics *ExporterMetrics

type ExporterMetrics struct {
	offerCount   prometheus.Gauge
	machineCount prometheus.Gauge
	hostCount    prometheus.Gauge

	apiRequestDurationSeconds *prometheus.GaugeVec
	apiResponseSizeBytes      *prometheus.GaugeVec
	apiBytesRead              *prometheus.CounterVec

	serverResponseSizeBytes *prometheus.GaugeVec
	serverBytesWritten      *prometheus.CounterVec
	serverRequestsTotal     *prometheus.CounterVec
	serverNotModifiedTotal  *prometheus.CounterVec

	apiErrorsTotal *prometheus.CounterVec

	processDurationSeconds *prometheus.GaugeVec
	processSecondsTotal    *prometheus.CounterVec

	marshalerBufferCapBytes *prometheus.GaugeVec
}

func newExporterMetrics() *ExporterMetrics {
	namespace := "vastai_exporter"
	subsystemAPI := "api"
	subsystemServer := "server"
	subsystemProcess := "process"

	return &ExporterMetrics{
		offerCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "offer_count",
			Help:      "Number of offers currently tracked.",
		}),
		machineCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_count",
			Help:      "Number of whole machines currently tracked.",
		}),
		hostCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "host_count",
			Help:      "Number of unique hosts currently tracked.",
		}),

		apiRequestDurationSeconds: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemAPI,
			Name:      "request_duration_seconds",
			Help:      "Duration of the last API request in seconds.",
		}, []string{"endpoint"}),
		apiResponseSizeBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemAPI,
			Name:      "response_size_bytes",
			Help:      "Size of the last API response body in bytes.",
		}, []string{"endpoint"}),
		apiBytesRead: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemAPI,
			Name:      "bytes_read_total",
			Help:      "Total bytes read from API responses.",
		}, []string{"endpoint"}),

		serverResponseSizeBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemServer,
			Name:      "response_size_bytes",
			Help:      "Size of the last server response body in bytes.",
		}, []string{"endpoint"}),
		serverBytesWritten: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemServer,
			Name:      "bytes_written_total",
			Help:      "Total bytes written in server responses.",
		}, []string{"endpoint"}),
		serverRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemServer,
			Name:      "requests_total",
			Help:      "Total number of server requests handled (including 304).",
		}, []string{"endpoint"}),
		serverNotModifiedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemServer,
			Name:      "not_modified_total",
			Help:      "Total number of 304 Not Modified responses.",
		}, []string{"endpoint"}),

		apiErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemAPI,
			Name:      "errors_total",
			Help:      "Total number of API request errors by status code.",
		}, []string{"endpoint", "status"}),

		processDurationSeconds: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemProcess,
			Name:      "duration_seconds",
			Help:      "Duration of the last processing run in seconds.",
		}, []string{"stage"}),
		processSecondsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemProcess,
			Name:      "seconds_total",
			Help:      "Total CPU seconds spent in processing.",
		}, []string{"stage"}),

		marshalerBufferCapBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "marshaler_buffer_cap_bytes",
			Help:      "Total capacity of preallocated marshaler buffers in bytes.",
		}, []string{"endpoint"}),
	}
}

func (m *ExporterMetrics) Describe(ch chan<- *prometheus.Desc) {
	m.offerCount.Describe(ch)
	m.machineCount.Describe(ch)
	m.hostCount.Describe(ch)

	m.apiRequestDurationSeconds.Describe(ch)
	m.apiResponseSizeBytes.Describe(ch)
	m.apiBytesRead.Describe(ch)

	m.serverResponseSizeBytes.Describe(ch)
	m.serverBytesWritten.Describe(ch)
	m.serverRequestsTotal.Describe(ch)
	m.serverNotModifiedTotal.Describe(ch)

	m.apiErrorsTotal.Describe(ch)

	m.processDurationSeconds.Describe(ch)
	m.processSecondsTotal.Describe(ch)

	m.marshalerBufferCapBytes.Describe(ch)
}

func (m *ExporterMetrics) Collect(ch chan<- prometheus.Metric) {
	m.offerCount.Collect(ch)
	m.machineCount.Collect(ch)
	m.hostCount.Collect(ch)

	m.apiRequestDurationSeconds.Collect(ch)
	m.apiResponseSizeBytes.Collect(ch)
	m.apiBytesRead.Collect(ch)

	m.serverResponseSizeBytes.Collect(ch)
	m.serverBytesWritten.Collect(ch)
	m.serverRequestsTotal.Collect(ch)
	m.serverNotModifiedTotal.Collect(ch)

	m.apiErrorsTotal.Collect(ch)

	m.processDurationSeconds.Collect(ch)
	m.processSecondsTotal.Collect(ch)

	m.marshalerBufferCapBytes.WithLabelValues("offers").Set(float64(offersMarshaler.BufCap()))
	m.marshalerBufferCapBytes.WithLabelValues("machines").Set(float64(machinesMarshaler.BufCap()))
	m.marshalerBufferCapBytes.Collect(ch)
}

func (m *ExporterMetrics) ObserveAPIDuration(endpoint string, seconds float64) {
	m.apiRequestDurationSeconds.WithLabelValues(endpoint).Set(seconds)
}

func (m *ExporterMetrics) ObserveAPIResponseSize(endpoint string, bytes int) {
	m.apiResponseSizeBytes.WithLabelValues(endpoint).Set(float64(bytes))
	m.apiBytesRead.WithLabelValues(endpoint).Add(float64(bytes))
}

func (m *ExporterMetrics) ObserveServerResponse(endpoint string, bytes int) {
	m.serverResponseSizeBytes.WithLabelValues(endpoint).Set(float64(bytes))
	m.serverBytesWritten.WithLabelValues(endpoint).Add(float64(bytes))
	m.serverRequestsTotal.WithLabelValues(endpoint).Inc()
}

func (m *ExporterMetrics) ObserveServerNotModified(endpoint string) {
	m.serverNotModifiedTotal.WithLabelValues(endpoint).Inc()
	m.serverRequestsTotal.WithLabelValues(endpoint).Inc()
}

func (m *ExporterMetrics) ObserveAPIError(endpoint string, status string) {
	m.apiErrorsTotal.WithLabelValues(endpoint, status).Inc()
}

func (m *ExporterMetrics) UpdateCounts(offers, machines int) {
	m.offerCount.Set(float64(offers))
	m.machineCount.Set(float64(machines))
}

func (m *ExporterMetrics) ObserveProcessing(stage string, d time.Duration) {
	seconds := d.Seconds()
	m.processDurationSeconds.WithLabelValues(stage).Set(seconds)
	m.processSecondsTotal.WithLabelValues(stage).Add(seconds)
}

func (m *ExporterMetrics) TimeProcessing(stage string) func() {
	start := time.Now()
	return func() {
		m.ObserveProcessing(stage, time.Since(start))
	}
}