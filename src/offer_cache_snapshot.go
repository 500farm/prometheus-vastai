package main

import (
	"time"
)

type OfferCacheSnapshot struct {
	offerCount int
	machines   VastAiMachineOffers
	responses  SerializedResponses
	ts         time.Time
}

func (cache *OfferCache) Snapshot() *OfferCacheSnapshot {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	return &OfferCacheSnapshot{
		offerCount: cache.offerCount,
		machines:   cache.machines,
		responses:  cache.responses,
		ts:         cache.ts,
	}
}

func (snap *OfferCacheSnapshot) getCachedResponse(endpoint string) *CachedResponse {
	if snap.responses == nil {
		return nil
	}
	return snap.responses[endpoint]
}

func (snap *OfferCacheSnapshot) Offers() *CachedResponse     { return snap.getCachedResponse("/offers") }
func (snap *OfferCacheSnapshot) Machines() *CachedResponse   { return snap.getCachedResponse("/machines") }
func (snap *OfferCacheSnapshot) Hosts() *CachedResponse      { return snap.getCachedResponse("/hosts") }
func (snap *OfferCacheSnapshot) GpuStats() *CachedResponse   { return snap.getCachedResponse("/gpu-stats") }
func (snap *OfferCacheSnapshot) GpuStatsV2() *CachedResponse { return snap.getCachedResponse("/gpu-stats/v2") }
func (snap *OfferCacheSnapshot) HostMapData(filter string) *CachedResponse {
	if filter == "" {
		return snap.getCachedResponse("/host-map-data")
	}
	return snap.getCachedResponse("/host-map-data?filter=" + filter)
}
