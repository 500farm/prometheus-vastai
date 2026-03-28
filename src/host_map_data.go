package main

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
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
	Datacenter  bool         `json:"datacenter"`
	Location    *MapLocation `json:"location"`
	Connection  string       `json:"connection"`
	Note        string       `json:"note,omitempty"`
}

type HostMapItems []HostMapItem

type HostMapResponse struct {
	Notes []string     `json:"notes"`
	Items HostMapItems `json:"items"`
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
		TflopsSqrt:  math.Sqrt(host.Tflops),
		Datacenter:  host.Datacenter,
		Location:    &loc,
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
	slices.SortFunc(gpus, func(a, b Gpu) int {
		if c := cmp.Compare(b.count, a.count); c != 0 {
			return c
		}
		return cmp.Compare(a.name, b.name)
	})
	strs := make([]string, 0, len(gpus))
	for _, gpu := range gpus {
		strs = append(strs, fmt.Sprintf("%dx %s", gpu.count, gpu.name))
	}
	return strings.Join(strs, ", ")
}

func intListToString(ints []int) string {
	if len(ints) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(strconv.Itoa(ints[0]))
	for _, i := range ints[1:] {
		b.WriteString(", ")
		b.WriteString(strconv.Itoa(i))
	}
	return b.String()
}
