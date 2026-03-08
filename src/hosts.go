package main

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"time"
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

// getHosts builds host records from whole-machine data.
func (machines VastAiMachineOffers) getHosts() Hosts {
	result := make(map[string]Host, len(machines))

	for _, m := range machines {
		gpus := make(GpuCounts)
		gpus[m.GpuName] = m.NumGpus

		item := Host{
			HostId:      m.HostId,
			MachineIds:  []int{m.MachineId},
			IpAddresses: []string{m.IpAddr},
			Gpus:        gpus,
			Tflops:      m.Tflops,
			InetUp:      m.InetUp,
			InetDown:    m.InetDown,
			Location:    m.Location,
		}

		k := item.mergeKey()
		result[k] = result[k].merge(item)
	}

	result2 := make(Hosts, 0, len(result))
	for _, item := range result {
		slices.Sort(item.MachineIds)
		item.MachineIds = slices.Compact(item.MachineIds)

		slices.Sort(item.IpAddresses)
		item.IpAddresses = slices.Compact(item.IpAddresses)

		result2 = append(result2, item)
	}

	slices.SortFunc(result2, func(a, b Host) int {
		if c := cmp.Compare(b.Tflops, a.Tflops); c != 0 {
			return c
		}
		return cmp.Compare(a.MachineIds[0], b.MachineIds[0])
	})

	if metrics != nil {
		metrics.hostCount.Set(float64(len(result2)))
	}

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
		InetUp:      max(item1.InetUp, item2.InetUp),
		InetDown:    max(item1.InetDown, item2.InetDown),
	}
}