package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/aquilax/truncate"
	"github.com/prometheus/common/log"
)

type VastAiApiResults struct {
	offers      *[]VastAiOffer
	myMachines  *[]VastAiMachine
	myInstances *[]VastAiInstance
	payouts     *PayoutInfo
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
	InetDown                      float64 `json:"inet_down"`
	InetUp                        float64 `json:"inet_up"`
	NumGpus                       int     `json:"num_gpus"`
	GpuName                       string  `json:"gpu_name"`
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

func getVastAiInfoFromApi() VastAiApiResults {
	result := VastAiApiResults{}

	if err := loadOffers(&result); err != nil {
		log.Errorln(err)
	}

	{
		var response struct {
			Machines []VastAiMachine `json:"machines"`
		}
		if err := vastApiCall(&response, "machines", nil); err != nil {
			log.Errorln(err)
		} else {
			result.myMachines = &response.Machines
		}
	}

	{
		var response struct {
			Instances []VastAiInstance `json:"instances"`
		}
		if err := vastApiCall(&response, "instances", nil); err != nil {
			log.Errorln(err)
		} else {
			result.myInstances = &response.Instances
		}
	}

	payouts, err := getPayouts()
	if err != nil {
		log.Errorln(err)
	} else {
		result.payouts = payouts
	}

	return result
}

func isDefaultJob(instance *VastAiInstance) bool {
	return instance.BundleId == nil
}

func vastApiCall(result interface{}, endpoint string, args url.Values) error {
	if args == nil {
		args = make(url.Values)
	}
	args.Set("api_key", *apiKey)
	resp, err := http.Get("https://vast.ai/api/v0/" + endpoint + "/?" + args.Encode())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logBody := func(body []byte) {
		bodyStr := regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(string(body)), " ")
		log.Errorln(truncate.Truncate(bodyStr, 200, "...", truncate.PositionEnd))
	}
	if resp.StatusCode != 200 {
		logBody(body)
		return fmt.Errorf("endpoint /%s returned: %s", endpoint, resp.Status)
	}
	err = json.Unmarshal(body, result)
	if err != nil {
		logBody(body)
		return err
	}
	return nil
}

func boolToFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
