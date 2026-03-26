package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
)

var (
	listenAddress = kingpin.Flag(
		"listen",
		"Address to listen on.",
	).Default(":8622").String()
	apiKey = kingpin.Flag(
		"key",
		"Vast.ai API key",
	).Default("").String()
	updateInterval = kingpin.Flag(
		"update-interval",
		"How often to query Vast.ai for updates (default 5s with --master-url, 1m otherwise)",
	).Default("0").Duration()
	stateDir = kingpin.Flag(
		"state-dir",
		"Path to store state files (default $HOME)",
	).String()
	masterUrl = kingpin.Flag(
		"master-url",
		"Query global data from the master exporter and not from Vast.ai directly.",
	).String()
	maxMindKey = kingpin.Flag(
		"maxmind-key",
		"API key for MaxMind GeoIP web services.",
	).PlaceHolder("USERID:KEY").String()
	noGeoLocation = kingpin.Flag(
		"no-geolocation",
		"Exclude IP ranges from geolocation",
	).PlaceHolder("IP[/NN],IP[/NN],...").String()
	userAgent = kingpin.Flag(
		"user-agent",
		"User-Agent header to use for Vast.ai API requests.",
	).Default("vastai-exporter/1.0 (+https://github.com/500farm/prometheus-vastai)").String()
	downloadTestDataFlag = kingpin.Flag(
		"download-test-data",
		"Download raw API data to state-dir/test-data/ and exit.",
	).Bool()
	testParsingFlag = kingpin.Flag(
		"test-parsing",
		"Parse test data from state-dir/test-data/, write outputs to state-dir/test-output/, and exit.",
	).Bool()
)

var (
	goCollector      = collectors.NewGoCollector()
	processCollector = collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})
)

func metricsHandler(w http.ResponseWriter, r *http.Request, collectors ...prometheus.Collector) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(goCollector)
	registry.MustRegister(processCollector)
	for _, c := range collectors {
		registry.MustRegister(c)
	}
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

func main() {
	kingpin.Version(version.Print("vastai_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	if *updateInterval == 0 {
		if *masterUrl != "" {
			*updateInterval = 5 * time.Second
		} else {
			*updateInterval = 1 * time.Minute
		}
	}

	log.SetFlags(0)

	if *stateDir == "" {
		*stateDir = os.Getenv("HOME")
	}
	if *stateDir == "" {
		*stateDir = "/tmp"
	}

	// "download test data" mode: fetch from API, save to files, exit.
	if *downloadTestDataFlag {
		if *apiKey == "" {
			log.Fatalln("API key is required for --download-test-data")
		}
		downloadTestData()
		return
	}

	// "test parsing mode": redirect all API calls to saved files.
	if *testParsingFlag {
		testDataSource = filepath.Join(*stateDir, "test-data")
		if *apiKey == "" {
			*apiKey = "test"
		}
	}

	if *apiKey == "" {
		log.Fatalln("API key is required")
	}

	log.Println("INFO: Starting vast.ai exporter")

	// load or init geolocation cache (will be nil if MaxMind key is not supploid)
	var err error
	geoCache, err = loadGeoCache()
	if err != nil {
		log.Fatalln(err)
	}

	metrics = newExporterMetrics()

	log.Println("INFO: Reading initial Vast.ai info (may take a minute)")

	// read info from vast.ai: offers
	info := getVastAiInfo(*masterUrl)
	err = offerCache.InitialUpdateFrom(info)
	if err != nil {
		// initial update must succeed, otherwise exit
		log.Fatalln(err)
	}

	snap := offerCache.Snapshot()

	// read info from vast.ai: global stats
	vastAiGlobalCollector := newVastAiGlobalCollector()
	vastAiGlobalCollector.UpdateFrom(snap)

	// read info from vast.ai: account stats (if api key is specified)
	useAccount := *apiKey != ""
	vastAiAccountCollector := newVastAiAccountCollector()
	if useAccount {
		err = vastAiAccountCollector.InitialUpdateFrom(info, snap)
		if err != nil {
			// initial update must succeed, otherwise exit
			log.Fatalln(err)
		}
	} else {
		log.Println("INFO: No Vast.ai API key provided, only serving global stats")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/offers", func(w http.ResponseWriter, r *http.Request) {
		jsonHandler(w, r, offerCache.Snapshot().Offers())
	})
	mux.HandleFunc("/machines", func(w http.ResponseWriter, r *http.Request) {
		jsonHandler(w, r, offerCache.Snapshot().Machines())
	})
	mux.HandleFunc("/hosts", func(w http.ResponseWriter, r *http.Request) {
		jsonHandler(w, r, offerCache.Snapshot().Hosts())
	})
	mux.HandleFunc("/gpu-stats", func(w http.ResponseWriter, r *http.Request) {
		jsonHandler(w, r, offerCache.Snapshot().GpuStats())
	})
	mux.HandleFunc("/gpu-stats/v2", func(w http.ResponseWriter, r *http.Request) {
		jsonHandler(w, r, offerCache.Snapshot().GpuStatsV2())
	})
	mux.HandleFunc("/host-map-data/dc", func(w http.ResponseWriter, r *http.Request) {
		jsonHandler(w, r, offerCache.Snapshot().HostMapDataDC())
	})
	mux.HandleFunc("/host-map-data/non-dc", func(w http.ResponseWriter, r *http.Request) {
		jsonHandler(w, r, offerCache.Snapshot().HostMapDataNonDC())
	})
	mux.HandleFunc("/host-map-data/top-10", func(w http.ResponseWriter, r *http.Request) {
		jsonHandler(w, r, offerCache.Snapshot().HostMapDataTop10())
	})
	mux.HandleFunc("/host-map-data/top-100", func(w http.ResponseWriter, r *http.Request) {
		jsonHandler(w, r, offerCache.Snapshot().HostMapDataTop100())
	})
	mux.HandleFunc("/host-map-data", func(w http.ResponseWriter, r *http.Request) {
		jsonHandler(w, r, offerCache.Snapshot().HostMapData())
	})

	mux.HandleFunc("/metrics/global", func(w http.ResponseWriter, r *http.Request) {
		// global stats
		metricsHandler(w, r, vastAiGlobalCollector, metrics)
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// account stats (if api key is specified)
		if useAccount {
			metricsHandler(w, r, vastAiAccountCollector, metrics)
		} else {
			metricsHandler(w, r, vastAiGlobalCollector, metrics)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// index page
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		var metricsLinks []string
		if useAccount {
			metricsLinks = []string{
				`<h2>Prometheus endpoints</h2>`,
				`<p><a href="metrics">Account stats</a></p>`,
				`<p><a href="metrics/global">Per-model stats on GPUs</a></p>`,
			}
		} else {
			metricsLinks = []string{
				`<h2>Prometheus endpoints</h2>`,
				`<p><a href="metrics">Per-model stats on GPUs</a></p>`,
			}
		}
		lines := []string{
			`<html>`,
			`<head>`,
			`<title>Vast.ai Exporter</title>`,
			`<style>`,
			`  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }`,
			`  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; font-size: 15px; line-height: 1.6; color: #1a1a1a; background: #f9f9fb; max-width: 640px; margin: 40px 0 0 60px; padding: 0; }`,
			`  h1 { font-size: 1.6rem; font-weight: 600; letter-spacing: -0.02em; margin-bottom: 8px; }`,
			`  .subtitle { color: #666; font-size: 0.9rem; margin-bottom: 48px; }`,
			`  h2 { font-size: 0.7rem; font-weight: 600; letter-spacing: 0.1em; text-transform: uppercase; color: #999; margin: 32px 0 12px; }`,
			`  p { margin: 6px 0; }`,
			`  a { color: #0066cc; text-decoration: none; }`,
			`  a:hover { text-decoration: underline; }`,
			`  hr { border: none; border-top: 1px solid #e5e5e5; margin: 40px 0; }`,
			`</style>`,
			`</head>`,
			`<body>`,
			`<h1>Vast.ai Exporter</h1>`,
			`<p class="subtitle">Prometheus/JSON exporter for Vast.ai GPU marketplace</p>`,
		}
		lines = append(lines, metricsLinks...)
		lines = append(lines,
			`<hr>`,
			`<h2>JSON endpoints</h2>`,
			`<p><a href="offers">List of offers</a></p>`,
			`<p><a href="machines">List of machines</a></p>`,
			`<p><a href="hosts">List of hosts</a></p>`,
			`<p><a href="gpu-stats">Per-model stats on GPUs</a></p>`,
			`<p><a href="gpu-stats/v2">Per-model stats on GPUs (categorized)</a></p>`,
			`<p><a href="host-map-data">Data source for map of hosts</a></p>`,
			`</body>`,
			`</html>`,
		)
		fmt.Fprint(w, strings.Join(lines, "\n"))
	})

	// "test parsing mode": fetch all endpoints to files and exit.
	if *testParsingFlag {
		testFetchAllEndpoints(mux)
		return
	}

	go func() {
		for {
			time.Sleep(*updateInterval)

			info := getVastAiInfo(*masterUrl)
			offerCache.UpdateFrom(info)
			snap := offerCache.Snapshot()

			vastAiGlobalCollector.UpdateFrom(snap)
			if useAccount {
				vastAiAccountCollector.UpdateFrom(info, snap)
			}

			// not neeeded anymore
			offerCache.ClearMachines()
		}
	}()

	log.Println("INFO: Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, mux))
}
