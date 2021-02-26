package main

import (
	"fmt"
	"strconv"

	"github.com/montanaflynn/stats"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

type VastAiCollector struct {
	gpuName string
	hostId  int

	ondemand_price_median_dollars          *prometheus.GaugeVec
	ondemand_price_10th_percentile_dollars *prometheus.GaugeVec
	ondemand_price_90th_percentile_dollars *prometheus.GaugeVec
	pending_payout_dollars                 prometheus.Gauge

	machine_ondemand_price_per_gpu_dollars *prometheus.GaugeVec
	machine_is_verified                    *prometheus.GaugeVec
	machine_reliability                    *prometheus.GaugeVec
	machine_inet_bps                       *prometheus.GaugeVec
	machine_rentals_count                  *prometheus.GaugeVec

	instance_is_running                  *prometheus.GaugeVec
	instance_my_bid_per_gpu_dollars      *prometheus.GaugeVec
	instance_highest_bid_per_gpu_dollars *prometheus.GaugeVec
	instance_start_timestamp             *prometheus.GaugeVec
}

func newVastAiCollector(gpuName string) (*VastAiCollector, error) {
	namespace := "vastai"

	gpuLabelNames := []string{"gpu_name"}
	machineLabelNames := []string{"id", "hostname"}
	instanceLabelNames := []string{"id", "machine_id", "docker_image"}
	machineLabelNamesInet := append(append([]string{}, machineLabelNames...), "direction")
	machineLabelNamesRentals := append(append([]string{}, machineLabelNames...), "rental_type", "rental_status")

	return &VastAiCollector{
		gpuName: gpuName,
		hostId:  0,

		ondemand_price_median_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_median_dollars",
			Help:      fmt.Sprintf("Median on-demand price among verified %s GPUs with top DLPerf", gpuName),
		}, gpuLabelNames),
		ondemand_price_10th_percentile_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_10th_percentile_dollars",
			Help:      fmt.Sprintf("10th percentile of on-demand prices among verified %s GPUs with top DLPerf", gpuName),
		}, gpuLabelNames),
		ondemand_price_90th_percentile_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_90th_percentile_dollars",
			Help:      fmt.Sprintf("90th percentile of on-demand prices among verified %s GPUs with top DLPerf", gpuName),
		}, gpuLabelNames),
		pending_payout_dollars: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "pending_payout_dollars",
			Help:      "Pending payout (minus service fees)",
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
			Help:      "Count of current rentals (rental_type = 'ondemand'/'bid'/'default', rental_status = 'running'/'stopped')",
		}, machineLabelNamesRentals),

		instance_is_running: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "instance_is_running",
			Help:      "Is instance running (1) or stopped/outbid/initializing (0)",
		}, instanceLabelNames),
		instance_my_bid_per_gpu_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "instance_my_bid_per_gpu_dollars",
			Help:      "My bid on this instance per GPU/hour",
		}, instanceLabelNames),
		instance_highest_bid_per_gpu_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "instance_highest_bid_per_gpu_dollars",
			Help:      "Current highest bid on this instance per GPU/hour (=actual earnings if running)",
		}, instanceLabelNames),
		instance_start_timestamp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "instance_start_timestamp",
			Help:      "Unix timestamp when instance was started",
		}, instanceLabelNames),
	}, nil
}

func (e *VastAiCollector) Describe(ch chan<- *prometheus.Desc) {
	e.ondemand_price_median_dollars.Describe(ch)
	e.ondemand_price_10th_percentile_dollars.Describe(ch)
	e.ondemand_price_90th_percentile_dollars.Describe(ch)
	ch <- e.pending_payout_dollars.Desc()

	e.machine_ondemand_price_per_gpu_dollars.Describe(ch)
	e.machine_is_verified.Describe(ch)
	e.machine_reliability.Describe(ch)
	e.machine_inet_bps.Describe(ch)
	e.machine_rentals_count.Describe(ch)

	e.instance_is_running.Describe(ch)
	e.instance_my_bid_per_gpu_dollars.Describe(ch)
	e.instance_highest_bid_per_gpu_dollars.Describe(ch)
	e.instance_start_timestamp.Describe(ch)
}

func (e *VastAiCollector) Collect(ch chan<- prometheus.Metric) {
	e.ondemand_price_median_dollars.Collect(ch)
	e.ondemand_price_10th_percentile_dollars.Collect(ch)
	e.ondemand_price_90th_percentile_dollars.Collect(ch)
	ch <- e.pending_payout_dollars

	e.machine_ondemand_price_per_gpu_dollars.Collect(ch)
	e.machine_is_verified.Collect(ch)
	e.machine_reliability.Collect(ch)
	e.machine_inet_bps.Collect(ch)
	e.machine_rentals_count.Collect(ch)

	e.instance_is_running.Collect(ch)
	e.instance_my_bid_per_gpu_dollars.Collect(ch)
	e.instance_highest_bid_per_gpu_dollars.Collect(ch)
	e.instance_start_timestamp.Collect(ch)
}

func (e *VastAiCollector) Update(info *VastAiInfo) {
	if info.myMachines == nil {
		return
	}

	isMyMachineId := make(map[int]bool)
	for _, machine := range *info.myMachines {
		isMyMachineId[machine.Id] = true
	}

	if info.offers != nil {
		// Find the highest DLPerf for this kind of GPU. Then, use it ignore offers with DLPerf too low compared to the highest.
		topDlPerf := float64(0)
		for _, offer := range *info.offers {
			if offer.GpuName == e.gpuName && offer.GpuFrac == 1 {
				dlPerf := offer.DlPerf / float64(offer.NumGpus)
				if dlPerf > topDlPerf {
					topDlPerf = dlPerf
				}
			}
		}

		prices := []float64{}
		for _, offer := range *info.offers {
			if offer.GpuName == e.gpuName &&
				offer.GpuFrac == 1 &&
				offer.DlPerf/float64(offer.NumGpus) >= topDlPerf*0.80 &&
				!isMyMachineId[offer.MachineId] {
				prices = append(prices, offer.DphBase/float64(offer.NumGpus))
			}
		}

		if len(prices) > 0 {
			labels := prometheus.Labels{
				"gpu_name": e.gpuName,
			}

			median, _ := stats.Median(prices)
			e.ondemand_price_median_dollars.With(labels).Set(median)
			percentile20, _ := stats.Percentile(prices, 20)
			e.ondemand_price_10th_percentile_dollars.With(labels).Set(percentile20)
			percentile80, _ := stats.Percentile(prices, 80)
			e.ondemand_price_90th_percentile_dollars.With(labels).Set(percentile80)
		}
	}

	{
		for _, machine := range *info.myMachines {
			labels := prometheus.Labels{
				"id":       strconv.Itoa(machine.Id),
				"hostname": machine.Hostname,
			}

			e.machine_ondemand_price_per_gpu_dollars.With(labels).Set(machine.ListedGpuCost)
			e.machine_reliability.With(labels).Set(machine.Reliability)

			verified := float64(0)
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

			if info.myInstances != nil {
				defJobsRunning := 0
				defJobsStopped := 0

				for _, instance := range *info.myInstances {
					if instance.MachineId == machine.Id {
						if instance.ActualStatus == "running" {
							defJobsRunning++
						} else {
							defJobsStopped++
						}
					}
				}

				labels["rental_type"] = "default"
				labels["rental_status"] = "running"
				e.machine_rentals_count.With(labels).Set(float64(defJobsRunning))
				labels["rental_status"] = "stopped"
				e.machine_rentals_count.With(labels).Set(float64(defJobsStopped))
			}
		}
	}

	if info.myInstances != nil {
		for _, instance := range *info.myInstances {
			if isMyMachineId[instance.MachineId] {
				labels := prometheus.Labels{
					"id":           strconv.Itoa(instance.Id),
					"machine_id":   strconv.Itoa(instance.MachineId),
					"docker_image": instance.ImageUuid,
				}
				running := float64(0)
				if instance.ActualStatus == "running" {
					running = 1
				}
				e.instance_is_running.With(labels).Set(running)
				e.instance_my_bid_per_gpu_dollars.With(labels).Set(instance.DphBase)
				e.instance_highest_bid_per_gpu_dollars.With(labels).Set(instance.MinBid)
				e.instance_start_timestamp.With(labels).Set(instance.StartDate)
				e.hostId = instance.HostId
			}
		}
	}

	if e.hostId > 0 {
		pendingPayout, err := getPendingPayout(e.hostId)
		if err != nil {
			log.Errorln(err)
		} else {
			e.pending_payout_dollars.Set(pendingPayout)
		}
	}
}
