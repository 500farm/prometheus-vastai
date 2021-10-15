package main

import (
	"fmt"
	"math"
	"net/url"

	"github.com/montanaflynn/stats"
	"github.com/prometheus/common/log"
)

type VastAiRawOffer map[string]interface{}
type VastAiRawOffers []VastAiRawOffer

type VastAiOffer struct {
	MachineId     int
	GpuName       string
	NumGpus       int
	NumGpusRented int
	PricePerGpu   float64
	Verified      bool
}
type VastAiOffers []VastAiOffer

type GroupedOffers map[string]VastAiOffers

type OfferStats struct {
	Count                                 int
	Median, PercentileLow, PercentileHigh float64
}

type OfferStats2 struct {
	Verified, Unverified, All OfferStats
}

func getOffersFromApi(result *VastAiApiResults) error {
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
	return offers.filter(func(offer VastAiRawOffer) bool {
		// check if required fields are ok and have a correct type
		// (e.g. for some machines gpu_frac can be null for unknown reason)
		_, ok1 := offer["machine_id"].(float64)
		_, ok2 := offer["gpu_name"].(string)
		_, ok3 := offer["num_gpus"].(float64)
		_, ok4 := offer["gpu_frac"].(float64)
		_, ok5 := offer["dph_base"].(float64)
		_, ok6 := offer["rentable"].(bool)
		if ok1 && ok2 && ok3 && ok4 && ok5 && ok6 {
			return true
		}
		// this happens often for some offers, probably just listed - ignoring for now
		// log.Errorln("Invalid offer record:", offer)
		return false
	})
}

func (offers VastAiRawOffers) groupByMachineId() map[int]VastAiRawOffers {
	grouped := make(map[int]VastAiRawOffers)
	for _, offer := range offers {
		machineId := int(offer["machine_id"].(float64))
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
			numGpus := int(offer["num_gpus"].(float64))
			if numGpus < minChunkSize {
				minChunkSize = numGpus
			}
		}

		// - sum gpu numbers over offers minimal chunk offers
		totalGpus := 0
		usedGpus := 0
		for _, offer := range offers {
			numGpus := int(offer["num_gpus"].(float64))
			rentable := offer["rentable"].(bool)
			if numGpus == minChunkSize {
				totalGpus += numGpus
				if !rentable {
					usedGpus += numGpus
				}
			}
		}

		// - find whole machine offer
		var wholeOffers []VastAiRawOffer
		for _, offer := range offers {
			gpuFrac := offer["gpu_frac"].(float64)
			if gpuFrac == 1 {
				wholeOffers = append(wholeOffers, offer)
			}
		}

		// - validate: there must be exactly one whole machine offer
		if len(wholeOffers) == 0 {
			log.Errorln(fmt.Sprintf("No offers with gpu_frac=1 for machine %d", machineId))
			continue
		}
		if len(wholeOffers) > 1 {
			log.Errorln(fmt.Sprintf("Multiple offers with gpu_frac=1 for machine %d: %v", machineId, wholeOffers))
			continue
		}
		wholeOffer := wholeOffers[0]

		// - validate: sum of numGpus of minimal rental chunks must equal to total numGpus of the machine
		machineGpus := int(wholeOffer["num_gpus"].(float64))
		if totalGpus != machineGpus {
			log.Errorln(fmt.Sprintf("GPU number mismtach for machine %d: machine has %d GPUs, min chunks sum up to %d GPUs: %v",
				machineId, machineGpus, totalGpus, offers))
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

func (offers VastAiRawOffers) decode() VastAiOffers {
	result := VastAiOffers{}
	for _, offer := range offers {
		numGpus := offer["num_gpus"].(float64)
		result = append(result, VastAiOffer{
			MachineId:     int(offer["machine_id"].(float64)),
			GpuName:       offer["gpu_name"].(string),
			NumGpus:       int(numGpus),
			NumGpusRented: int(offer["num_gpus_rented"].(int)),
			PricePerGpu:   offer["dph_base"].(float64) / numGpus,
			Verified:      offer["verified"].(bool),
		})
	}
	return result
}

func (offers VastAiOffers) groupByGpu() GroupedOffers {
	offersByGpu := make(GroupedOffers)
	for _, offer := range offers {
		name := offer.GpuName
		offersByGpu[name] = append(offersByGpu[name], offer)
	}
	return offersByGpu
}

func (offers VastAiOffers) filter(filter func(*VastAiOffer) bool) VastAiOffers {
	result := VastAiOffers{}
	for _, offer := range offers {
		if filter(&offer) {
			result = append(result, offer)
		}
	}
	return result
}

func (offers VastAiOffers) filterVerified() VastAiOffers {
	return offers.filter(func(offer *VastAiOffer) bool { return offer.Verified })
}

func (offers VastAiOffers) filterUnverified() VastAiOffers {
	return offers.filter(func(offer *VastAiOffer) bool { return !offer.Verified })
}

func (offers VastAiOffers) stats() OfferStats {
	prices := []float64{}
	for _, offer := range offers {
		pricePerGpu := offer.PricePerGpu
		for i := 0; i < offer.NumGpus; i++ {
			prices = append(prices, pricePerGpu)
		}
	}

	result := OfferStats{
		Count:          len(prices),
		Median:         math.NaN(),
		PercentileLow:  math.NaN(),
		PercentileHigh: math.NaN(),
	}
	if len(prices) > 0 {
		result.Median, _ = stats.Median(prices)
		result.PercentileLow, _ = stats.Percentile(prices, 10)
		result.PercentileHigh, _ = stats.Percentile(prices, 90)
	}
	return result
}

func (offers VastAiOffers) stats2() OfferStats2 {
	return OfferStats2{
		Verified:   offers.filterVerified().stats(),
		Unverified: offers.filterUnverified().stats(),
		All:        offers.stats(),
	}
}
