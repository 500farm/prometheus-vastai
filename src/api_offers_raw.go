package main

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"

	"strconv"
	"strings"
	"time"

	"log"

	jsonv2 "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

type VastAiRawOffer map[string]any
type VastAiRawOffers []VastAiRawOffer

func getRawOffersFromMaster(masterUrl string, result *VastAiApiResults) error {
	url := strings.TrimRight(masterUrl, "/") + "/offers"

	start := time.Now()

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if ts := offerCache.Timestamp(); !ts.IsZero() {
		req.Header.Set("If-Modified-Since", ts.UTC().Format(http.TimeFormat))
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		log.Println("INFO: Master returned 304 Not Modified")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		if metrics != nil {
			metrics.ObserveAPIError("master/offers", strconv.Itoa(resp.StatusCode))
		}
		return fmt.Errorf(`URL %s returned "%s"`, url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	elapsed := time.Since(start)
	log.Println("INFO: GET", url, "took", elapsed)

	if metrics != nil {
		metrics.ObserveAPIDuration("master/offers", elapsed.Seconds())
		metrics.ObserveAPIResponseSize("master/offers", len(body))
	}

	defer timeStage("parse_master")()

	var j struct {
		Url       string           `json:"url"`
		Timestamp time.Time        `json:"timestamp"`
		Offers    *VastAiRawOffers `json:"offers"`
	}
	err = jsonv2.Unmarshal(body, &j,
		jsontext.AllowDuplicateNames(true),
	)
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

	if err := vastApiCall(&t, "bundles", url.Values{
		"q": {`{"external":{"eq":"false"},"type":"on-demand","disable_bundling":true}`},
	}, bundleTimeout); err != nil {
		return err
	}

	defer timeStage("parse_api_post")()

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
	result.ts = time.Now()

	return nil
}

func (offer VastAiRawOffer) fixFloats() {
	for k, v := range offer {
		if fv, ok := v.(float64); ok {
			if math.IsInf(fv, 0) || math.IsNaN(fv) {
				log.Println("WARN:", fmt.Sprintf("Inf or NaN found with key '%s' in %v", k, offer))
				offer[k] = nil
			}
		}
	}
}
