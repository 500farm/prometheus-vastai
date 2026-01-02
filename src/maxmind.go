package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"log"
)

type GeoLocation struct {
	Country      string  `json:"country,omitempty"`
	Location     string  `json:"location,omitempty"`
	Lat          float64 `json:"lat,omitempty"`
	Long         float64 `json:"long,omitempty"`
	Accuracy     float64 `json:"accuracy,omitempty"` // in kilometers
	ISP          string  `json:"isp,omitempty"`
	Organization string  `json:"organization,omitempty"`
	Domain       string  `json:"domain,omitempty"`
}

type GeoCacheEntry struct {
	Expires  time.Time
	Location *GeoLocation
}

type GeoCacheEntries map[string]GeoCacheEntry

type GeoCache struct {
	Entries     GeoCacheEntries
	maxMindUser string
	maxMindPass string
	failed      bool
	skipNets    []net.IPNet
}

type MaxMindResponse struct {
	Country struct {
		IsoCode string `json:"iso_code"`
	} `json:"country"`
	Traits struct {
		Isp          string `json:"isp"`
		Organization string `json:"organization"`
		Domain       string `json:"domain"`
	} `json:"traits"`
	City struct {
		Names struct {
			En string `json:"en"`
		} `json:"names"`
	} `json:"city"`
	Location struct {
		Accuracy float64 `json:"accuracy_radius"`
		Lat      float64 `json:"latitude"`
		Long     float64 `json:"longitude"`
	} `json:"location"`
	SubDivisions []struct {
		IsoCode string `json:"iso_code"`
		Names   struct {
			En string `json:"en"`
		} `json:"names"`
	} `json:"subdivisions"`
}

const GeoLocationTTL = 7 * 24 * time.Hour
const GeoLocationTTLVariance = 3 * time.Hour

var geoCache *GeoCache

func newGeoCache() *GeoCache {
	return &GeoCache{
		Entries: make(GeoCacheEntries),
	}
}

func loadGeoCache() (*GeoCache, error) {
	if *maxMindKey == "" {
		return nil, nil
	}

	cache := newGeoCache()

	t := strings.Split(*maxMindKey, ":")
	if len(t) != 2 {
		return nil, fmt.Errorf(`invalid MaxMind auth "%s": please specify user id and license key separated with ":"`, *maxMindKey)
	}
	cache.maxMindUser = t[0]
	cache.maxMindPass = t[1]

	j, err := os.ReadFile(geoCacheFile())
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		err := json.Unmarshal(j, &cache)
		if err != nil {
			return nil, err
		}
	}

	cache.removeExpired()
	log.Printf("INFO: Loaded geolocation cache: %d items", len(cache.Entries))

	if *noGeoLocation != "" {
		for _, netStr := range strings.Split(*noGeoLocation, ",") {
			netStr = strings.TrimSpace(netStr)
			if len(netStr) > 0 {
				_, net, err := net.ParseCIDR(netStr)
				if err != nil {
					log.Println("ERROR:", err)
				} else {
					cache.skipNets = append(cache.skipNets, *net)
				}
			}
		}
		log.Printf("INFO: Will skip geolocation for: %v", cache.skipNets)
	}

	return cache, nil
}

func (cache *GeoCache) ipLocation(ip string) *GeoLocation {
	parsedIp := net.ParseIP(ip)
	if parsedIp == nil {
		log.Println("WARN: Invalid IP address:", ip)
		return nil
	}
	if !parsedIp.IsGlobalUnicast() || parsedIp.IsPrivate() || isCgnatIp(parsedIp) {
		log.Println("WARN: IP address from an invalid range:", ip)
		return nil
	}
	for _, net := range cache.skipNets {
		if net.Contains(parsedIp) {
			log.Println("INFO: Skipped geolocation for IP:", ip)
			return nil
		}
	}

	entry, found := cache.Entries[ip]
	if found && !entry.expired() {
		return entry.Location
	}

	if cache.failed {
		return nil
	}

	location, err := cache.queryMaxMind(ip)
	if err == nil {
		cache.Entries[ip] = GeoCacheEntry{
			Expires:  makeExpireTime(),
			Location: location,
		}
	} else {
		log.Println("ERROR:", err)
	}
	return location
}

func (cache *GeoCache) save() {
	cache.removeExpired()
	j, _ := json.MarshalIndent(cache, "", "    ")
	err := os.WriteFile(geoCacheFile(), j, 0600)
	if err != nil {
		log.Println("ERROR:", err)
	}
}

func (cache *GeoCache) queryMaxMind(ip string) (*GeoLocation, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("https://geoip.maxmind.com/geoip/v2.1/city/%s?pretty", ip)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(cache.maxMindUser, cache.maxMindPass)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	code := resp.StatusCode
	if code == 404 {
		// IP not found in database, it's not an error
		log.Println("WARN: IP not found by MaxMind:", ip)
		return nil, nil
	}
	if code != 200 {
		log.Println("ERROR:", string(body))
		if code == 401 || code == 402 {
			// 401 Unauthorized or 402 Payment Required
			cache.failed = true
			log.Println("ERROR: Disabling MaxMind until restart because of following:")
		}
		return nil, fmt.Errorf("%s returned: %s", url, resp.Status)
	}
	var j MaxMindResponse
	err = json.Unmarshal(body, &j)
	if err != nil {
		return nil, err
	}

	r := GeoLocation{
		Country:  j.Country.IsoCode,
		Location: j.City.Names.En,
		Lat:      j.Location.Lat,
		Long:     j.Location.Long,
		Accuracy: j.Location.Accuracy,
		ISP:      j.Traits.Isp,
		Domain:   j.Traits.Domain,
	}
	for i := len(j.SubDivisions) - 1; i >= 0; i-- {
		name := j.SubDivisions[i].Names.En
		name = strings.Replace(name, "St.-", "St ", -1) // St.-Petersburg => St Petersburg
		if name != j.City.Names.En {
			if r.Location != "" {
				r.Location += ", "
			}
			r.Location += name
		}
	}
	if j.Traits.Organization != j.Traits.Isp {
		r.Organization = j.Traits.Organization
	}
	return &r, nil
}

func (cache *GeoCache) removeExpired() {
	cache.Entries = cache.Entries.removeExpired()
}

func (entry *GeoCacheEntry) expired() bool {
	return entry.Expires.Before(time.Now())
}

func (entries GeoCacheEntries) removeExpired() GeoCacheEntries {
	newEntries := make(GeoCacheEntries)
	for ip, entry := range entries {
		if !entry.expired() {
			newEntries[ip] = entry
		}
	}
	return newEntries
}

func makeExpireTime() time.Time {
	// randomize so all entries do not expire at the same time
	extra := time.Duration(rand.Int63n(int64(GeoLocationTTLVariance)))
	return time.Now().Add(GeoLocationTTL).Add(extra)
}

func geoCacheFile() string {
	return *stateDir + "/.vastai_geo_cache"
}

var _, cgnNet, _ = net.ParseCIDR("100.64.0.0/10")

func isCgnatIp(ip net.IP) bool {
	if ip == nil {
		return false
	}
	ip = ip.To4()
	if ip == nil {
		return false
	}
	return cgnNet.Contains(ip)
}
