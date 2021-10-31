package main

import (
	"fmt"
	"net/url"
	"sort"

	"github.com/prometheus/common/log"
)

type VastAiRawOffer map[string]interface{}
type VastAiRawOffers []VastAiRawOffer

func getRawOffersFromApi(result *VastAiApiResults) error {
	var verified, unverified struct {
		Offers VastAiRawOffers `json:"offers"`
	}
	if err := vastApiCall(&verified, "bundles", url.Values{
		"q": {`{"external":{"eq":"false"},"verified":{"eq":"true"},"type":"on-demand","disable_bundling":true}`},
	}); err != nil {
		return err
	}
	if err := vastApiCall(&unverified, "bundles", url.Values{
		"q": {`{"external":{"eq":"false"},"verified":{"eq":"false"},"type":"on-demand","disable_bundling":true}`},
	}); err != nil {
		return err
	}
	result.offersVerified = &verified.Offers
	result.offersUnverified = &unverified.Offers
	return nil
}

func mergeRawOffers(verified VastAiRawOffers, unverified VastAiRawOffers) VastAiRawOffers {
	result := VastAiRawOffers{}
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
	result := VastAiRawOffers{}
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
	invalid := []int{}
	result := offers.filter(func(offer VastAiRawOffer) bool {
		// check if required fields are ok and have a correct type
		machineId, ok1 := offer["machine_id"].(float64)
		_, ok2 := offer["gpu_name"].(string)
		_, ok3 := offer["num_gpus"].(float64)
		_, ok4 := offer["gpu_frac"].(float64)
		_, ok5 := offer["dph_base"].(float64)
		_, ok6 := offer["rentable"].(bool)
		if ok1 && ok2 && ok3 && ok4 && ok5 && ok6 {
			return true
		}
		invalid = append(invalid, int(machineId))
		return false
	})
	if len(invalid) > 0 {
		// this happens often when gpu_frac=nil
		log.Warnln(fmt.Sprintf("Offer list inconsistency: %d offers with missing required fields (machineIds=%v)",
			len(invalid), invalid))
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

func (offers VastAiRawOffers) filterWholeMachines() VastAiRawOffers {
	result := VastAiRawOffers{}

	for machineId, offers := range offers.groupByMachineId() {
		// for each machine:
		// - find out minimal chunk size
		minChunkSize := 10000
		for _, offer := range offers {
			numGpus := offer.numGpus()
			if numGpus < minChunkSize {
				minChunkSize = numGpus
			}
		}

		// - sum gpu numbers over offers minimal chunk offers
		totalGpus := 0
		usedGpus := 0
		for _, offer := range offers {
			numGpus := offer.numGpus()
			if numGpus == minChunkSize {
				totalGpus += numGpus
				if !offer.rentable() {
					usedGpus += numGpus
				}
			}
		}

		// - find whole machine offer
		var wholeOffers []VastAiRawOffer
		for _, offer := range offers {
			if offer.gpuFrac() == 1 {
				wholeOffers = append(wholeOffers, offer)
			}
		}

		// collect list of chunks log message
		chunkList := func() []int {
			chunks := make([]int, 0, len(offers))
			for _, offer := range offers {
				chunks = append(chunks, offer.numGpus())
			}
			sort.Ints(chunks)
			return chunks
		}

		// - validate: there must be exactly one whole machine offer
		if len(wholeOffers) == 0 {
			log.Warnln(fmt.Sprintf("Offer list inconsistency: no offers with gpu_frac=1 for machine %d (chunks=%v)",
				machineId, chunkList()))
			continue
		}
		if len(wholeOffers) > 1 {
			log.Warnln(fmt.Sprintf("Offer list inconsistency: multiple offers with gpu_frac=1 for machine %d (chunks=%v)",
				machineId, chunkList()))
			continue
		}
		wholeOffer := wholeOffers[0]

		// - validate: sum of numGpus of minimal rental chunks must equal to total numGpus of the machine
		machineGpus := wholeOffer.numGpus()
		if totalGpus != machineGpus {
			log.Warnln(fmt.Sprintf("Offer list inconsistency: machine %d has %d GPUs, min chunks sum up to %d GPUs (chunks=%v)",
				machineId, machineGpus, totalGpus, chunkList()))
			continue
		}

		// - produce modified offer record with added num_gpus_rented and removed gpu_frac etc
		newOffer := VastAiRawOffer{
			"num_gpus_rented": usedGpus,
		}
		for k, v := range wholeOffer {
			if k != "gpu_frac" && k != "rentable" && k != "bundle_id" && k != "cpu_cores_effective" {
				newOffer[k] = v
			}
		}
		result = append(result, newOffer)
	}

	return result
}

func (offer VastAiRawOffer) numGpus() int {
	return int(offer["num_gpus"].(float64))
}

func (offer VastAiRawOffer) numGpusRented() int {
	return offer["num_gpus_rented"].(int)
}

func (offer VastAiRawOffer) gpuFrac() float64 {
	return offer["gpu_frac"].(float64)
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
