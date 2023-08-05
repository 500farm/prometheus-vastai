package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/prometheus/common/log"
)

type MapLocation struct {
	Country      string  `json:"country"`
	Location     string  `json:"location"`
	Lat          float64 `json:"lat"`
	Long         float64 `json:"long"`
	Accuracy     float64 `json:"accuracy"` // in kilometers
	ISP          string  `json:"isp"`
	Organization string  `json:"organization"`
	Domain       string  `json:"domain"`
}

type HostMapItem struct {
	Gpus        string       `json:"gpus"`
	HostId      string       `json:"host_id"`
	MachineIds  string       `json:"machine_ids"`
	IpAddresses string       `json:"ip_addresses"`
	Tflops      float64      `json:"tflops"`
	TflopsSqrt  float64      `json:"tflops_sqrt"`
	Location    *MapLocation `json:"location"`
	Connection  string       `json:"connection,omitempty"`
}

type HostMapItems []HostMapItem

type HostMapResponse struct {
	Items HostMapItems `json:"items"`
}

func (cache *OfferCache) hostMapJson() []byte {
	hosts := cache.wholeMachineRawOffers.getHosts()

	mapItems := make(HostMapItems, 0, len(hosts))
	for _, host := range hosts {
		item := host.mapItem()
		if item != nil {
			mapItems = append(mapItems, *item)
		}
	}

	result, err := json.MarshalIndent(HostMapResponse{
		Items: mapItems,
	}, "", "    ")
	if err != nil {
		log.Errorln(err)
		return nil
	}
	return result
}

func (host *Host) mapItem() *HostMapItem {
	if host.Location == nil {
		return nil
	}
	loc := MapLocation(*host.Location)
	connection := ""
	if host.InetDown > 0 && host.InetUp > 0 {
		connection = fmt.Sprintf("↓ %.0f ↑ %.0f Mb/s", host.InetDown, host.InetUp)
	}
	r := HostMapItem{
		Gpus:        host.Gpus.String(),
		HostId:      strconv.Itoa(host.HostId),
		MachineIds:  intListToString(host.MachineIds),
		IpAddresses: strings.Join(host.IpAddresses, ", "),
		Tflops:      host.Tflops,
		Location:    &loc,
		TflopsSqrt:  math.Sqrt(host.Tflops),
		Connection:  connection,
	}
	return &r
}

func (m GpuCounts) String() string {
	type Gpu struct {
		name  string
		count int
	}
	gpus := make([]Gpu, 0, len(m))
	for name, count := range m {
		gpus = append(gpus, Gpu{name: name, count: count})
	}
	sort.Slice(gpus, func(i, j int) bool {
		c1 := gpus[i].count
		c2 := gpus[j].count
		if c1 == c2 {
			return gpus[i].name < gpus[j].name
		}
		return c1 > c2
	})
	strs := make([]string, 0, len(gpus))
	for _, gpu := range gpus {
		strs = append(strs, fmt.Sprintf("%dx %s", gpu.count, gpu.name))
	}
	return strings.Join(strs, ", ")
}

func intListToString(ints []int) string {
	r := ""
	for _, i := range ints {
		if r != "" {
			r += ", "
		}
		r += strconv.Itoa(i)
	}
	return r
}
