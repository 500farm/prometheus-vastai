package main

import (
	"cmp"
	"fmt"
	"log"
	"slices"

	"github.com/hashicorp/go-set/v2"
)

type VastAiMachineOffers []VastAiMachineOffer

type VastAiMachineOffer struct {
	Raw               VastAiRawOffer
	MachineId         int
	HostId            int
	GpuName           string
	NumGpus           int
	NumGpusRented     int
	MinChunk          int
	PricePerGpu       int // in cents
	Verified          bool
	Datacenter        bool
	StaticIp          bool
	VmsEnabled        bool
	IpAddr            string
	Tflops            float64
	Vram              float64
	InetUp            float64
	InetDown          float64
	DlperfPerGpuChunk float64
	DlperfPerGpuWhole float64
	TflopsPerGpu      float64
	GpuIds            []int
	Chunks            []Chunk2
	Location          *GeoLocation
}

type Chunk2 struct {
	Size     int   `json:"size"`
	OfferId  int   `json:"offerId"`
	Rentable bool  `json:"rentable"`
	GpuIds   []int `json:"gpu_ids"`
}

var machineRawSkipFields = map[string]bool{
	"gpu_frac":           true, // always 1.0 for whole machines
	"rentable":           true, // only makes sense for separate offers
	"bundle_id":          true, // useless
	"bundled_results":    true, // useless
	"cpu_cores_effective": true, // for whole machines equals to cpu_cores
	"hostname":           true, // always null
	"id":                 true, // only makes sense for separate offers
	"ask_contract_id":    true, // equals to id
	"instance":           true, // useless
	"search":             true, // useless
	"time_remaining":     true, // always null
	"time_remaining_isbid": true, // always null
	"gpu_ids":            true, // replaced by whole-machine gpu_ids
}

func (m *VastAiMachineOffer) asRaw() VastAiRawOffer {
	result := make(VastAiRawOffer, len(m.Raw)*2)

	for k, v := range m.Raw {
		if !machineRawSkipFields[k] {
			result[k] = v
		}
	}

	result["num_gpus_rented"] = m.NumGpusRented
	result["min_chunk"] = m.MinChunk
	result["chunks"] = m.Chunks
	result["gpu_ids"] = m.GpuIds

	if m.InetUp > 0 && m.InetDown > 0 {
		result["inet_up"] = m.InetUp
		result["inet_down"] = m.InetDown
	} else {
		result["inet_up"] = nil
		result["inet_down"] = nil
	}

	result["dlperf_chunk"] = m.DlperfPerGpuChunk * float64(m.NumGpus)

	if m.Location != nil {
		result["location"] = m.Location
	}

	result.fixFloats()

	return result
}

type Chunk struct {
	offer    VastAiOffer
	offerId  int
	size     int
	frac     float64
	rentable bool
	dlperf   float64
	gpuIds   set.Set[int]
}

func (chunk Chunk) gpuIdsSorted() []int {
	gpuIds := chunk.gpuIds.Slice()
	slices.Sort(gpuIds)
	return gpuIds
}

func (offers VastAiOffers) collectMachineOffers() VastAiMachineOffers {
	result := make(VastAiMachineOffers, 0, len(offers)/4)

	offers.groupByMachineId(func(machineId int, group VastAiOffers) {
		// - collect array of chunks from smallest to largest
		chunks := make([]Chunk, 0, len(group))
		for _, offer := range group {
			gpuIds := offer.GpuIds
			if gpuIds == nil {
				gpuIds = set.New[int](0)
			}
			chunks = append(chunks, Chunk{
				offer:    offer,
				offerId:  offer.Id,
				size:     offer.NumGpus,
				frac:     offer.GpuFrac,
				rentable: offer.Rentable,
				dlperf:   offer.Dlperf,
				gpuIds:   *gpuIds,
			})
		}
		slices.SortFunc(chunks, func(a, b Chunk) int {
			if c := cmp.Compare(a.size, b.size); c != 0 {
				return c
			}
			return cmp.Compare(a.offerId, b.offerId)
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
					log.Println("WARN:", fmt.Sprintf("Offer list inconsistency: machine %d listed multiple times", machineId))
				}
			}
			if chunk.rentable {
				freeGpuIds.InsertSet(&chunk.gpuIds)
			}
		}

		if wholeMachine == nil {
			log.Println("WARN:", fmt.Sprintf("Offer list inconsistency: machine %d has no chunk with frac=1.0, skipping", machineId))
			return
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
			chunkSizes := make([]int, 0, len(group))
			offerIds := make([]int, 0, len(group))
			for _, chunk := range chunks {
				offerIds = append(offerIds, chunk.offerId)
				chunkSizes = append(chunkSizes, chunk.size)
			}
			log.Println("WARN:", fmt.Sprintf("Offer list inconsistency: machine %d has weird chunk set %v, offer_ids %v",
				machineId, chunkSizes, offerIds))
		}

		// - build chunks2 for the decoded machine
		chunks2 := make([]Chunk2, 0, len(group))
		for _, chunk := range chunks {
			chunks2 = append(chunks2, Chunk2{
				OfferId:  chunk.offerId,
				Size:     chunk.size,
				Rentable: chunk.rentable,
				GpuIds:   chunk.gpuIdsSorted(),
			})
		}

		// - add geolocation
		var location *GeoLocation
		if geoCache != nil {
			if wholeMachine.offer.IpAddr != "" {
				if loc := geoCache.ipLocation(wholeMachine.offer.IpAddr, machineId); loc != nil {
					location = loc
				}
			}
		}

		// - calculate inet_up/down
		maxUp := 0.0
		maxDown := 0.0
		for _, offer := range group {
			maxUp = max(maxUp, offer.InetUp)
			maxDown = max(maxDown, offer.InetDown)
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
		dlperfPerGpuChunk := dlperfPerGpuSum / dlperfPerGpuCount

		// - build the decoded whole machine
		pricePerGpu := 0
		if totalGpus > 0 {
			pricePerGpu = int(wholeMachine.offer.DphBase / float64(totalGpus) * 100)
		}

		if location == nil {
			location = wholeMachine.offer.Location
		}

		wm := VastAiMachineOffer{
			Raw:               wholeMachine.offer.Raw,
			MachineId:         machineId,
			HostId:            wholeMachine.offer.HostId,
			GpuName:           wholeMachine.offer.GpuName,
			NumGpus:           totalGpus,
			NumGpusRented:     usedGpus,
			MinChunk:          minChunkSize,
			PricePerGpu:       pricePerGpu,
			Verified:          wholeMachine.offer.Verified,
			Datacenter:        wholeMachine.offer.Datacenter,
			StaticIp:          wholeMachine.offer.StaticIp,
			VmsEnabled:        wholeMachine.offer.VmsEnabled,
			IpAddr:            wholeMachine.offer.IpAddr,
			Tflops:            wholeMachine.offer.Tflops,
			Vram:              wholeMachine.offer.Vram,
			InetUp:            maxUp,
			InetDown:          maxDown,
			DlperfPerGpuChunk: dlperfPerGpuChunk,
			GpuIds:            wholeMachine.gpuIdsSorted(),
			Chunks:            chunks2,
			Location:          location,
		}
		if totalGpus > 0 {
			wm.DlperfPerGpuWhole = wholeMachine.offer.Dlperf / float64(totalGpus)
			wm.TflopsPerGpu = wholeMachine.offer.Tflops / float64(totalGpus)
		}

		result = append(result, wm)
	})

	return result
}
