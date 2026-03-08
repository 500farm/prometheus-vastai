package main

import (
	"cmp"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"time"
)

type SerializedResponses map[string]*CachedResponse

type RawOffersResponse struct {
	Url       string           `json:"url"`
	Timestamp time.Time        `json:"timestamp"`
	Count     int              `json:"count"`
	Note      string           `json:"note,omitempty"`
	Offers    *VastAiRawOffers `json:"offers"`
}

type GpuStatsModel struct {
	Name  string      `json:"name"`
	Stats OfferStats3 `json:"stats"`
	Info  GpuInfo     `json:"info"`
}

type GpuStatsResponse struct {
	Url       string          `json:"url"`
	Timestamp time.Time       `json:"timestamp"`
	Note      string          `json:"note,omitempty"`
	Models    []GpuStatsModel `json:"models"`
}

func NewSerializedResponses(
	rawOffers VastAiRawOffers,
	wholeMachineRawOffers VastAiRawOffers,
	machines VastAiOffers,
	ts time.Time,
) SerializedResponses {
	responses := make(SerializedResponses, 5)

	responses["/offers"] = serializeOffers(rawOffers, ts)
	responses["/machines"] = serializeMachines(wholeMachineRawOffers, ts)
	responses["/hosts"] = serializeHosts(wholeMachineRawOffers, ts)
	responses["/gpu-stats"] = serializeGpuStats(machines, ts)
	responses["/host-map-data"] = serializeHostMap(wholeMachineRawOffers, ts)

	return responses
}

func makeEtag(ts time.Time, endpoint string) string {
	hash := sha256.Sum256([]byte(ts.Format(time.RFC3339Nano) + "|" + endpoint))
	return fmt.Sprintf(`"%x"`, hash[:8])
}

func buildCachedResponse(ts time.Time, endpoint string, jsonBytes []byte) *CachedResponse {
	if jsonBytes == nil {
		return &CachedResponse{
			ts:   ts,
			etag: makeEtag(ts, endpoint),
		}
	}

	resp := &CachedResponse{
		ts:      ts,
		etag:    makeEtag(ts, endpoint),
		raw:     jsonBytes,
		gzipped: gzip(jsonBytes),
	}

	log.Printf("INFO: Pre-serialized %s: %d bytes raw, %d bytes gzipped",
		endpoint, len(jsonBytes), len(resp.gzipped))

	return resp
}

func serializeOffers(rawOffers VastAiRawOffers, ts time.Time) *CachedResponse {
	defer timeStage("json_offers")()

	result, err := jsonMarshalV2(RawOffersResponse{
		Url:       "/offers",
		Timestamp: ts.UTC(),
		Count:     len(rawOffers),
		Offers:    &rawOffers,
	})

	if err != nil {
		log.Println("ERROR:", err)
		return buildCachedResponse(ts, "/offers", nil)
	}

	return buildCachedResponse(ts, "/offers", result)
}

func serializeMachines(wholeMachineRawOffers VastAiRawOffers, ts time.Time) *CachedResponse {
	defer timeStage("json_machines")()

	result, err := jsonMarshalV2(RawOffersResponse{
		Url:       "/machines",
		Timestamp: ts.UTC(),
		Count:     len(wholeMachineRawOffers),
		Note:      "Sorted from newest to oldest.",
		Offers:    &wholeMachineRawOffers,
	})

	if err != nil {
		log.Println("ERROR:", err)
		return buildCachedResponse(ts, "/machines", nil)
	}

	return buildCachedResponse(ts, "/machines", result)
}

func serializeHosts(wholeMachineRawOffers VastAiRawOffers, ts time.Time) *CachedResponse {
	defer timeStage("json_hosts")()

	hosts := wholeMachineRawOffers.getHosts()

	result, err := json.MarshalIndent(HostsResponse{
		Url:       "/hosts",
		Timestamp: ts.UTC(),
		Count:     len(hosts),
		Note:      "Sorted by total TFLOPS (largest first). Hosts with multiple geo locations are split into multiple records.",
		Hosts:     &hosts,
	}, "", "    ")
	if err != nil {
		log.Println("ERROR:", err)
		return buildCachedResponse(ts, "/hosts", nil)
	}

	return buildCachedResponse(ts, "/hosts", result)
}

func serializeGpuStats(machines VastAiOffers, ts time.Time) *CachedResponse {
	result := prepareGpuStats(machines, ts)

	defer timeStage("json_gpu_stats")()

	j, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		log.Println("ERROR:", err)
		return buildCachedResponse(ts, "/gpu-stats", nil)
	}
	return buildCachedResponse(ts, "/gpu-stats", j)
}

func prepareGpuStats(machines VastAiOffers, ts time.Time) GpuStatsResponse {
	defer timeStage("calc_gpu_stats")()

	groupedOffers := machines.groupByGpu()
	result := GpuStatsResponse{
		Url:       "/gpu-stats",
		Timestamp: ts.UTC(),
		Note:      "Sorted from most to least popular.",
	}

	for gpuName, offers := range groupedOffers {
		info := offers.gpuInfo()
		if info == nil {
			continue
		}
		result.Models = append(result.Models, GpuStatsModel{
			Name:  gpuName,
			Stats: offers.stats3(false),
			Info:  *info,
		})
	}

	slices.SortFunc(result.Models, func(a, b GpuStatsModel) int {
		return cmp.Compare(b.Stats.All.All.Count, a.Stats.All.All.Count)
	})

	return result
}

func serializeHostMap(wholeMachineRawOffers VastAiRawOffers, ts time.Time) *CachedResponse {
	mapItems := prepareHostMap(wholeMachineRawOffers)

	defer timeStage("json_host_map")()

	result, err := json.MarshalIndent(HostMapResponse{
		Items: mapItems,
	}, "", "    ")
	if err != nil {
		log.Println("ERROR:", err)
		return buildCachedResponse(ts, "/host-map-data", nil)
	}

	return buildCachedResponse(ts, "/host-map-data", result)
}

func prepareHostMap(wholeMachineRawOffers VastAiRawOffers) HostMapItems {
	defer timeStage("calc_host_map")()

	hosts := wholeMachineRawOffers.getHosts()

	mapItems := make(HostMapItems, 0, len(hosts))
	for _, host := range hosts {
		item := host.mapItem()
		if item != nil {
			mapItems = append(mapItems, *item)
		}
	}

	return mapItems
}
