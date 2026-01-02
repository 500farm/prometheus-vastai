package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"log"

	"github.com/mpvl/unique"
)

type GpuCounts map[string]int

type Host struct {
	HostId      int          `json:"host_id"`
	MachineIds  []int        `json:"machine_ids"`
	IpAddresses []string     `json:"ip_addresses"`
	Gpus        GpuCounts    `json:"gpus"`
	Tflops      float64      `json:"tflops"`
	Location    *GeoLocation `json:"location,omitempty"`
	InetUp      float64      `json:"inet_up,omitempty"`
	InetDown    float64      `json:"inet_down,omitempty"`
}

type Hosts []Host

type HostsResponse struct {
	Url       string    `json:"url"`
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count"`
	Note      string    `json:"note,omitempty"`
	Hosts     *Hosts    `json:"hosts"`
}

func (cache *OfferCache) hostsJson() []byte {
	hosts := cache.wholeMachineRawOffers.getHosts()

	result, err := json.MarshalIndent(HostsResponse{
		Url:       "/hosts",
		Timestamp: cache.ts.UTC(),
		Count:     len(hosts),
		Note:      "Sorted by total TFLOPS (largest first). Hosts with multiple geo locations are split into multiple records.",
		Hosts:     &hosts,
	}, "", "    ")
	if err != nil {
		log.Println("ERROR:", err)
		return nil
	}
	return result
}

func (offers VastAiRawOffers) getHosts() Hosts {
	result := make(map[string]Host, len(offers))

	for _, offer := range offers {
		gpus := make(GpuCounts)
		gpus[offer.gpuName()] = offer.numGpus()

		hostId, _ := offer["host_id"].(float64)
		ipAddr, _ := offer["public_ipaddr"].(string)
		tflops, _ := offer["total_flops"].(float64)
		inetUp, _ := offer["inet_up"].(float64)
		inetDown, _ := offer["inet_down"].(float64)

		item := Host{
			HostId:      int(hostId),
			MachineIds:  []int{offer.machineId()},
			IpAddresses: []string{ipAddr},
			Gpus:        gpus,
			Tflops:      tflops,
			InetUp:      inetUp,
			InetDown:    inetDown,
		}

		location, ok := offer["location"].(*GeoLocation)
		if ok && (location.Lat != 0 || location.Long != 0) {
			item.Location = location
		}

		k := item.mergeKey()
		result[k] = result[k].merge(item)
	}

	result2 := make(Hosts, 0, len(result))
	for _, item := range result {
		sort.Ints(item.MachineIds)
		unique.Ints(&item.MachineIds)

		sort.Strings(item.IpAddresses)
		unique.Strings(&item.IpAddresses)

		result2 = append(result2, item)
	}

	sort.Slice(result2, func(i, j int) bool {
		return result2[i].Tflops > result2[j].Tflops
	})

	return result2
}

func (item Host) mergeKey() string {
	if item.Location != nil {
		return fmt.Sprintf("%d:%.3f:%.3f:%s", item.HostId, item.Location.Lat, item.Location.Long, item.Location.ISP)
	}
	return strconv.Itoa(item.HostId)
}

func (item1 Host) merge(item2 Host) Host {
	gpus := item1.Gpus
	if gpus == nil {
		gpus = make(GpuCounts)
	}
	for name, count := range item2.Gpus {
		gpus[name] += count
	}
	return Host{
		HostId:      item2.HostId,
		MachineIds:  append(item1.MachineIds, item2.MachineIds...),
		IpAddresses: append(item1.IpAddresses, item2.IpAddresses...),
		Gpus:        gpus,
		Tflops:      item1.Tflops + item2.Tflops,
		Location:    item2.Location,
		InetUp:      math.Max(item1.InetUp, item2.InetUp),
		InetDown:    math.Max(item1.InetDown, item2.InetDown),
	}
}
