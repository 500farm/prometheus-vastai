package main

import (
	"encoding/json"
	"fmt"
	"sort"
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

type MapItem struct {
	GpuName       string       `json:"gpu_name"`
	HostIds       string       `json:"host_ids"`
	MachineIds    string       `json:"machine_ids"`
	NumGpus       int          `json:"num_gpus"`
	NumGpusRented int          `json:"num_gpus_rented"`
	IpAddresses   string       `json:"ip_addresses"`
	Tflops        float64      `json:"tflops"`
	Location      *MapLocation `json:"location"`
}

type MapItems []*MapItem

type MapResponse struct {
	Items *MapItems `json:"items"`
}

func (cache *OfferCache) mapJson() []byte {
	items := cache.wholeMachineRawOffers.prepareForMap()
	result, err := json.MarshalIndent(MapResponse{
		Items: &items,
	}, "", "    ")
	if err != nil {
		log.Errorln(err)
		return nil
	}
	return result
}

func (offers VastAiRawOffers) prepareForMap() MapItems {
	result := make(map[string]*MapItem, len(offers))

	for _, offer := range offers {
		location, ok := offer["location"].(*GeoLocation)
		if ok && (location.Lat != 0 || location.Long != 0) {
			loc := location.forMap()
			hostId, _ := offer["host_id"].(float64)
			ipAddr, _ := offer["public_ipaddr"].(string)
			tflops, _ := offer["total_flops"].(float64)
			item := &MapItem{
				GpuName:       offer.gpuName(),
				HostIds:       fmt.Sprintf("%.0f", hostId),
				MachineIds:    fmt.Sprintf("%d", offer.machineId()),
				NumGpus:       offer.numGpus(),
				NumGpusRented: offer.numGpusRented(),
				IpAddresses:   ipAddr,
				Tflops:        tflops,
				Location:      &loc,
			}
			hash := item.hash()
			result[hash] = result[hash].merge(item)
		}
	}

	result2 := make(MapItems, 0, len(result))
	for _, item := range result {
		item.HostIds = makeUniqueList(item.HostIds)
		item.MachineIds = makeUniqueList(item.MachineIds)
		item.IpAddresses = makeUniqueList(item.IpAddresses)
		result2 = append(result2, item)
	}

	sort.Slice(result2, func(i, j int) bool {
		return result2[i].Tflops > result2[j].Tflops
	})

	return result2
}

func (loc *GeoLocation) forMap() MapLocation {
	return MapLocation{
		Country:      loc.Country,
		Location:     loc.Location,
		Lat:          loc.Lat,
		Long:         loc.Long,
		Accuracy:     loc.Accuracy,
		ISP:          loc.ISP,
		Organization: loc.Organization,
		Domain:       loc.Domain,
	}
}

func (item MapItem) hash() string {
	return fmt.Sprintf("%s:%.3f:%.3f:%s", item.GpuName, item.Location.Lat, item.Location.Long, item.Location.ISP)
}

func (item1 *MapItem) merge(item2 *MapItem) *MapItem {
	if item1 == nil {
		return item2
	}
	return &MapItem{
		GpuName:       item1.GpuName,
		HostIds:       mergeLists(item1.HostIds, item2.HostIds),
		MachineIds:    mergeLists(item1.MachineIds, item2.MachineIds),
		NumGpus:       item1.NumGpus + item2.NumGpus,
		NumGpusRented: item1.NumGpusRented + item2.NumGpusRented,
		IpAddresses:   mergeLists(item1.IpAddresses, item2.IpAddresses),
		Tflops:        item1.Tflops + item2.Tflops,
		Location:      item1.Location,
	}
}

func mergeLists(l1 string, l2 string) string {
	result := l1
	if l1 != "" {
		result += ","
	}
	result += l2
	return result
}

func makeUniqueList(s string) string {
	l := strings.Split(s, ",")
	return strings.Join(unique(l), ", ")
}

func unique(data []string) []string {
	m := make(map[string]bool)
	for _, s := range data {
		m[s] = true
	}
	result := make([]string, 0, len(data))
	for s := range m {
		result = append(result, s)
	}
	return result
}
