package main

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	listenAddress = kingpin.Flag(
		"listen",
		"Address to listen on.",
	).Default("0.0.0.0:8622").String()
	apiKey = kingpin.Flag(
		"key",
		"Vast.ai API key",
	).Required().String()
	updateInterval = kingpin.Flag(
		"update-interval",
		"How often to query Vast.ai for updates",
	).Default("1m").Duration()
	gpuName = kingpin.Flag(
		"gpu-name",
		"Name of the GPU used to calculate average on-demand price",
	).Default("RTX 3090").String()
	minDlPerf = kingpin.Flag(
		"min-dl-perf",
		"Minimal DLPerf for GPUs used to calculate average on-demand price",
	).Default("0").Float64()
)

func metricsHandler(w http.ResponseWriter, r *http.Request, vastAiCollector *VastAiCollector) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(vastAiCollector)
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

func main() {
	kingpin.Version(version.Print("vastai_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting vast.ai exporter")

	if err := setVastAiApiKey(*apiKey); err != nil {
		log.Fatalln(err)
	}

	vastAiCollector, _ := newVastAiCollector(*gpuName, *minDlPerf)
	log.Infoln("Reading initial Vast.ai info")
	info, err := getVastAiInfo()
	if err != nil {
		log.Errorln(err)
	} else {
		if info.offers != nil {
			log.Infoln(len(*info.offers), "offers")
		}
		if info.myMachines != nil {
			log.Infoln(len(*info.myMachines), "machines")
		}
		if info.myInstances != nil {
			log.Infoln(len(*info.myInstances), "instances")
		}
		vastAiCollector.Update(info)
	}

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metricsHandler(w, r, vastAiCollector)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
		<head>
		<title>Vast.ai Exporter</title>
		</head>
		<body>
		<h1>Vast.ai Exporter</h1>
		<a href="/metrics">Metrics</a>
		</body>
		</html>`))
	})

	go func() {
		for {
			time.Sleep(*updateInterval)
			info, err := getVastAiInfo()
			if err != nil {
				log.Errorln(err)
			} else {
				vastAiCollector.Update(info)
			}
		}
	}()

	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
