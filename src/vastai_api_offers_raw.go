package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

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
}

type Chunk2 struct {
	Size     int  `json:"size"`
	OfferId  int  `json:"offerId"`
	Rentable bool `json:"rentable"`
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
			})
		}
		sort.Slice(chunks, func(i, j int) bool {
			return chunks[i].size*1e12+chunks[i].offerId < chunks[j].size*1e12+chunks[j].offerId
		})

		// - create map: chunk size => number of chunks of this size
		// - find offer corresponding to the whole machine
		countBySize := make(map[int]int)
		var wholeMachine *Chunk
		for _, chunk := range chunks {
			countBySize[chunk.size]++
			if chunk.frac == 1.0 {
				wholeMachine = &chunk
			}
		}

		// - correction for the case where whole machine size is not a multiple of min_chunk
		// - in this case, there is always a single remainder chunk which is smaller than actual min_chunk
		//   examples: [1 2 2 2 3 4 7] [1 2 3], actual min_chunk is 2
		//             [3 4 7], actual min_chunk is 4
		// TODO currently unhandled:
		//             [1 3 3 3 4 7], actual min_chunk is 3
		//             [2 4 6 8 10], ???
		minChunkSize := chunks[0].size
		if countBySize[minChunkSize] == 1 && len(chunks) >= 3 {
			minChunkSize = chunks[1].size
		}

		// - iterate over non-dividable chunks and sum up GPU counts
		totalGpus := 0
		usedGpus := 0
		for _, offer := range offers {
			numGpus := offer.numGpus()
			if numGpus <= minChunkSize {
				totalGpus += numGpus
				if !offer.rentable() {
					usedGpus += numGpus
				}
			}
		}

		// - validate: there must be exactly one whole machine offer, and non-dividable chunks must sum up to the machine size
		if wholeMachine == nil || wholeMachine.size != totalGpus || countBySize[wholeMachine.size] != 1 {
			chunkSizes := make([]int, 0, len(offers))
			offerIds := make([]int, 0, len(offers))
			for _, chunk := range chunks {
				offerIds = append(offerIds, chunk.offerId)
				chunkSizes = append(chunkSizes, chunk.size)
			}

			log.Warnln(fmt.Sprintf("Offer list inconsistency: machine %d has invalid chunk split %v, offer_ids %v",
				machineId, chunkSizes, offerIds))

			// preserve existing record for this machine (if exists)
			for _, prevOffer := range prevResult {
				if prevOffer.machineId() == machineId {
					result = append(result, prevOffer)
					break
				}
			}

			continue
		}

		// - produce modified offer record with added num_gpus_rented and removed gpu_frac etc
		chunks2 := make([]Chunk2, 0, len(offers))
		for _, chunk := range chunks {
			chunks2 = append(chunks2, Chunk2{
				OfferId:  chunk.offerId,
				Size:     chunk.size,
				Rentable: chunk.rentable,
			})
		}
		newOffer := VastAiRawOffer{
			"num_gpus_rented": usedGpus,
			"min_chunk":       minChunkSize,
			"chunks":          chunks2,
		}
		for k, v := range wholeMachine.offer {
			if k != "gpu_frac" && k != "rentable" && k != "bundle_id" && k != "cpu_cores_effective" && k != "hostname" && k != "id" {
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
		newOffer["dlperf"] = dlperfPerGpu * float64(totalGpus)
		v := dlperfPerGpu / float64(wholeMachine.offer.pricePerGpu()) * 100
		if !math.IsInf(v, 0) {
			newOffer["dlperf_per_dphtotal"] = v
		}

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

func (offer VastAiRawOffer) rentable() bool {
	return offer["rentable"].(bool)
}

func (offer VastAiRawOffer) dlperf() float64 {
	return offer["dlperf"].(float64)
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
