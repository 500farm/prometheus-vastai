package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mpvl/unique"
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

type MapItem struct {
	Gpus        map[string]int
	HostId      int
	MachineIds  []int
	IpAddresses []string
	Tflops      float64
	Location    *MapLocation
}

type MapItems []MapItem

func (offers VastAiRawOffers) prepareForMap() MapItems {
	result := make(map[string]MapItem, len(offers))

	for _, offer := range offers {
		location, ok := offer["location"].(*GeoLocation)
		if ok && (location.Lat != 0 || location.Long != 0) {

			gpus := make(map[string]int)
			gpus[offer.gpuName()] = offer.numGpus()

			hostId, _ := offer["host_id"].(float64)
			ipAddr, _ := offer["public_ipaddr"].(string)
			tflops, _ := offer["total_flops"].(float64)

			loc := MapLocation(*location)

			item := MapItem{
				Gpus:        gpus,
				HostId:      int(hostId),
				MachineIds:  []int{offer.machineId()},
				IpAddresses: []string{ipAddr},
				Tflops:      tflops,
				Location:    &loc,
			}

			hash := item.hash()
			result[hash] = result[hash].merge(item)
		}
	}

	result2 := make(MapItems, 0, len(result))
	for _, item := range result {
		result2 = append(result2, item)
	}

	sort.Slice(result2, func(i, j int) bool {
		return result2[i].Tflops > result2[j].Tflops
	})

	return result2
}

func (item MapItem) hash() string {
	return fmt.Sprintf("%d:%.3f:%.3f:%s", item.HostId, item.Location.Lat, item.Location.Long, item.Location.ISP)
}

func (item1 MapItem) merge(item2 MapItem) MapItem {
	gpus := make(map[string]int)
	if item1.Gpus != nil {
		for name, count := range item1.Gpus {
			gpus[name] += count
		}
	}
	for name, count := range item2.Gpus {
		gpus[name] += count
	}
	return MapItem{
		Gpus:        gpus,
		HostId:      item2.HostId,
		MachineIds:  append(item1.MachineIds, item2.MachineIds...),
		IpAddresses: append(item1.IpAddresses, item2.IpAddresses...),
		Tflops:      item1.Tflops + item2.Tflops,
		Location:    item2.Location,
	}
}

type MapItemJson struct {
	Gpus        string       `json:"gpus"`
	HostId      string       `json:"host_id"`
	MachineIds  string       `json:"machine_ids"`
	IpAddresses string       `json:"ip_addresses"`
	Tflops      float64      `json:"tflops"`
	Location    *MapLocation `json:"location"`
}

type MapItemsJson []MapItemJson

type MapResponse struct {
	Items MapItemsJson `json:"items"`
}

func (cache *OfferCache) mapJson() []byte {
	items := cache.wholeMachineRawOffers.prepareForMap()

	itemsForJson := make(MapItemsJson, 0, len(items))
	for _, item := range items {
		itemsForJson = append(itemsForJson, item.forJson())
	}

	result, err := json.MarshalIndent(MapResponse{
		Items: itemsForJson,
	}, "", "    ")
	if err != nil {
		log.Errorln(err)
		return nil
	}
	return result
}

func (item MapItem) forJson() MapItemJson {
	return MapItemJson{
		Location:    item.Location,
		Gpus:        gpusToString(item.Gpus),
		HostId:      strconv.Itoa(item.HostId),
		MachineIds:  intListToString(item.MachineIds),
		IpAddresses: listToString(item.IpAddresses),
		Tflops:      item.Tflops,
	}
}

func gpusToString(m map[string]int) string {
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
	sort.Ints(ints)
	unique.Ints(&ints)
	r := ""
	for _, i := range ints {
		if r != "" {
			r += ", "
		}
		r += strconv.Itoa(i)
	}
	return r
}

func listToString(strs []string) string {
	sort.Strings(strs)
	unique.Strings(&strs)
	return strings.Join(strs, ", ")
}
