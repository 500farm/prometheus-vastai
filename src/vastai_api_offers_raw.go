package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	var verified, unverified struct {
		Offers VastAiRawOffers `json:"offers"`
	}
	result.ts = time.Now()
	if err := vastApiCall(&verified, "bundles", url.Values{
		"q": {`{"external":{"eq":"false"},"verified":{"eq":"true"},"type":"on-demand","disable_bundling":true}`},
	}, bundleTimeout); err != nil {
		return err
	}
	if err := vastApiCall(&unverified, "bundles", url.Values{
		"q": {`{"external":{"eq":"false"},"verified":{"eq":"false"},"type":"on-demand","disable_bundling":true}`},
	}, bundleTimeout); err != nil {
		return err
	}
	offers := mergeRawOffers(verified.Offers, unverified.Offers)
	result.offers = &offers
	return nil
}

func mergeRawOffers(verified VastAiRawOffers, unverified VastAiRawOffers) VastAiRawOffers {
	result := make(VastAiRawOffers, 0, len(verified)+len(unverified))
	for _, offer := range verified {
		offer["verified"] = true
		result = append(result, offer)
	}
	for _, offer := range unverified {
		offer["verified"] = false
		result = append(result, offer)
	}
	for _, offer := range result {
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
	}
	return result
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
		if ok1 && ok2 && ok3 && ok4 && ok5 {
			return true
		}
		log.Warnln(fmt.Sprintf("Offer is missing required fields: %v", offer))
		return false
	})

	// also log offers with gpu_frac=null (this happens for whatever reason)
	for machineId, offers := range offers.groupByMachineId() {
		bad := false
		for _, offer := range offers {
			if _, ok := offer["gpu_frac"].(float64); !ok {
				bad = true
			}
		}
		if bad {
			log.Warnln(fmt.Sprintf("Offer list inconsistency: machine %d has offers with gpu_frac=null", machineId))
		}
	}

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

func (offers VastAiRawOffers) filterWholeMachines(prevResult VastAiRawOffers) VastAiRawOffers {
	result := make(VastAiRawOffers, 0, len(prevResult))

	for machineId, offers := range offers.groupByMachineId() {
		// for each machine:

		// - collect array of chunks from smallest to largest
		chunks := make([]int, 0, len(offers))
		for _, offer := range offers {
			chunks = append(chunks, offer.numGpus())
		}
		sort.Ints(chunks)

		// - create map: chunk size => number of chunks of this size
		countBySize := make(map[int]int)
		for _, size := range chunks {
			countBySize[size]++
		}

		// - find out smallest and largest chunk size
		minChunkSize := chunks[0]
		wholeMachineSize := chunks[len(chunks)-1]

		// - correction for the case where whole machine size is not a multiple of min_chunk
		// - in this case, there is always a single remainder chunk which is smaller than actual min_chunk
		//   examples: [1 2 2 2 3 4 7] [1 2 3], actual min_chunk is 2
		//             [3 4 7], actual min_chunk is 4
		if countBySize[minChunkSize] == 1 && len(chunks) >= 3 {
			minChunkSize = chunks[1]
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
		if countBySize[wholeMachineSize] != 1 || wholeMachineSize != totalGpus {
			log.Warnln(fmt.Sprintf("Offer list inconsistency: machine %d has invalid chunk split %v",
				machineId, chunks))

			// preserve existing record for this machine (if exists)
			for _, prevOffer := range prevResult {
				if prevOffer.machineId() == machineId {
					result = append(result, prevOffer)
					break
				}
			}

			continue
		}

		// - find whole machine offer (with size=wholeMachineSize)
		var wholeOffer VastAiRawOffer
		for _, offer := range offers {
			if offer.numGpus() == wholeMachineSize {
				wholeOffer = offer
				break
			}
		}

		// - produce modified offer record with added num_gpus_rented and removed gpu_frac etc
		newOffer := VastAiRawOffer{
			"num_gpus_rented": usedGpus,
			"min_chunk":       minChunkSize,
		}
		for k, v := range wholeOffer {
			if k != "gpu_frac" && k != "rentable" && k != "bundle_id" && k != "cpu_cores_effective" && k != "hostname" && k != "id" {
				newOffer[k] = v
			}
		}

		// - add geolocation
		if geoCache != nil {
			ip, _ := wholeOffer["public_ipaddr"].(string)
			if ip != "" {
				if location := geoCache.ipLocation(ip); location != nil {
					newOffer["location"] = location
				}
			}
		}

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

func (offer VastAiRawOffer) numGpusRented() int {
	return offer["num_gpus_rented"].(int)
}

func (offer VastAiRawOffer) pricePerGpu() int { // in cents
	return int(offer["dph_base"].(float64) / offer["num_gpus"].(float64) * 100)
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
