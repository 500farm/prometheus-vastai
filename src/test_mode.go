package main

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// when non-empty, makes vastApiCallRaw read from files instead of the network
var testDataSource string

var testDataFiles = map[string]string{
	"bundles":                testFileBundles,
	"machines":               testFileMachines,
	"instances":              testFileInstances,
	"users/current/invoices": testFileInvoices,
}

const (
	testFileBundles   = "bundles.json"
	testFileMachines  = "machines.json"
	testFileInstances = "instances.json"
	testFileInvoices  = "invoices.json"
)

func readTestData(endpoint string) ([]byte, bool) {
	if testDataSource == "" {
		return nil, false
	}

	filename, ok := testDataFiles[endpoint]
	if !ok {
		return nil, false
	}

	path := filepath.Join(testDataSource, filename)
	body, err := os.ReadFile(path)
	if err != nil {
		log.Printf("ERROR: %v", err)
		return nil, false
	}

	log.Printf("INFO: Read %s (%d bytes)", path, len(body))
	return body, true
}

func downloadTestData() {
	dir := filepath.Join(*stateDir, "test-data")
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalln("ERROR:", err)
	}

	type fetch struct {
		name     string
		file     string
		endpoint string
		args     url.Values
		timeout  time.Duration
	}

	for _, f := range []fetch{
		{"bundles (offers)", testFileBundles, "bundles",
			url.Values{"q": {`{"external":{"eq":"false"},"type":"on-demand","disable_bundling":true}`}},
			bundleTimeout},
		{"machines", testFileMachines, "machines", nil, defaultTimeout},
		{"instances", testFileInstances, "instances", nil, defaultTimeout},
		{"invoices", testFileInvoices, "users/current/invoices", nil, defaultTimeout},
	} {
		log.Printf("INFO: Downloading %s...", f.name)

		body, err := vastApiCallRaw(f.endpoint, f.args, f.timeout)
		if err != nil {
			log.Fatalf("ERROR: Failed to fetch %s: %v", f.name, err)
		}

		path := filepath.Join(dir, f.file)
		if err := os.WriteFile(path, body, 0644); err != nil {
			log.Fatalf("ERROR: Failed to write %s: %v", path, err)
		}

		log.Printf("INFO: Saved %s (%d bytes)", path, len(body))

		time.Sleep(queryInterval)
	}

	log.Println("INFO: All test data downloaded to", dir)
}

func testFetchAllEndpoints(mux http.Handler) {
	outDir := filepath.Join(*stateDir, "test-output")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalln("ERROR:", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	for _, ep := range []struct{ path, file string }{
		{"/offers", "offers.json"},
		{"/machines", "machines.json"},
		{"/hosts", "hosts.json"},
		{"/gpu-stats", "gpu-stats.json"},
		{"/gpu-stats/v2", "gpu-stats-v2.json"},
		{"/host-map-data", "host-map-data.json"},
		{"/host-map-data?filter=dc", "host-map-data-dc.json"},
		{"/host-map-data?filter=non-dc", "host-map-data-non-dc.json"},
		{"/host-map-data?filter=top-10", "host-map-data-top-10.json"},
		{"/host-map-data?filter=top-100", "host-map-data-top-100.json"},
		{"/metrics", "metrics.txt"},
		{"/metrics/global", "metrics-global.txt"},
	} {
		resp, err := ts.Client().Get(ts.URL + ep.path)
		if err != nil {
			log.Printf("ERROR: GET %s: %v", ep.path, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("ERROR: GET %s: %s", ep.path, resp.Status)
			continue
		}

		path := filepath.Join(outDir, ep.file)
		if err := os.WriteFile(path, body, 0644); err != nil {
			log.Printf("ERROR: %v", err)
			continue
		}

		log.Printf("INFO: Wrote %s (%d bytes)", path, len(body))
	}

	log.Println("INFO: All test output written to", outDir)
}
