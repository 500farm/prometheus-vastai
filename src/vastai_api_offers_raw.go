package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-set/v2"
	"github.com/prometheus/common/log"
)

type VastAiRawOffer map[string]interface{}
type VastAiRawOffers []VastAiRawOffer

func getRawOffersFromMaster(masterUrl string, result *VastAiApiResults) error {
	url := strings.TrimRight(masterUrl, "/") + "/offers"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf(`URL %s returned "%s"`, url, resp.Status)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var j RawOffersResponse
	err = json.Unmarshal(body, &j)
	if err != nil {
		return err
	}
	if j.Url != "/offers" {
		return fmt.Errorf("not a Vast.ai exporter URL: %s", masterUrl)
	}
	result.ts = j.Timestamp
	result.offers = j.Offers
	return nil
}

func getRawOffersFromApi(result *VastAiApiResults) error {
	var t struct {
		Offers VastAiRawOffers `json:"offers"`
	}
	result.ts = time.Now()
	if err := vastApiCall(&t, "bundles", url.Values{
		"q": {`{"external":{"eq":"false"},"type":"on-demand","disable_bundling":true}`},
	}, bundleTimeout); err != nil {
		return err
	}
	for _, offer := range t.Offers {
		// remove useless fields
		delete(offer, "external")
		delete(offer, "webpage")
		delete(offer, "logo")
		delete(offer, "pending_count")
		delete(offer, "inet_down_billed")
		delete(offer, "inet_up_billed")
		delete(offer, "storage_total_cost")
		delete(offer, "dph_total")
		delete(offer, "rented")
		delete(offer, "is_bid")
		// fix whitespace in public_ipaddr
		if ip, ok := offer["public_ipaddr"].(string); ok {
			offer["public_ipaddr"] = strings.TrimSpace(ip)
		}
		offer["verified"] = offer["verification"] == "verified"
	}
	result.offers = &t.Offers
	return nil
}

func (offers VastAiRawOffers) filter(filter func(VastAiRawOffer) bool) VastAiRawOffers {
	return offers.filter2(filter, nil)
}

func (offers VastAiRawOffers) filter2(filter func(VastAiRawOffer) bool, postProcess func(VastAiRawOffer) VastAiRawOffer) VastAiRawOffers {
	result := make(VastAiRawOffers, 0, len(offers))
	for _, offer := range offers {
		if filter(offer) {
			if postProcess != nil {
				result = append(result, postProcess(offer))
			} else {
				result = append(result, offer)
			}
		}
	}
	return result
}

func (offers VastAiRawOffers) validate() VastAiRawOffers {
	result := offers.filter(func(offer VastAiRawOffer) bool {
		// check if required fields are ok and have a correct type
		_, ok1 := offer["machine_id"].(float64)
		_, ok2 := offer["gpu_name"].(string)
		_, ok3 := offer["num_gpus"].(float64)
		_, ok4 := offer["dph_base"].(float64)
		_, ok5 := offer["rentable"].(bool)
		_, ok6 := offer["gpu_frac"].(float64)
		if ok1 && ok2 && ok3 && ok4 && ok5 && ok6 {
			return true
		}
		log.Warnln(fmt.Sprintf("Offer is missing required fields: %v", offer))
		return false
	})

	return result
}

func (offers VastAiRawOffers) groupByMachineId() map[int]VastAiRawOffers {
	grouped := make(map[int]VastAiRawOffers)
	for _, offer := range offers {
		machineId := offer.machineId()
		grouped[machineId] = append(grouped[machineId], offer)
	}
	return grouped
}

type Chunk struct {
	size     int
	frac     float64
	offer    VastAiRawOffer
	offerId  int
	rentable bool
	dlperf   float64
	gpuIds   set.Set[int]
}

type Chunk2 struct {
	Size     int   `json:"size"`
	OfferId  int   `json:"offerId"`
	Rentable bool  `json:"rentable"`
	GpuIds   []int `json:"gpu_ids"`
}

func (chunk Chunk) gpuIdsSorted() []int {
	gpuIds := chunk.gpuIds.Slice()
	slices.Sort(gpuIds)
	return gpuIds
}

func (offers VastAiRawOffers) collectWholeMachines(prevResult VastAiRawOffers) VastAiRawOffers {
	result := make(VastAiRawOffers, 0, len(prevResult))

	for machineId, offers := range offers.groupByMachineId() {
		// for each machine:

		// - collect array of chunks from smallest to largest
		chunks := make([]Chunk, 0, len(offers))
		for _, offer := range offers {
			chunks = append(chunks, Chunk{
				offer:    offer,
				offerId:  offer.id(),
				size:     offer.numGpus(),
				frac:     offer.gpuFrac(),
				rentable: offer.rentable(),
				dlperf:   offer.dlperf(),
				gpuIds:   *offer.gpuIds(),
			})
		}
		sort.Slice(chunks, func(i, j int) bool {
			return chunks[i].size*1e12+chunks[i].offerId < chunks[j].size*1e12+chunks[j].offerId
		})

		// - find offer corresponding to the whole machine
		// - build up the list of free gpu_ids
		var wholeMachine *Chunk
		freeGpuIds := set.New[int](32)
		for _, chunk := range chunks {
			if chunk.frac == 1.0 {
				if wholeMachine == nil {
					wholeMachine = &chunk
				} else {
					log.Warnln(fmt.Sprintf("Offer list inconsistency: machine %d listed multiple times", machineId))
				}
			}
			if chunk.rentable {
				freeGpuIds.InsertSet(&chunk.gpuIds)
			}
		}
		totalGpus := wholeMachine.size
		usedGpus := totalGpus - freeGpuIds.Size()

		// - figure out min_chunk for this machine
		// - correction for the case where whole machine size is not a multiple of min_chunk
		// - in this case, there is always a single remainder chunk which is smaller than actual min_chunk
		//   examples: [1 2 2 2 3 4 7] [1 2 3], actual min_chunk is 2
		//             [3 4 7], actual min_chunk is 4
		//             [1 3 3 3 4 7], actual min_chunk is 3
		// TODO can not be handled properly:
		//             [2 4 6 8 10], actual min_chunk is unknown
		//             [4 6 6 8 12], actual min_chunk is unknown
		minChunkSize := chunks[0].size
		if len(chunks) >= 3 && chunks[0].size != chunks[1].size {
			minChunkSize = chunks[1].size
		}

		// - validate: non-dividable chunks must sum up to the machine size
		chunkGpus := 0
		for _, chunk := range chunks {
			if chunk.size <= minChunkSize {
				chunkGpus += chunk.size
			}
		}
		if chunkGpus != totalGpus {
			chunkSizes := make([]int, 0, len(offers))
			offerIds := make([]int, 0, len(offers))
			for _, chunk := range chunks {
				offerIds = append(offerIds, chunk.offerId)
				chunkSizes = append(chunkSizes, chunk.size)
			}
			log.Warnln(fmt.Sprintf("Offer list inconsistency: machine %d has weird chunk set %v, offer_ids %v",
				machineId, chunkSizes, offerIds))
		}

		// - produce modified offer record with added num_gpus_rented and removed gpu_frac etc
		chunks2 := make([]Chunk2, 0, len(offers))
		for _, chunk := range chunks {
			chunks2 = append(chunks2, Chunk2{
				OfferId:  chunk.offerId,
				Size:     chunk.size,
				Rentable: chunk.rentable,
				GpuIds:   chunk.gpuIdsSorted(),
			})
		}
		newOffer := VastAiRawOffer{
			"num_gpus_rented": usedGpus,
			"min_chunk":       minChunkSize,
			"chunks":          chunks2,
			"gpu_ids":         wholeMachine.gpuIdsSorted(),
		}
		for k, v := range wholeMachine.offer {
			// skip some fields useless for this purpose:
			if k != "gpu_frac" && // always 1.0 for whole machines
				k != "rentable" && // only makes sense for separate offers
				k != "bundle_id" && // useless
				k != "bundled_results" && // useless
				k != "cpu_cores_effective" && // for whole machines equals to cpu_cores
				k != "hostname" && // always null
				k != "id" && // only makes sense for separate offers
				k != "ask_contract_id" && // equals to id
				k != "instance" && // useless
				k != "search" && // useless
				k != "time_remaining" && // always null
				k != "time_remaining_isbid" && //always null
				k != "gpu_ids" { // already there
				newOffer[k] = v
			}
		}

		// - add geolocation
		if geoCache != nil {
			ip, _ := wholeMachine.offer["public_ipaddr"].(string)
			if ip != "" {
				if location := geoCache.ipLocation(ip); location != nil {
					newOffer["location"] = location
				}
			}
		}

		// - calculate inet_up/down
		maxUp := 0.0
		maxDown := 0.0
		for _, offer := range offers {
			maxUp = math.Max(maxUp, offer["inet_up"].(float64))
			maxDown = math.Max(maxDown, offer["inet_down"].(float64))
		}
		if maxUp > 0 && maxDown > 0 {
			newOffer["inet_up"] = maxUp
			newOffer["inet_down"] = maxDown
		} else {
			newOffer["inet_up"] = nil
			newOffer["inet_down"] = nil
		}

		// - find out avg dlperf among minimal chunks
		dlperfPerGpuSum := 0.0
		dlperfPerGpuCount := 0.0
		for _, chunk := range chunks {
			if chunk.size <= minChunkSize {
				dlperfPerGpuSum += chunk.dlperf
				dlperfPerGpuCount += float64(chunk.size)
			}
		}
		dlperfPerGpu := dlperfPerGpuSum / dlperfPerGpuCount
		newOffer["dlperf_chunk"] = dlperfPerGpu * float64(totalGpus)

		// - ensure there is no NaN/Inf in the result
		newOffer.fixFloats()

		result = append(result, newOffer)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].machineId() > result[j].machineId()
	})

	return result
}

func (offer VastAiRawOffer) numGpus() int {
	return int(offer["num_gpus"].(float64))
}

func (offer VastAiRawOffer) gpuFrac() float64 {
	return offer["gpu_frac"].(float64)
}

func (offer VastAiRawOffer) numGpusRented() int {
	return offer["num_gpus_rented"].(int)
}

func (offer VastAiRawOffer) pricePerGpu() int { // in cents
	return int(offer["dph_base"].(float64) / offer["num_gpus"].(float64) * 100)
}

func (offer VastAiRawOffer) id() int {
	return int(offer["id"].(float64))
}

func (offer VastAiRawOffer) machineId() int {
	return int(offer["machine_id"].(float64))
}

func (offer VastAiRawOffer) gpuName() string {
	return offer["gpu_name"].(string)
}

func (offer VastAiRawOffer) verified() bool {
	return offer["verified"].(bool)
}

func (offer VastAiRawOffer) datacenter() bool {
	return int(offer["hosting_type"].(float64)) > 0
}

func (offer VastAiRawOffer) staticIp() bool {
	return offer["static_ip"].(bool)
}

func (offer VastAiRawOffer) rentable() bool {
	return offer["rentable"].(bool)
}

func (offer VastAiRawOffer) dlperf() float64 {
	return offer["dlperf"].(float64)
}

func (offer VastAiRawOffer) gpuIds() *set.Set[int] {
	items := offer["gpu_ids"].([]interface{})
	result := set.New[int](len(items))
	for _, item := range items {
		result.Insert(int(item.(float64)))
	}
	return result
}

func (offer VastAiRawOffer) fixFloats() {
	for k, v := range offer {
		switch fv := v.(type) {
		case float64:
			if math.IsInf(fv, 0) || math.IsNaN(fv) {
				log.Warnln(fmt.Sprintf("Inf or NaN found with key '%s' in %v", k, offer))
				offer[k] = nil
			}
		}
	}
}
