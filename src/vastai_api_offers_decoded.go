package main

import (
	"cmp"
	"fmt"
	"log"
	"math"
	"slices"

	"github.com/hashicorp/go-set/v2"
)

type VastAiOffer struct {
	Raw        VastAiRawOffer
	Id         int
	MachineId  int
	HostId     int
	GpuName    string
	NumGpus    int
	DphBase    float64
	GpuFrac    float64
	Rentable   bool
	Verified   bool
	Datacenter bool
	StaticIp   bool
	VmsEnabled bool
	Dlperf     float64
	Tflops     float64
	Vram       float64
	InetUp     float64
	InetDown   float64
	IpAddr     string
	GpuIds     *set.Set[int]
	Location   *GeoLocation
}

type VastAiOffers []VastAiOffer

func (raw VastAiRawOffer) decode() (VastAiOffer, bool) {
	idVal, ok1 := raw["id"].(float64)
	machineId, ok2 := raw["machine_id"].(float64)
	hostId, ok3 := raw["host_id"].(float64)
	gpuName, ok4 := raw["gpu_name"].(string)
	numGpus, ok5 := raw["num_gpus"].(float64)
	dphBase, ok6 := raw["dph_base"].(float64)
	rentable, ok7 := raw["rentable"].(bool)
	gpuFrac, ok8 := raw["gpu_frac"].(float64)
	if !(ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && ok7 && ok8) {
		return VastAiOffer{}, false
	}

	verified, _ := raw["verified"].(bool)
	dlperf, _ := raw["dlperf"].(float64)
	tflops, _ := raw["total_flops"].(float64)
	vram, _ := raw["gpu_ram"].(float64)
	inetUp, _ := raw["inet_up"].(float64)
	inetDown, _ := raw["inet_down"].(float64)
	ipAddr, _ := raw["public_ipaddr"].(string)
	staticIp, _ := raw["static_ip"].(bool)
	vmsEnabled, _ := raw["vms_enabled"].(bool)

	datacenter := false
	if v, ok := raw["hosting_type"].(float64); ok {
		datacenter = int(v) > 0
	}

	var location *GeoLocation
	if loc, ok := raw["location"].(*GeoLocation); ok && (loc.Lat != 0 || loc.Long != 0) {
		location = loc
	}

	var gpuIds *set.Set[int]
	if items, ok := raw["gpu_ids"].([]any); ok {
		gpuIds = set.New[int](len(items))
		for _, item := range items {
			if v, ok := item.(float64); ok {
				gpuIds.Insert(int(v))
			}
		}
	}

	return VastAiOffer{
		Raw:        raw,
		Id:         int(idVal),
		MachineId:  int(machineId),
		HostId:     int(hostId),
		GpuName:    gpuName,
		NumGpus:    int(numGpus),
		DphBase:    dphBase,
		GpuFrac:    gpuFrac,
		Rentable:   rentable,
		Verified:   verified,
		Datacenter: datacenter,
		StaticIp:   staticIp,
		VmsEnabled: vmsEnabled,
		Dlperf:     dlperf,
		Tflops:     tflops,
		Vram:       math.Ceil(vram / 1024),
		InetUp:     inetUp,
		InetDown:   inetDown,
		IpAddr:     ipAddr,
		GpuIds:     gpuIds,
		Location:   location,
	}, true
}

func (rawOffers VastAiRawOffers) decode() VastAiOffers {
	result := make(VastAiOffers, 0, len(rawOffers))
	for _, raw := range rawOffers {
		if offer, ok := raw.decode(); ok {
			result = append(result, offer)
		} else {
			log.Println("WARN:", fmt.Sprintf("Offer is missing required fields: %v", raw))
		}
	}

	// sort by id for dedup
	slices.SortFunc(result, func(a, b VastAiOffer) int {
		return cmp.Compare(a.Id, b.Id)
	})

	// dedupe assuming sorted
	totalDups := 0
	n := 0
	for _, offer := range result {
		if offer.Id == 0 || (n > 0 && offer.Id == result[n-1].Id) {
			totalDups++
			continue
		}
		result[n] = offer
		n++
	}
	result = result[:n]
	if totalDups > 0 {
		log.Println("WARN:", fmt.Sprintf("Removed %d duplicate offers", totalDups))
	}

	// final sort: machine_id desc, id asc
	slices.SortFunc(result, func(a, b VastAiOffer) int {
		if c := cmp.Compare(b.MachineId, a.MachineId); c != 0 {
			return c
		}
		return cmp.Compare(a.Id, b.Id)
	})

	return result
}

func (offers VastAiOffers) groupByMachineId(fn func(machineId int, group VastAiOffers)) {
	for i := 0; i < len(offers); {
		machineId := offers[i].MachineId
		j := i + 1
		for j < len(offers) && offers[j].MachineId == machineId {
			j++
		}
		fn(machineId, offers[i:j])
		i = j
	}
}
