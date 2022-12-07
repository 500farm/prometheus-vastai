package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/aquilax/truncate"
	"github.com/prometheus/common/log"
)

type VastAiApiResults struct {
	offers      *VastAiRawOffers
	myMachines  *[]VastAiMachine
	myInstances *[]VastAiInstance
	payouts     *PayoutInfo
	ts          time.Time
}

type VastAiMachine struct {
	Id                            int     `json:"id"`
	Hostname                      string  `json:"hostname"`
	Verification                  string  `json:"verification"`
	Listed                        bool    `json:"listed"`
	Reliability                   float64 `json:"reliability2"`
	Timeout                       float64 `json:"timeout"`
	ListedGpuCost                 float64 `json:"listed_gpu_cost"`
	CurrentRentalsOnDemand        int     `json:"current_rentals_on_demand"`
	CurrentRentalsResident        int     `json:"current_rentals_resident"`
	CurrentRentalsRunning         int     `json:"current_rentals_running"`
	CurrentRentalsRunningOnDemand int     `json:"current_rentals_running_on_demand"`
	GpuOccupancy                  string  `json:"gpu_occupancy"`
	InetDown                      float64 `json:"inet_down"`
	InetUp                        float64 `json:"inet_up"`
	NumGpus                       int     `json:"num_gpus"`
	GpuName                       string  `json:"gpu_name"`
	TFlops                        float64 `json:"total_flops"`
}

type VastAiInstance struct {
	Id           int     `json:"id"`
	MachineId    int     `json:"machine_id"`
	ActualStatus string  `json:"actual_status"`
	DphBase      float64 `json:"dph_base"`
	ImageUuid    string  `json:"image_uuid"`
	StartDate    float64 `json:"start_date"`
	IsBid        bool    `json:"is_bid"`
	MinBid       float64 `json:"min_bid"`
	BundleId     *int    `json:"bundle_id"`
	NumGpus      int     `json:"num_gpus"`
	GpuName      string  `json:"gpu_name"`
}

const defaultTimeout = 30 * time.Second
const bundleTimeout = 30 * time.Second
const queryInterval = 5 * time.Second

func getVastAiInfo(masterUrl string) VastAiApiResults {
	result := VastAiApiResults{}

	var err error
	if masterUrl != "" {
		// query offer from master exporter
		err = getRawOffersFromMaster(masterUrl, &result)
	} else {
		// query offers from Vast.ai API
		err = getRawOffersFromApi(&result)
		time.Sleep(queryInterval)
	}
	if err != nil {
		log.Errorln(err)
	}

	if *apiKey == "" {
		return result
	}

	var response1 struct {
		Machines []VastAiMachine `json:"machines"`
	}
	if err := vastApiCall(&response1, "machines", nil, defaultTimeout); err != nil {
		log.Errorln(err)
	} else {
		result.myMachines = &response1.Machines
	}
	time.Sleep(queryInterval)

	var response2 struct {
		Instances []VastAiInstance `json:"instances"`
	}
	if err := vastApiCall(&response2, "instances", nil, defaultTimeout); err != nil {
		log.Errorln(err)
	} else {
		result.myInstances = &response2.Instances
	}
	time.Sleep(queryInterval)

	payouts, err := getPayouts()
	if err != nil {
		log.Errorln(err)
	} else {
		result.payouts = payouts
	}

	return result
}

func (instance *VastAiInstance) isDefaultJob() bool {
	return instance.BundleId == nil
}

func vastApiCall(result interface{}, endpoint string, args url.Values, timeout time.Duration) error {
	body, err := vastApiCallRaw(endpoint, args, timeout)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, result)
	if err != nil {
		logErrorBody(body)
		return err
	}
	return nil
}

func vastApiCallRaw(endpoint string, args url.Values, timeout time.Duration) ([]byte, error) {
	if args == nil {
		args = make(url.Values)
	}
	if *apiKey != "" {
		args.Set("api_key", *apiKey)
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get("https://vast.ai/api/v0/" + endpoint + "/?" + args.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		logErrorBody(body)
		return nil, fmt.Errorf("endpoint /%s returned: %s", endpoint, resp.Status)
	}
	return body, nil
}

func logErrorBody(body []byte) {
	bodyStr := regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(string(body)), " ")
	log.Errorln(truncate.Truncate(bodyStr, 200, "...", truncate.PositionEnd))
}

func boolToFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
