package main

import (
	"errors"
	"strconv"

	"github.com/montanaflynn/stats"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

type instanceInfo struct {
	labels *prometheus.Labels
	keep   bool
}

type instanceInfoMap map[int]*instanceInfo

type VastAiCollector struct {
	knownInstances instanceInfoMap
	lastPayouts    *PayoutInfo

	ondemand_price_median_dollars          *prometheus.GaugeVec
	ondemand_price_10th_percentile_dollars *prometheus.GaugeVec
	ondemand_price_90th_percentile_dollars *prometheus.GaugeVec
	pending_payout_dollars                 prometheus.Gauge
	paid_out_dollars                       prometheus.Gauge

	machine_is_verified *prometheus.GaugeVec
	machine_is_listed   *prometheus.GaugeVec
	machine_is_online   *prometheus.GaugeVec
	machine_reliability *prometheus.GaugeVec
	machine_inet_bps    *prometheus.GaugeVec

	machine_ondemand_price_per_gpu_dollars *prometheus.GaugeVec
	machine_gpu_count                      *prometheus.GaugeVec
	machine_rentals_count                  *prometheus.GaugeVec

	instance_is_running              *prometheus.GaugeVec
	instance_my_bid_per_gpu_dollars  *prometheus.GaugeVec
	instance_min_bid_per_gpu_dollars *prometheus.GaugeVec
	instance_start_timestamp         *prometheus.GaugeVec
	instance_gpu_count               *prometheus.GaugeVec
	instance_gpu_fraction            *prometheus.GaugeVec
}

func newVastAiCollector() (*VastAiCollector, error) {
	namespace := "vastai"

	gpuLabelNames := []string{"gpu_name"}
	machineLabelNames := []string{"id", "hostname"}
	machineLabelNamesGpu := append(append([]string{}, machineLabelNames...), "gpu_name")
	machineLabelNamesInet := append(append([]string{}, machineLabelNames...), "direction")
	machineLabelNamesRentals := append(append([]string{}, machineLabelNamesGpu...), "rental_type", "rental_status")
	instanceLabelNames := []string{"id", "machine_id", "docker_image", "rental_type", "gpu_name"}

	return &VastAiCollector{
		knownInstances: make(instanceInfoMap),
		lastPayouts:    readLastPayouts(),

		ondemand_price_median_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_median_dollars",
			Help:      "Median on-demand price among verified GPUs with top DLPerf",
		}, gpuLabelNames),
		ondemand_price_10th_percentile_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_10th_percentile_dollars",
			Help:      "10th percentile of on-demand prices among verified GPUs with top DLPerf",
		}, gpuLabelNames),
		ondemand_price_90th_percentile_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ondemand_price_90th_percentile_dollars",
			Help:      "90th percentile of on-demand prices among verified GPUs with top DLPerf",
		}, gpuLabelNames),
		pending_payout_dollars: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "pending_payout_dollars",
			Help:      "Pending payout (minus service fees)",
		}),
		paid_out_dollars: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "paid_out_dollars",
			Help:      "All-time paid out amount (minus service fees)",
		}),

		machine_is_verified: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_is_verified",
			Help:      "Is machine verified (1) or not (0)",
		}, machineLabelNames),
		machine_is_listed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_is_listed",
			Help:      "Is machine listed (1) or not (0)",
		}, machineLabelNames),
		machine_is_online: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_is_online",
			Help:      "Is machine online (1) or not (0)",
		}, machineLabelNames),
		machine_reliability: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_reliability",
			Help:      "Reliability indicator (0.0-1.0)",
		}, machineLabelNames),
		machine_inet_bps: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_inet_bps",
			Help:      "Measured internet speed, download or upload (direction = 'up'/'down')",
		}, machineLabelNamesInet),

		machine_ondemand_price_per_gpu_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_ondemand_price_per_gpu_dollars",
			Help:      "Machine on-demand price per GPU/hour",
		}, machineLabelNamesGpu),
		machine_gpu_count: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_gpu_count",
			Help:      "Number of GPUs",
		}, machineLabelNamesGpu),
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
		instance_min_bid_per_gpu_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "instance_min_bid_per_gpu_dollars",
			Help:      "Min bid to outbid this instance per GPU/hour (makes sense if rental_type = 'default'/'bid')",
		}, instanceLabelNames),
		instance_start_timestamp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "instance_start_timestamp",
			Help:      "Unix timestamp when instance was started",
		}, instanceLabelNames),
		instance_gpu_count: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "instance_gpu_count",
			Help:      "Number of GPUs assigned to this instance",
		}, instanceLabelNames),
		instance_gpu_fraction: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "instance_gpu_fraction",
			Help:      "Number of GPUs assigned to this instance divided by total number of GPUs on the host",
		}, instanceLabelNames),
	}, nil
}

func (e *VastAiCollector) Describe(ch chan<- *prometheus.Desc) {
	e.ondemand_price_median_dollars.Describe(ch)
	e.ondemand_price_10th_percentile_dollars.Describe(ch)
	e.ondemand_price_90th_percentile_dollars.Describe(ch)
	ch <- e.pending_payout_dollars.Desc()
	ch <- e.paid_out_dollars.Desc()

	e.machine_is_verified.Describe(ch)
	e.machine_is_listed.Describe(ch)
	e.machine_is_online.Describe(ch)
	e.machine_reliability.Describe(ch)
	e.machine_inet_bps.Describe(ch)

	e.machine_ondemand_price_per_gpu_dollars.Describe(ch)
	e.machine_gpu_count.Describe(ch)
	e.machine_rentals_count.Describe(ch)

	e.instance_is_running.Describe(ch)
	e.instance_my_bid_per_gpu_dollars.Describe(ch)
	e.instance_min_bid_per_gpu_dollars.Describe(ch)
	e.instance_start_timestamp.Describe(ch)
	e.instance_gpu_count.Describe(ch)
	e.instance_gpu_fraction.Describe(ch)
}

func (e *VastAiCollector) Collect(ch chan<- prometheus.Metric) {
	e.ondemand_price_median_dollars.Collect(ch)
	e.ondemand_price_10th_percentile_dollars.Collect(ch)
	e.ondemand_price_90th_percentile_dollars.Collect(ch)
	ch <- e.pending_payout_dollars
	ch <- e.paid_out_dollars

	e.machine_is_verified.Collect(ch)
	e.machine_is_listed.Collect(ch)
	e.machine_is_online.Collect(ch)
	e.machine_reliability.Collect(ch)
	e.machine_inet_bps.Collect(ch)

	e.machine_ondemand_price_per_gpu_dollars.Collect(ch)
	e.machine_gpu_count.Collect(ch)
	e.machine_rentals_count.Collect(ch)

	e.instance_is_running.Collect(ch)
	e.instance_my_bid_per_gpu_dollars.Collect(ch)
	e.instance_min_bid_per_gpu_dollars.Collect(ch)
	e.instance_start_timestamp.Collect(ch)
	e.instance_gpu_count.Collect(ch)
	e.instance_gpu_fraction.Collect(ch)
}

func (e *VastAiCollector) UpdateFrom(info *VastAiApiResults) {
	if info.myMachines == nil {
		return
	}

	isMyMachineId := make(map[int]bool)
	numGpus := make(map[int]int)
	myGpus := []string{}
	for _, machine := range *info.myMachines {
		isMyMachineId[machine.Id] = true
		myGpus = append(myGpus, machine.GpuName)
		numGpus[machine.Id] = machine.NumGpus
	}

	// process offers
	if info.offers != nil {
		for _, gpuName := range myGpus {
			// Find the highest DLPerf for this kind of GPU. Then, use it ignore offers with DLPerf too low compared to the highest.
			topDlPerf := float64(0)
			for _, offer := range *info.offers {
				if offer.GpuName == gpuName && offer.GpuFrac == 1 {
					dlPerf := offer.DlPerf / float64(offer.NumGpus)
					if dlPerf > topDlPerf {
						topDlPerf = dlPerf
					}
				}
			}

			prices := []float64{}
			for _, offer := range *info.offers {
				if offer.GpuName == gpuName &&
					offer.GpuFrac == 1 &&
					offer.DlPerf/float64(offer.NumGpus) >= topDlPerf*0.80 &&
					!isMyMachineId[offer.MachineId] {
					prices = append(prices, offer.DphBase/float64(offer.NumGpus))
				}
			}

			if len(prices) > 0 {
				labels := prometheus.Labels{
					"gpu_name": gpuName,
				}

				median, _ := stats.Median(prices)
				e.ondemand_price_median_dollars.With(labels).Set(median)
				percentile20, _ := stats.Percentile(prices, 20)
				e.ondemand_price_10th_percentile_dollars.With(labels).Set(percentile20)
				percentile80, _ := stats.Percentile(prices, 80)
				e.ondemand_price_90th_percentile_dollars.With(labels).Set(percentile80)
			}
		}
	}

	// process machines
	{
		for _, machine := range *info.myMachines {
			labels := prometheus.Labels{
				"id":       strconv.Itoa(machine.Id),
				"hostname": machine.Hostname,
			}

			e.machine_is_verified.With(labels).Set(boolToFloat(machine.Verification == "verified"))
			e.machine_is_listed.With(labels).Set(boolToFloat(machine.Listed))
			e.machine_is_online.With(labels).Set(boolToFloat(machine.Timeout == 0))
			e.machine_reliability.With(labels).Set(machine.Reliability)

			// inet up/down
			t := e.machine_inet_bps.MustCurryWith(labels)
			t.With(prometheus.Labels{"direction": "up"}).Set(machine.InetUp * 1e6)
			t.With(prometheus.Labels{"direction": "down"}).Set(machine.InetDown * 1e6)

			//
			labels["gpu_name"] = machine.GpuName
			e.machine_ondemand_price_per_gpu_dollars.With(labels).Set(machine.ListedGpuCost)
			e.machine_gpu_count.With(labels).Set(float64(machine.NumGpus))

			// count different categories of rentals
			countOnDemandRunning := machine.CurrentRentalsRunningOnDemand
			countOnDemandStopped := machine.CurrentRentalsOnDemand - countOnDemandRunning
			countBidRunning := machine.CurrentRentalsRunning - countOnDemandRunning
			countBidStopped := machine.CurrentRentalsResident - machine.CurrentRentalsOnDemand - countBidRunning

			t = e.machine_rentals_count.MustCurryWith(labels)
			t.With(prometheus.Labels{"rental_type": "ondemand", "rental_status": "running"}).Set(float64(countOnDemandRunning))
			t.With(prometheus.Labels{"rental_type": "ondemand", "rental_status": "stopped"}).Set(float64(countOnDemandStopped))
			t.With(prometheus.Labels{"rental_type": "bid", "rental_status": "running"}).Set(float64(countBidRunning))
			t.With(prometheus.Labels{"rental_type": "bid", "rental_status": "stopped"}).Set(float64(countBidStopped))

			// count my/default jobs
			if info.myInstances != nil {
				defJobsRunning := 0
				defJobsStopped := 0
				myJobsRunning := 0
				myJobsStopped := 0

				for _, instance := range *info.myInstances {
					if instance.MachineId == machine.Id {
						if isDefaultJob(&instance) {
							if instance.ActualStatus == "running" {
								defJobsRunning++
							} else {
								defJobsStopped++
							}
						} else {
							if instance.ActualStatus == "running" {
								myJobsRunning++
							} else {
								myJobsStopped++
							}
						}
					}
				}

				t.With(prometheus.Labels{"rental_type": "default", "rental_status": "running"}).Set(float64(defJobsRunning))
				t.With(prometheus.Labels{"rental_type": "default", "rental_status": "stopped"}).Set(float64(defJobsStopped))
				t.With(prometheus.Labels{"rental_type": "my", "rental_status": "running"}).Set(float64(myJobsRunning))
				t.With(prometheus.Labels{"rental_type": "my", "rental_status": "stopped"}).Set(float64(myJobsStopped))
			}
		}
	}

	// process instances
	if info.myInstances != nil {
		for _, t := range e.knownInstances {
			t.keep = false
		}

		for _, instance := range *info.myInstances {
			if isMyMachineId[instance.MachineId] {
				rentalType := "ondemand"
				if isDefaultJob(&instance) {
					rentalType = "default"
				} else if instance.IsBid {
					rentalType = "bid"
				}
				labels := prometheus.Labels{
					"id":           strconv.Itoa(instance.Id),
					"machine_id":   strconv.Itoa(instance.MachineId),
					"docker_image": instance.ImageUuid,
					"rental_type":  rentalType,
					"gpu_name":     instance.GpuName,
				}

				e.instance_is_running.With(labels).Set(boolToFloat(instance.ActualStatus == "running"))
				e.instance_my_bid_per_gpu_dollars.With(labels).Set(instance.DphBase)
				e.instance_min_bid_per_gpu_dollars.With(labels).Set(instance.MinBid)
				e.instance_start_timestamp.With(labels).Set(instance.StartDate)
				e.instance_gpu_count.With(labels).Set(float64(instance.NumGpus))
				e.instance_gpu_fraction.With(labels).Set(float64(instance.NumGpus) / float64(numGpus[instance.MachineId]))

				e.knownInstances[instance.Id] = &instanceInfo{&labels, true}
			}
		}

		// remove metrics for disappeared instances
		for id, t := range e.knownInstances {
			if !t.keep {
				labels := t.labels
				e.instance_is_running.Delete(*labels)
				e.instance_my_bid_per_gpu_dollars.Delete(*labels)
				e.instance_min_bid_per_gpu_dollars.Delete(*labels)
				e.instance_start_timestamp.Delete(*labels)
				e.instance_gpu_count.Delete(*labels)
				e.instance_gpu_fraction.Delete(*labels)
				delete(e.knownInstances, id)
			}
		}
	}

	// process payouts
	if info.payouts != nil {
		// workaround: make sure pendingPayout grows strictly monotonically unless a payout happened
		if e.lastPayouts == nil ||
			info.payouts.PendingPayout > e.lastPayouts.PendingPayout ||
			info.payouts.PaidOut > e.lastPayouts.PaidOut {

			e.pending_payout_dollars.Set(info.payouts.PendingPayout)
			e.paid_out_dollars.Set(info.payouts.PaidOut)

			// store lastPayouts and write them to the status file
			e.lastPayouts = info.payouts
			storeLastPayouts(info.payouts)
		}
	}
}

func (e *VastAiCollector) InitialUpdateFrom(info *VastAiApiResults) error {
	if info.offers == nil || info.myInstances == nil || info.myMachines == nil || info.payouts == nil {
		return errors.New("Could not read all required data from Vast.ai")
	}

	if e.lastPayouts != nil {
		e.pending_payout_dollars.Set(e.lastPayouts.PendingPayout)
		e.paid_out_dollars.Set(e.lastPayouts.PaidOut)
	}

	e.UpdateFrom(info)

	log.Infoln(len(*info.offers), "offers,", len(*info.myMachines), "machines,", len(*info.myInstances), "instances, payouts:", *info.payouts)
	return nil
}
