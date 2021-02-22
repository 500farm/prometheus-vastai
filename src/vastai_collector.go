package main

import (
	"fmt"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

type VastAiCollector struct {
	gpuName   string
	minDlPerf float64

	average_ondemand_price_dollars prometheus.Gauge

	machine_ondemand_price_per_gpu_dollars *prometheus.GaugeVec
	machine_is_verified                    *prometheus.GaugeVec
	machine_reliability                    *prometheus.GaugeVec
	machine_inet_bps                       *prometheus.GaugeVec
	machine_rentals_count                  *prometheus.GaugeVec

	instance_is_running            *prometheus.GaugeVec
	instance_price_per_gpu_dollars *prometheus.GaugeVec
	instance_start_timestamp       *prometheus.GaugeVec
}

func newVastAiCollector(gpuName string, minDlPerf float64) (*VastAiCollector, error) {
	namespace := "vastai"

	machineLabelNames := []string{"id", "hostname"}
	instanceLabelNames := []string{"id", "machine_id", "docker_image"}
	machineLabelNamesInet := append(append([]string{}, machineLabelNames...), "direction")
	machineLabelNamesRentals := append(append([]string{}, machineLabelNames...), "rental_type", "rental_status")

	return &VastAiCollector{
		gpuName:   gpuName,
		minDlPerf: minDlPerf,

		average_ondemand_price_dollars: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "average_ondemand_price_dollars",
			Help:      fmt.Sprintf("Average on-demand price among %s GPUs with DLPerf >= %f", gpuName, minDlPerf),
		}),

		machine_ondemand_price_per_gpu_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_ondemand_price_per_gpu_dollars",
			Help:      "Machine on-demand price per GPU/hour",
		}, machineLabelNames),
		machine_is_verified: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_is_verified",
			Help:      "Is machine verified (1) or not (0)",
		}, machineLabelNames),
		machine_reliability: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_reliability",
			Help:      "Reliability indicator (0.0-1.0)",
		}, machineLabelNames),

		machine_inet_bps: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_inet_bps",
			Help:      "Measured internet speed, download or upload (direction = 'up' / 'down')",
		}, machineLabelNamesInet),
		machine_rentals_count: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_rentals_count",
			Help:      "Count of current rentals (rental_type = 'ondemand'/'bid', rental_status = 'running'/'stopped')",
		}, machineLabelNamesRentals),

		instance_is_running: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "instance_is_running",
			Help:      "Is instance running (1) or stopped/outbid/initializing (0)",
		}, instanceLabelNames),
		instance_price_per_gpu_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "instance_price_per_gpu_dollars",
			Help:      "Bid/on-demand price of this instance per GPU/hour",
		}, instanceLabelNames),
		instance_start_timestamp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "instance_start_timestamp",
			Help:      "Unix timestamp when instance was started",
		}, instanceLabelNames),
	}, nil
}

func (e *VastAiCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.average_ondemand_price_dollars.Desc()

	e.machine_ondemand_price_per_gpu_dollars.Describe(ch)
	e.machine_is_verified.Describe(ch)
	e.machine_reliability.Describe(ch)
	e.machine_inet_bps.Describe(ch)
	e.machine_rentals_count.Describe(ch)

	e.instance_is_running.Describe(ch)
	e.instance_price_per_gpu_dollars.Describe(ch)
	e.instance_start_timestamp.Describe(ch)
}

func (e *VastAiCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- e.average_ondemand_price_dollars

	e.machine_ondemand_price_per_gpu_dollars.Collect(ch)
	e.machine_is_verified.Collect(ch)
	e.machine_reliability.Collect(ch)
	e.machine_inet_bps.Collect(ch)
	e.machine_rentals_count.Collect(ch)

	e.instance_is_running.Collect(ch)
	e.instance_price_per_gpu_dollars.Collect(ch)
	e.instance_start_timestamp.Collect(ch)
}

func (e *VastAiCollector) Update(info *VastAiInfo) {
	if info.myMachines != nil && info.offers != nil {
		isMyMachineId := make(map[int]bool)
		for _, machine := range *info.myMachines {
			isMyMachineId[machine.Id] = true
		}

		sum := float64(0)
		count := 0
		for _, offer := range *info.offers {
			if offer.GpuName == e.gpuName &&
				offer.GpuFrac == 1 &&
				offer.DlPerf/float64(offer.NumGpus) >= e.minDlPerf &&
				!isMyMachineId[offer.MachineId] {
				sum += offer.DphBase / float64(offer.NumGpus)
				count++
			}
		}

		if count > 0 {
			e.average_ondemand_price_dollars.Set(sum / float64(count))
		}
	}

	if info.myMachines != nil {
		for _, machine := range *info.myMachines {
			labels := prometheus.Labels{
				"id":       strconv.Itoa(machine.Id),
				"hostname": machine.Hostname,
			}

			e.machine_ondemand_price_per_gpu_dollars.With(labels).Set(machine.ListedGpuCost)
			e.machine_reliability.With(labels).Set(machine.Reliability)

			var verified float64 = 0
			if machine.Verification == "verified" {
				verified = 1
			}
			e.machine_is_verified.With(labels).Set(verified)

			labels["direction"] = "up"
			e.machine_inet_bps.With(labels).Set(machine.InetUp * 1e6)
			labels["direction"] = "down"
			e.machine_inet_bps.With(labels).Set(machine.InetDown * 1e6)
			delete(labels, "direction")

			countOnDemandRunning := machine.CurrentRentalsRunningOnDemand
			countOnDemandStopped := machine.CurrentRentalsOnDemand - countOnDemandRunning
			countBidRunning := machine.CurrentRentalsRunning - countOnDemandRunning
			countBidStopped := machine.CurrentRentalsResident - machine.CurrentRentalsOnDemand - countBidRunning

			labels["rental_type"] = "ondemand"
			labels["rental_status"] = "running"
			e.machine_rentals_count.With(labels).Set(float64(countOnDemandRunning))
			labels["rental_status"] = "stopped"
			e.machine_rentals_count.With(labels).Set(float64(countOnDemandStopped))
			labels["rental_type"] = "bid"
			labels["rental_status"] = "running"
			e.machine_rentals_count.With(labels).Set(float64(countBidRunning))
			labels["rental_status"] = "stopped"
			e.machine_rentals_count.With(labels).Set(float64(countBidStopped))
		}
	}

	if info.myInstances != nil {
		for _, instance := range *info.myInstances {
			labels := prometheus.Labels{
				"id":           strconv.Itoa(instance.Id),
				"machine_id":   strconv.Itoa(instance.MachineId),
				"docker_image": instance.ImageUuid,
			}
			var running float64 = 0
			if instance.ActualStatus == "running" {
				running = 1
			}
			e.instance_is_running.With(labels).Set(running)
			e.instance_price_per_gpu_dollars.With(labels).Set(instance.DphBase)
			e.instance_start_timestamp.With(labels).Set(instance.StartDate)
		}
	}
}
