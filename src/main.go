package main

import (
	"net/http"
	"os"
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
	stateDir = kingpin.Flag(
		"state-dir",
		"Path to store state files (default $HOME)",
	).String()
)

func metricsHandler(w http.ResponseWriter, r *http.Request, vastAiCollector prometheus.Collector) {
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

	if *stateDir == "" {
		*stateDir = os.Getenv("HOME")
	}
	if *stateDir == "" {
		*stateDir = "/tmp"
	}

	vastAiCollector := newVastAiCollector()
	log.Infoln("Reading initial Vast.ai info")
	info := getVastAiInfoFromApi()
	err := vastAiCollector.InitialUpdateFrom(info)
	if err != nil {
		// initial update must succeed, otherwise exit
		log.Fatalln(err)
	}

	// additional collector for /metrics-allgpus
	vastAiCollectorAllGpus := newVastAiCollectorAllGpus()
	vastAiCollectorAllGpus.UpdateFrom(info)

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metricsHandler(w, r, vastAiCollector)
	})
	http.HandleFunc("/metrics/allgpus", func(w http.ResponseWriter, r *http.Request) {
		metricsHandler(w, r, vastAiCollectorAllGpus)
	})
	http.HandleFunc("/raw-offers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(info.rawOffers.json())
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
		<head>
		<title>Vast.ai Exporter</title>
		</head>
		<body>
		<h1>Vast.ai Exporter</h1>
		<a href="/metrics">Metrics</a><br>
		<a href="/metrics/allgpus">Site-wide stats for all GPUs</a>
		</body>
		</html>`))
	})

	go func() {
		for {
			time.Sleep(*updateInterval)
			info := getVastAiInfoFromApi()
			vastAiCollector.UpdateFrom(info)
			vastAiCollectorAllGpus.UpdateFrom(info)
		}
	}()

	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
