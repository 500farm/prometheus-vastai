package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/prometheus/common/log"
)

type VastAiInfo struct {
	offers      *[]VastAiOffer
	myMachines  *[]VastAiMachine
	myInstances *[]VastAiInstance
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
	Reliability                   float64 `json:"reliability2"`
	ListedGpuCost                 float64 `json:"listed_gpu_cost"`
	CurrentRentalsOnDemand        int     `json:"current_rentals_on_demand"`
	CurrentRentalsResident        int     `json:"current_rentals_resident"`
	CurrentRentalsRunning         int     `json:"current_rentals_running"`
	CurrentRentalsRunningOnDemand int     `json:"current_rentals_running_on_demand"`
	InetDown                      float64 `json:"inet_down"`
	InetUp                        float64 `json:"inet_up"`
}

type VastAiInstance struct {
	Id           int     `json:"id"`
	MachineId    int     `json:"machine_id"`
	HostId       int     `json:"host_id"`
	ActualStatus string  `json:"actual_status"`
	DphBase      float64 `json:"dph_base"`
	ImageUuid    string  `json:"image_uuid"`
	StartDate    float64 `json:"start_date"`
}

func getVastAiInfo() *VastAiInfo {
	result := new(VastAiInfo)

	var offers []VastAiOffer
	if err := callVastCliJson(&offers, "search", "offers", "-n", "--storage=0", "--disable-bundling", "verified=true"); err != nil {
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

	return result
}

func setVastAiApiKey(key string) error {
	exe, _ := os.Executable()
	os.Chdir(filepath.Dir(exe))
	if os.Getenv("HOME") == "" {
		tmpHome := "/tmp/vast-control"
		os.MkdirAll(tmpHome, 0700)
		os.Setenv("HOME", tmpHome)
	}
	_, err := callVastCli("set", "api-key", key)
	return err
}

func callVastCli(args ...string) ([]byte, error) {
	cmd := exec.Command("./vast", args...)
	stdout, err := cmd.Output()
	stdoutStr := string(stdout)
	if err != nil {
		log.Errorln(cmd.String())
		log.Errorln("output: " + stdoutStr)
		return stdout, err
	}
	if strings.Contains(stdoutStr, "failed with error") {
		log.Errorln(cmd.String())
		log.Errorln("output: " + stdoutStr)
		return stdout, errors.New("Vast CLI call failed")
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
