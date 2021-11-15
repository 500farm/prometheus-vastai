package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/common/log"
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

	expires time.Time
}

type GeoCache map[string]*GeoLocation

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

var (
	geoCache      GeoCache
	maxMindUser   string
	maxMindPass   string
	maxMindFailed bool
)

func ipLocation(ip string) *GeoLocation {
	if !useMaxMind() {
		return nil
	}

	r := geoCache[ip]
	if r != nil && r.expires.After(time.Now()) {
		return r
	}

	r, err := queryMaxMind(ip)
	if err == nil {
		geoCache[ip] = r
	} else {
		log.Errorln(err)
	}
	return r
}

func queryMaxMind(ip string) (*GeoLocation, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("https://geoip.maxmind.com/geoip/v2.1/city/%s?pretty", ip)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(maxMindUser, maxMindPass)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	code := resp.StatusCode
	if code == 404 {
		// IP not found in database, it's not an error, return empty record
		return &GeoLocation{
			expires: time.Now().Add(GeoLocationTTL),
		}, nil
	}
	if code != 200 {
		log.Errorln(string(body))
		if code == 401 || code == 402 {
			// 401 Unauthorized or 402 Payment Required
			maxMindFailed = true
			log.Errorln("Disabling MaxMind until restart because of following:")
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
		expires:  time.Now().Add(GeoLocationTTL),
	}
	for _, sd := range j.SubDivisions {
		name := sd.Names.En
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

func loadGeoCache() error {
	if !useMaxMind() {
		return nil
	}

	t := strings.Split(*maxMindKey, ":")
	if len(t) != 2 {
		return fmt.Errorf(`invalid MaxMind auth "%s": please specify user id and license key separated with ":"`, *maxMindKey)
	}
	maxMindUser = t[0]
	maxMindPass = t[1]

	geoCache = make(GeoCache)
	j, err := ioutil.ReadFile(geoCacheFile())
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		err := json.Unmarshal(j, &geoCache)
		if err != nil {
			return err
		}
	}
	log.Infof("Loaded geolocation cache: %d items", len(geoCache))
	return nil
}

func saveGeoCache() {
	if !useMaxMind() {
		return
	}

	j, _ := json.Marshal(geoCache)
	err := ioutil.WriteFile(geoCacheFile(), j, 0600)
	if err != nil {
		log.Errorln(err)
	}
}

func geoCacheFile() string {
	return *stateDir + "/.vastai_geo_cache"
}

func useMaxMind() bool {
	return *maxMindKey != "" && !maxMindFailed
}
