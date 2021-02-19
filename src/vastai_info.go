package main

type VastAiInfo struct {
	offers      *[]VastAiOffer
	myMachines  *[]VastAiMachine
	myInstances *[]VastAiInstance
}

type VastAiOffer struct {
	Price float64
}

type VastAiMachine struct {
}

type VastAiInstance struct {
}

func getVastAiInfo() (*VastAiInfo, error) {
	result := new(VastAiInfo)
	return result, nil
}
