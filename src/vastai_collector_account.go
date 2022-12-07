package main

import (
	"errors"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

type instanceInfo struct {
	labels *prometheus.Labels
	keep   bool
}

type instanceInfoMap map[int]*instanceInfo

type VastAiAccountCollector struct {
	knownInstances instanceInfoMap
	lastPayouts    *PayoutInfo

	VastAiPriceStatsCollector
	pending_payout_dollars prometheus.Gauge
	paid_out_dollars       prometheus.Gauge
	last_payout_time       prometheus.Gauge

	machine_info                 *prometheus.GaugeVec
	machine_is_verified          *prometheus.GaugeVec
	machine_is_listed            *prometheus.GaugeVec
	machine_is_online            *prometheus.GaugeVec
	machine_reliability          *prometheus.GaugeVec
	machine_inet_bps             *prometheus.GaugeVec
	machine_per_gpu_teraflops    *prometheus.GaugeVec
	machine_per_gpu_dlperf_score *prometheus.GaugeVec

	machine_ondemand_price_per_gpu_dollars *prometheus.GaugeVec
	machine_gpu_count                      *prometheus.GaugeVec
	machine_rentals_count                  *prometheus.GaugeVec
	machine_used_gpu_count                 *prometheus.GaugeVec

	instance_info                    *prometheus.GaugeVec
	instance_is_running              *prometheus.GaugeVec
	instance_my_bid_per_gpu_dollars  *prometheus.GaugeVec
	instance_min_bid_per_gpu_dollars *prometheus.GaugeVec
	instance_start_timestamp         *prometheus.GaugeVec
	instance_gpu_count               *prometheus.GaugeVec
	instance_gpu_fraction            *prometheus.GaugeVec
}

func newVastAiAccountCollector() *VastAiAccountCollector {
	namespace := "vastai"

	instanceLabelNames := []string{"instance_id", "machine_id", "rental_type"}
	instanceInfoLabelNamess := append(append([]string{}, instanceLabelNames...), "docker_image", "gpu_name")

	return &VastAiAccountCollector{
		knownInstances: make(instanceInfoMap),
		lastPayouts:    readLastPayouts(),

		VastAiPriceStatsCollector: newVastAiPriceStatsCollector(),

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
		last_payout_time: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_payout_time",
			Help:      "Unix timestamp of last completed payout",
		}),

		machine_info: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_info",
			Help:      "Machine info",
		}, []string{"machine_id", "gpu_name", "hostname"}),

		machine_is_verified: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_is_verified",
			Help:      "Is machine verified (1) or not (0)",
		}, []string{"machine_id"}),
		machine_is_listed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_is_listed",
			Help:      "Is machine listed (1) or not (0)",
		}, []string{"machine_id"}),
		machine_is_online: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_is_online",
			Help:      "Is machine online (1) or not (0)",
		}, []string{"machine_id"}),
		machine_reliability: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_reliability",
			Help:      "Reliability indicator (0.0-1.0)",
		}, []string{"machine_id"}),
		machine_inet_bps: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_inet_bps",
			Help:      "Measured internet speed, download or upload (direction = 'up'/'down')",
		}, []string{"machine_id", "direction"}),
		machine_per_gpu_teraflops: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_per_gpu_teraflops",
			Help:      "Performance in TFLOPS per GPU",
		}, []string{"machine_id"}),
		machine_per_gpu_dlperf_score: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_per_gpu_dlperf_score",
			Help:      "DLPerf score per GPU",
		}, []string{"machine_id"}),

		machine_ondemand_price_per_gpu_dollars: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_ondemand_price_per_gpu_dollars",
			Help:      "Machine on-demand price per GPU/hour",
		}, []string{"machine_id"}),
		machine_gpu_count: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_gpu_count",
			Help:      "Number of GPUs",
		}, []string{"machine_id"}),
		machine_rentals_count: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_rentals_count",
			Help:      "Count of current rentals (rental_type = 'ondemand'/'bid'/'default'/'my', rental_status = 'running'/'stopped')",
		}, []string{"machine_id", "rental_type", "rental_status"}),
		machine_used_gpu_count: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "machine_used_gpu_count",
			Help:      "Number of GPUs running jobs (rental_type = 'ondemand'/'bid'/'default'/'my')",
		}, []string{"machine_id", "rental_type"}),

		instance_info: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "instance_info",
			Help:      "Instance info",
		}, instanceInfoLabelNamess),

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
	}
}

func (e *VastAiAccountCollector) Describe(ch chan<- *prometheus.Desc) {
	e.VastAiPriceStatsCollector.Describe(ch)

	ch <- e.pending_payout_dollars.Desc()
	ch <- e.paid_out_dollars.Desc()
	ch <- e.last_payout_time.Desc()

	e.machine_info.Describe(ch)
	e.machine_is_verified.Describe(ch)
	e.machine_is_listed.Describe(ch)
	e.machine_is_online.Describe(ch)
	e.machine_reliability.Describe(ch)
	e.machine_inet_bps.Describe(ch)
	e.machine_per_gpu_teraflops.Describe(ch)
	e.machine_per_gpu_dlperf_score.Describe(ch)

	e.machine_ondemand_price_per_gpu_dollars.Describe(ch)
	e.machine_gpu_count.Describe(ch)
	e.machine_rentals_count.Describe(ch)
	e.machine_used_gpu_count.Describe(ch)

	e.instance_info.Describe(ch)
	e.instance_is_running.Describe(ch)
	e.instance_my_bid_per_gpu_dollars.Describe(ch)
	e.instance_min_bid_per_gpu_dollars.Describe(ch)
	e.instance_start_timestamp.Describe(ch)
	e.instance_gpu_count.Describe(ch)
	e.instance_gpu_fraction.Describe(ch)
}

func (e *VastAiAccountCollector) Collect(ch chan<- prometheus.Metric) {
	e.VastAiPriceStatsCollector.Collect(ch)

	ch <- e.pending_payout_dollars
	ch <- e.paid_out_dollars
	ch <- e.last_payout_time

	e.machine_info.Collect(ch)
	e.machine_is_verified.Collect(ch)
	e.machine_is_listed.Collect(ch)
	e.machine_is_online.Collect(ch)
	e.machine_reliability.Collect(ch)
	e.machine_inet_bps.Collect(ch)
	e.machine_per_gpu_teraflops.Collect(ch)
	e.machine_per_gpu_dlperf_score.Collect(ch)

	e.machine_ondemand_price_per_gpu_dollars.Collect(ch)
	e.machine_gpu_count.Collect(ch)
	e.machine_rentals_count.Collect(ch)
	e.machine_used_gpu_count.Collect(ch)

	e.instance_info.Collect(ch)
	e.instance_is_running.Collect(ch)
	e.instance_my_bid_per_gpu_dollars.Collect(ch)
	e.instance_min_bid_per_gpu_dollars.Collect(ch)
	e.instance_start_timestamp.Collect(ch)
	e.instance_gpu_count.Collect(ch)
	e.instance_gpu_fraction.Collect(ch)
}

func (e *VastAiAccountCollector) UpdateFrom(info VastAiApiResults, offerCache *OfferCache) {
	e.UpdateMachinesAndInstances(info, offerCache)
	e.UpdatePayouts(info)
}

func (e *VastAiAccountCollector) UpdateMachinesAndInstances(info VastAiApiResults, offerCache *OfferCache) {
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
	e.VastAiPriceStatsCollector.UpdateFrom(offerCache, myGpus)

	// process machines
	// TODO handle disappeared machines
	for _, machine := range *info.myMachines {
		labels := prometheus.Labels{
			"machine_id": strconv.Itoa(machine.Id),
		}

		e.machine_info.
			MustCurryWith(labels).
			With(prometheus.Labels{"hostname": machine.Hostname, "gpu_name": machine.GpuName}).
			Set(1.0)
		e.machine_is_verified.With(labels).Set(boolToFloat(machine.Verification == "verified"))
		e.machine_is_listed.With(labels).Set(boolToFloat(machine.Listed))
		e.machine_is_online.With(labels).Set(boolToFloat(machine.Timeout == 0))
		e.machine_reliability.With(labels).Set(machine.Reliability)
		e.machine_per_gpu_teraflops.With(labels).Set(machine.TFlops / float64(machine.NumGpus))

		// inet up/down
		t := e.machine_inet_bps.MustCurryWith(labels)
		t.With(prometheus.Labels{"direction": "up"}).Set(machine.InetUp * 1e6)
		t.With(prometheus.Labels{"direction": "down"}).Set(machine.InetDown * 1e6)

		//
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

		// get dlperf from offer list
		dlPerf := 0.0
		for _, offer := range offerCache.machines {
			if offer.MachineId == machine.Id {
				dlPerf = offer.DlperfPerGpu
				break
			}
		}

		if dlPerf > 0 {
			e.machine_per_gpu_dlperf_score.With(labels).Set(dlPerf)
		}

		// count my/default jobs
		if info.myInstances != nil {
			defJobsRunning := 0
			defJobsStopped := 0
			myJobsRunning := 0
			myJobsStopped := 0
			defJobsUsedGpus := 0
			myJobsUsedGpus := 0

			for _, instance := range *info.myInstances {
				if instance.MachineId == machine.Id {
					if instance.isDefaultJob() {
						if instance.ActualStatus == "running" {
							defJobsRunning++
							defJobsUsedGpus += instance.NumGpus
						} else {
							defJobsStopped++
						}
					} else {
						if instance.ActualStatus == "running" {
							myJobsRunning++
							myJobsUsedGpus += instance.NumGpus
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

			numGpusOnDemand := strings.Count(machine.GpuOccupancy, "D")
			numGpusBid := strings.Count(machine.GpuOccupancy, "I")

			u := e.machine_used_gpu_count.MustCurryWith(labels)
			u.With(prometheus.Labels{"rental_type": "default"}).Set(float64(defJobsUsedGpus))
			u.With(prometheus.Labels{"rental_type": "my"}).Set(float64(myJobsUsedGpus))
			u.With(prometheus.Labels{"rental_type": "bid"}).Set(float64(numGpusBid))
			u.With(prometheus.Labels{"rental_type": "ondemand"}).Set(float64(numGpusOnDemand))
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
				if instance.isDefaultJob() {
					rentalType = "default"
				} else if instance.IsBid {
					rentalType = "bid"
				}
				labels := prometheus.Labels{
					"instance_id": strconv.Itoa(instance.Id),
					"machine_id":  strconv.Itoa(instance.MachineId),
					"rental_type": rentalType,
				}

				e.instance_info.
					MustCurryWith(labels).
					With(prometheus.Labels{
						"docker_image": instance.ImageUuid,
						"gpu_name":     instance.GpuName,
					}).
					Set(1.0)
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
				e.instance_info.Delete(*labels)
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
}

func (e *VastAiAccountCollector) UpdatePayouts(info VastAiApiResults) {
	if info.payouts == nil {
		return
	}

	// workaround: make sure pendingPayout grows strictly monotonically unless a payout happened
	if e.lastPayouts == nil ||
		info.payouts.PendingPayout > e.lastPayouts.PendingPayout ||
		info.payouts.PaidOut > e.lastPayouts.PaidOut {

		e.pending_payout_dollars.Set(info.payouts.PendingPayout)
		e.paid_out_dollars.Set(info.payouts.PaidOut)
		e.last_payout_time.Set(info.payouts.LastPayoutTime)

		// store lastPayouts and write them to the status file
		e.lastPayouts = info.payouts
		storeLastPayouts(info.payouts)
	}
}

func (e *VastAiAccountCollector) InitialUpdateFrom(info VastAiApiResults, offerCache *OfferCache) error {
	if info.myInstances == nil || info.myMachines == nil || info.payouts == nil {
		return errors.New("could not read all required data from Vast.ai")
	}

	if e.lastPayouts != nil {
		e.pending_payout_dollars.Set(e.lastPayouts.PendingPayout)
		e.paid_out_dollars.Set(e.lastPayouts.PaidOut)
		e.last_payout_time.Set(e.lastPayouts.LastPayoutTime)
	}

	e.UpdateFrom(info, offerCache)

	log.Infoln(len(offerCache.rawOffers), "raw offers,", len(*info.myMachines), "machines,", len(*info.myInstances), "instances, payouts:", *info.payouts)
	return nil
}
