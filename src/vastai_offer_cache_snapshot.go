package main

import (
	"cmp"
	"log"
	"slices"
	"time"

	json "github.com/goccy/go-json"
)

type OfferCacheSnapshot struct {
	rawOffers             VastAiRawOffers
	wholeMachineRawOffers VastAiRawOffers
	machines              VastAiOffers
	ts                    time.Time
}

func (cache *OfferCache) Snapshot() *OfferCacheSnapshot {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	return &OfferCacheSnapshot{
		rawOffers:             cache.rawOffers,
		wholeMachineRawOffers: cache.wholeMachineRawOffers,
		machines:              cache.machines,
		ts:                    cache.ts,
	}
}

func (cache *OfferCacheSnapshot) rawOffersJson() JsonResponse {
	defer timeStage("json_offers")()

	result, err := json.MarshalIndent(RawOffersResponse{
		Url:       "/offers",
		Timestamp: cache.ts.UTC(),
		Count:     len(cache.rawOffers),
		Offers:    &cache.rawOffers,
	}, "", "    ")

	if err != nil {
		log.Println("ERROR:", err)
		return JsonResponse{Content: nil, LastModified: cache.ts, ETag: cache.etag("/offers")}
	}

	return JsonResponse{Content: result, LastModified: cache.ts, ETag: cache.etag("/offers")}
}

func (cache *OfferCacheSnapshot) wholeMachinesJson() JsonResponse {
	defer timeStage("json_machines")()

	result, err := json.MarshalIndent(RawOffersResponse{
		Url:       "/machines",
		Timestamp: cache.ts.UTC(),
		Count:     len(cache.wholeMachineRawOffers),
		Note:      "Sorted from newest to oldest.",
		Offers:    &cache.wholeMachineRawOffers,
	}, "", "    ")

	if err != nil {
		log.Println("ERROR:", err)
		return JsonResponse{Content: nil, LastModified: cache.ts, ETag: cache.etag("/machines")}
	}

	return JsonResponse{Content: result, LastModified: cache.ts, ETag: cache.etag("/machines")}
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

func (cache *OfferCacheSnapshot) gpuStatsJson() JsonResponse {
	defer timeStage("json_gpu_stats")()

	groupedOffers := cache.machines.groupByGpu()
	result := GpuStatsResponse{
		Url:       "/gpu-stats",
		Timestamp: cache.ts.UTC(),
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

	j, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		log.Println("ERROR:", err)
		return JsonResponse{Content: nil, LastModified: cache.ts, ETag: cache.etag("/gpu-stats")}
	}
	return JsonResponse{Content: j, LastModified: cache.ts, ETag: cache.etag("/gpu-stats")}
}
