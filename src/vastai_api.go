package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/prometheus/common/log"
)

type VastAiApiResults struct {
	offers      *[]VastAiOffer
	myMachines  *[]VastAiMachine
	myInstances *[]VastAiInstance
	payouts     *PayoutInfo
}

type VastAiOffer struct {
	MachineId int     `json:"machine_id"`
	GpuName   string  `json:"gpu_name"`
	NumGpus   int     `json:"num_gpus"`
	GpuFrac   float64 `json:"gpu_frac"`
	DlPerf    float64 `json:"dlperf"`
	DphBase   float64 `json:"dph_base"`
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

func getVastAiInfoFromApi() *VastAiApiResults {
	result := new(VastAiApiResults)

	var offers []VastAiOffer
	if err := callVastCliJson(&offers, "search", "offers", "-n", "--storage=0", "--disable-bundling", "verified=true", "external=false"); err != nil {
		log.Errorln(err)
	} else {
		result.offers = &offers
	}

	var myMachines []VastAiMachine
	if err := callVastCliJson(&myMachines, "show", "machines"); err != nil {
		log.Errorln(err)
	} else {
		result.myMachines = &myMachines
	}

	var myInstances []VastAiInstance
	if err := callVastCliJson(&myInstances, "show", "instances"); err != nil {
		log.Errorln(err)
	} else {
		result.myInstances = &myInstances
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

func setVastAiApiKey(key string) error {
	_, err := callVastCli("set", "api-key", key)
	return err
}

func execWithTimeout(arg0 string, args ...string) (string, []byte, error) {
	timeout := 60 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, arg0, args...)
	stdout, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return cmd.String(), nil, fmt.Errorf("Exec timeout (%v)", timeout)
	}
	return cmd.String(), stdout, err
}

func callVastCli(args ...string) ([]byte, error) {
	cmd, stdout, err := execWithTimeout("vast", args...)
	stdoutStr := strings.TrimSpace(string(stdout))
	if err != nil || strings.Contains(stdoutStr, "failed with error") {
		log.Errorln("-->", cmd)
		if stdoutStr != "" {
			log.Errorln("<--", stdoutStr)
		}
		if err == nil {
			err = errors.New("Vast CLI call failed")
		}
		return stdout, err
	}
	return stdout, nil
}

func callVastCliJson(result interface{}, args ...string) error {
	output, err := callVastCli(append(args, "--raw")...)
	if err != nil {
		return err
	}
	err1 := json.Unmarshal(output, result)
	if err1 != nil {
		return err1
	}
	return nil
}

func boolToFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
