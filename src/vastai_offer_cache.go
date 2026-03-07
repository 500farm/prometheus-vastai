package main

import (
	"cmp"
	json "github.com/goccy/go-json"
	"errors"
	"log"
	"slices"
	"time"
)

func timeStage(stage string) func() {
	if metrics != nil {
		return metrics.TimeProcessing(stage)
	}
	return func() {}
}

type OfferCache struct {
	rawOffers             VastAiRawOffers
	wholeMachineRawOffers VastAiRawOffers
	machines              VastAiOffers
	ts                    time.Time
	etag                  string
}

var offerCache OfferCache

func (cache *OfferCache) UpdateFrom(apiRes VastAiApiResults) {
	if apiRes.offers != nil {
		done := timeStage("validate_dedupe")
		cache.rawOffers = (*apiRes.offers).validate().dedupe()
		done()

		done = timeStage("collect_whole_machines")
		cache.wholeMachineRawOffers = cache.rawOffers.collectWholeMachines(cache.wholeMachineRawOffers)
		done()

		done = timeStage("decode")
		cache.machines = cache.wholeMachineRawOffers.decode()
		done()

		cache.ts = apiRes.ts
		cache.etag = computeETag(cache.ts)

		if metrics != nil {
			done = timeStage("get_hosts")
			hosts := cache.wholeMachineRawOffers.getHosts()
			done()
			metrics.UpdateCounts(len(cache.rawOffers), len(cache.wholeMachineRawOffers), len(hosts))
		}

		if geoCache != nil {
			geoCache.save()
		}
	}
}

func (cache *OfferCache) InitialUpdateFrom(apiRes VastAiApiResults) error {
	if apiRes.offers == nil {
		return errors.New("could not read offer data from Vast.ai")
	}
	cache.UpdateFrom(apiRes)
	return nil
}

type RawOffersResponse struct {
	Url       string           `json:"url"`
	Timestamp time.Time        `json:"timestamp"`
	Count     int              `json:"count"`
	Note      string           `json:"note,omitempty"`
	Offers    *VastAiRawOffers `json:"offers"`
}

func (cache *OfferCache) rawOffersJson(wholeMachines bool) JsonResponse {
	if wholeMachines {
		defer timeStage("json_machines")()
	} else {
		defer timeStage("json_offers")()
	}

	var offers *VastAiRawOffers
	url := "/offers"
	if wholeMachines {
		offers = &cache.wholeMachineRawOffers
		url = "/machines"
	} else {
		offers = &cache.rawOffers
	}
	note := ""
	if wholeMachines {
		note = "Sorted from newest to oldest."
	}

	result, err := json.MarshalIndent(RawOffersResponse{
		Url:       url,
		Timestamp: cache.ts.UTC(),
		Count:     len(*offers),
		Note:      note,
		Offers:    offers,
	}, "", "    ")

	if err != nil {
		log.Println("ERROR:", err)
		return JsonResponse{Content: nil, LastModified: cache.ts, ETag: cache.etag}
	}

	return JsonResponse{Content: result, LastModified: cache.ts, ETag: cache.etag}
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

func (cache *OfferCache) gpuStatsJson() JsonResponse {
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
		return JsonResponse{Content: []byte("{}"), LastModified: cache.ts, ETag: cache.etag}
	}
	return JsonResponse{Content: j, LastModified: cache.ts, ETag: cache.etag}
}
