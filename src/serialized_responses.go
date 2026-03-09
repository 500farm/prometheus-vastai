package main

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"time"

	pgzip "github.com/klauspost/pgzip"
)

type SerializedResponses map[string]*CachedResponse

var offersMarshaler = NewMarshaler()
var machinesMarshaler = NewMarshaler()

func NewSerializedResponses(
	offers VastAiOffers,
	machines VastAiMachineOffers,
	ts time.Time,
) SerializedResponses {
	responses := make(SerializedResponses, 6)

	responses["/offers"] = serializeOffers(&offers, ts)
	responses["/machines"] = serializeMachines(&machines, ts)
	responses["/hosts"] = serializeHosts(machines, ts)
	responses["/gpu-stats"] = serializeGpuStats(machines, ts)
	responses["/gpu-stats/v2"] = serializeGpuStatsV2(machines, ts)
	responses["/host-map-data"] = serializeHostMap(machines, ts)

	return responses
}

func makeEtag(ts time.Time, endpoint string) string {
	hash := sha256.Sum256([]byte(ts.Format(time.RFC3339Nano) + "|" + endpoint))
	return fmt.Sprintf(`"%x"`, hash[:8])
}

func gzip(data []byte) []byte {
	var buf bytes.Buffer
	w, _ := pgzip.NewWriterLevel(&buf, pgzip.DefaultCompression)
	_, _ = w.Write(data)
	_ = w.Close()
	return buf.Bytes()
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

type OffersResponse struct {
	Url       string                  `json:"url"`
	Timestamp time.Time               `json:"timestamp"`
	Count     int                     `json:"count"`
	Notes     []string                `json:"notes,omitempty"`
	Offers    *SerializableCollection `json:"offers"`
}

func serializeOffers(offers *VastAiOffers, ts time.Time) *CachedResponse {
	defer timeStage("json_offers")()

	o := *offers
	raw, gzipped, err := offersMarshaler.Marshal(OffersResponse{
		Url:       "/offers",
		Timestamp: ts.UTC(),
		Count:     len(o),
		Notes: []string{
			"Use Accept-Encoding: gzip for faster transfers.",
			"Use If-None-Match or If-Modified-Since to avoid redundant downloads.",
		},
		Offers: &SerializableCollection{
			marshaler: offersMarshaler,
			count:     len(o),
			get:       func(i int) any { return o[i].Raw },
		},
	})

	if err != nil {
		log.Println("ERROR:", err)
		return buildCachedResponse(ts, "/offers", nil)
	}

	log.Printf("INFO: Pre-serialized /offers: %d bytes raw, %d bytes gzipped",
		len(raw), len(gzipped))

	return &CachedResponse{ts: ts, etag: makeEtag(ts, "/offers"), raw: raw, gzipped: gzipped}
}

func serializeMachines(machines *VastAiMachineOffers, ts time.Time) *CachedResponse {
	defer timeStage("json_machines")()

	m := *machines
	raw, gzipped, err := machinesMarshaler.Marshal(OffersResponse{
		Url:       "/machines",
		Timestamp: ts.UTC(),
		Count:     len(m),
		Notes: []string{
			"Sorted from newest to oldest.",
			"Use Accept-Encoding: gzip for faster transfers.",
			"Use If-None-Match or If-Modified-Since to avoid redundant downloads.",
		},
		Offers: &SerializableCollection{
			marshaler: machinesMarshaler,
			count:     len(m),
			get:       func(i int) any { return m[i].asRaw() },
		},
	})

	if err != nil {
		log.Println("ERROR:", err)
		return buildCachedResponse(ts, "/machines", nil)
	}

	log.Printf("INFO: Pre-serialized /machines: %d bytes raw, %d bytes gzipped",
		len(raw), len(gzipped))

	return &CachedResponse{ts: ts, etag: makeEtag(ts, "/machines"), raw: raw, gzipped: gzipped}
}

func serializeHosts(machines VastAiMachineOffers, ts time.Time) *CachedResponse {
	defer timeStage("json_hosts")()

	hosts := machines.getHosts()

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

type GpuStatsModel struct {
	Name  string        `json:"name"`
	Stats MachineStats3 `json:"stats"`
	Info  GpuInfo       `json:"info"`
}

type GpuStatsResponse struct {
	Url       string          `json:"url"`
	Timestamp time.Time       `json:"timestamp"`
	Note      string          `json:"note,omitempty"`
	Models    []GpuStatsModel `json:"models"`
}

func serializeGpuStats(machines VastAiMachineOffers, ts time.Time) *CachedResponse {
	result := prepareGpuStats(machines, ts)

	defer timeStage("json_gpu_stats")()

	j, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		log.Println("ERROR:", err)
		return buildCachedResponse(ts, "/gpu-stats", nil)
	}
	return buildCachedResponse(ts, "/gpu-stats", j)
}

func prepareGpuStats(machines VastAiMachineOffers, ts time.Time) GpuStatsResponse {
	defer timeStage("calc_gpu_stats")()

	grouped := machines.groupByGpu()
	result := GpuStatsResponse{
		Url:       "/gpu-stats",
		Timestamp: ts.UTC(),
		Note:      "Sorted from most to least popular.",
	}

	for gpuName, machines := range grouped {
		info := machines.gpuInfo()
		if info == nil {
			continue
		}
		result.Models = append(result.Models, GpuStatsModel{
			Name:  gpuName,
			Stats: machines.stats3(false),
			Info:  *info,
		})
	}

	slices.SortFunc(result.Models, func(a, b GpuStatsModel) int {
		return cmp.Compare(b.Stats.All.All.Count, a.Stats.All.All.Count)
	})

	return result
}

type GpuStatsV2Model struct {
	Name       string                      `json:"name"`
	Count      int                         `json:"count"`
	Categories []CategorizedStats_Category `json:"categories"`
}

type GpuStatsV2Response struct {
	Url       string            `json:"url"`
	Timestamp time.Time         `json:"timestamp"`
	Notes     []string          `json:"notes,omitempty"`
	Models    []GpuStatsV2Model `json:"models"`
}

func serializeGpuStatsV2(machines VastAiMachineOffers, ts time.Time) *CachedResponse {
	result := prepareGpuStatsV2(machines, ts)

	defer timeStage("json_gpu_stats_v2")()

	j, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		log.Println("ERROR:", err)
		return buildCachedResponse(ts, "/gpu-stats/v2", nil)
	}
	return buildCachedResponse(ts, "/gpu-stats/v2", j)
}

func prepareGpuStatsV2(machines VastAiMachineOffers, ts time.Time) GpuStatsV2Response {
	defer timeStage("calc_gpu_stats_v2")()

	groups := machines.categorizedStatsByGpu()

	result := GpuStatsV2Response{
		Url:       "/gpu-stats/v2",
		Timestamp: ts.UTC(),
		Notes: []string{
			"Sorted from most to least popular.",
		},
	}

	for _, g := range groups {
		result.Models = append(result.Models, GpuStatsV2Model{
			Name:       g.GpuName,
			Count:      g.TotalCount,
			Categories: g.Categories,
		})
	}

	return result
}

func serializeHostMap(machines VastAiMachineOffers, ts time.Time) *CachedResponse {
	mapItems := prepareHostMap(machines)

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

func prepareHostMap(machines VastAiMachineOffers) HostMapItems {
	defer timeStage("calc_host_map")()

	hosts := machines.getHosts()

	mapItems := make(HostMapItems, 0, len(hosts))
	for _, host := range hosts {
		item := host.mapItem()
		if item != nil {
			mapItems = append(mapItems, *item)
		}
	}

	return mapItems
}
