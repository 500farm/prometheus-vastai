# AGENTS.md — Guide for AI Agents

## What This Project Is

A Prometheus exporter for [Vast.ai](https://vast.ai) GPU marketplace. It periodically fetches offer/machine data from the Vast.ai API, processes it into multiple views (offers, machines, hosts, GPU stats, host map), and serves them as JSON endpoints and Prometheus metrics.

## Build & Run

Go source lives in `src/`. The module root is the repo root (where `go.mod` is).

```sh
# Build
cd src && go build -o ../vastai_exporter .

# Run (requires Vast.ai API key)
./vastai_exporter --key=YOUR_API_KEY

# Run with geolocation
./vastai_exporter --key=YOUR_API_KEY --maxmind-key=USERID:KEY

# Default listen address
# http://localhost:8622
```

Go version: see `go.mod` (currently Go 1.26). Docker build uses `golang:1.26-alpine`.

## Test Mode (Offline Development)

The exporter has a built-in test mode that lets you work offline by replaying saved API data. This is the recommended way to develop and verify changes.

### Step 1: Download test data (requires API key, hits network)

```sh
./vastai_exporter --key=YOUR_API_KEY --state-dir=./testdir --download-test-data
```

This saves raw API responses to `testdir/test-data/`:
- `bundles.json` (~70 MB) — all offers from Vast.ai
- `machines.json` — your machines
- `instances.json` — your instances
- `invoices.json` — your invoices/payouts

### Step 2: Run parsing offline (no network needed)

```sh
./vastai_exporter --state-dir=./testdir --test-parsing
# Optionally with geolocation:
./vastai_exporter --state-dir=./testdir --maxmind-key=USERID:KEY --test-parsing
```

This reads the saved files, runs the full pipeline, and writes all endpoint outputs to `testdir/test-output/`:
- `offers.json`, `machines.json`, `hosts.json`, `gpu-stats.json`, `host-map-data.json`
- `metrics.txt`, `metrics-global.txt`

### Comparing branches

Build two binaries, run both on the same test data, diff the outputs:

```sh
git checkout main && cd src && go build -o ../exporter-a . && cd ..
git checkout other-branch && cd src && go build -o ../exporter-b . && cd ..

./exporter-a --state-dir=./testdir --test-parsing
mv testdir/test-output testdir/output-a

./exporter-b --state-dir=./testdir --test-parsing
mv testdir/test-output testdir/output-b

diff -r testdir/output-a testdir/output-b
```

Note: `testdir/` is in `.gitignore`. Do not commit test data or API keys.

## Data Pipeline

The core data flow on each update cycle:

```
Vast.ai API
    │
    ▼
VastAiRawOffers ([]map[string]any)     ← raw JSON from /bundles endpoint
    │
    ▼  .decode()
VastAiOffers ([]VastAiOffer)            ← decoded + deduplicated, sorted by machine_id
    │
    ▼  .collectMachineOffers()
VastAiMachineOffers ([]VastAiMachineOffer)  ← one per machine, with chunks, gpu_ids, geolocation
    │
    ▼  NewSerializedResponses()
SerializedResponses                     ← pre-serialized JSON + gzip for each endpoint
    │
    ├──► /offers         (raw offers, re-serialized from VastAiOffer.Raw)
    ├──► /machines       (machine-level view with chunks, rented GPUs)
    ├──► /hosts          (grouped by host_id + geolocation)
    ├──► /gpu-stats      (per-GPU-model price/count statistics)
    └──► /host-map-data  (lat/long data for Grafana map panels)
```

Separately, the account collector fetches `/machines`, `/instances`, `/invoices` for per-account Prometheus metrics.

## Key Types

| Type | File | Description |
|------|------|-------------|
| `VastAiRawOffer` | `api_offers_raw.go` | `map[string]any` — one raw JSON offer from the API |
| `VastAiOffer` | `api_offers_decoded.go` | Decoded offer with typed fields + `.Raw` reference |
| `VastAiMachineOffer` | `offer_machine.go` | Machine-level aggregate: whole machine + its chunks |
| `OfferCache` | `offer_cache.go` | Thread-safe cache holding current machine data + pre-serialized responses |
| `OfferCacheSnapshot` | `offer_cache_snapshot.go` | Immutable snapshot for serving requests |
| `CachedResponse` | `response.go` | Pre-serialized JSON (raw + gzipped) with ETag/Last-Modified |
| `Host` | `hosts.go` | Host record grouped by host_id + geolocation |
| `GeoLocation` | `maxmind.go` | Geolocation result from MaxMind, cached to disk |

## Source Files

| File | Purpose |
|------|---------|
| `main.go` | Entry point, CLI flags, HTTP routes, update loop |
| `api.go` | Vast.ai API client (`vastApiCall`, `vastApiCallRaw`), struct types for machines/instances |
| `api_offers_raw.go` | Raw offer fetching, deduplication, field cleanup |
| `api_offers_decoded.go` | `VastAiOffer` struct, decoding from raw, dedup, sorting |
| `api_invoices.go` | Payout/invoice fetching |
| `offer_machine.go` | `collectMachineOffers()` — groups offers into whole machines with chunk info |
| `offer_cache.go` | `OfferCache` — thread-safe store, update logic, triggers serialization |
| `offer_cache_snapshot.go` | Immutable snapshot for concurrent reads |
| `serialized_responses.go` | Builds all JSON responses from machines data |
| `parallel_json.go` | Parallel JSON marshalling/unmarshalling, gzip compression |
| `response.go` | `CachedResponse` struct, HTTP handler with ETag/gzip support |
| `hosts.go` | Host grouping by host_id + geolocation merge key |
| `host_map_data.go` | Grafana-compatible host map data |
| `machine_stats.go` | Per-GPU-model price statistics (median, percentiles, counts) |
| `collector_global.go` | Prometheus collector for global GPU stats |
| `collector_account.go` | Prometheus collector for per-account machine/instance stats |
| `collector_price_stats.go` | Shared price statistics collector logic |
| `metrics.go` | Internal exporter metrics (API latency, response sizes, processing time) |
| `maxmind.go` | MaxMind GeoIP integration, geo cache (persisted to `{state-dir}/.vastai_geo_cache`) |
| `test_mode.go` | `--download-test-data` and `--test-parsing` implementation |

## HTTP Endpoints

| Path | Content | Description |
|------|---------|-------------|
| `/offers` | JSON | All individual offers (raw API fields), ~25k items, ~95 MB |
| `/machines` | JSON | Machine-level view (one per machine), ~5.5k items, ~25 MB |
| `/hosts` | JSON | Hosts grouped by host_id + location, ~1.2k items |
| `/gpu-stats` | JSON | Per-GPU-model statistics (counts, price percentiles) |
| `/host-map-data` | JSON | Lat/long + GPU info for map visualization |
| `/metrics` | Prometheus | Account metrics (or global if no API key) |
| `/metrics/global` | Prometheus | Global per-GPU-model metrics |

All JSON endpoints support `Accept-Encoding: gzip`, `If-None-Match` (ETag), and `If-Modified-Since`.

## External Dependencies

- **Vast.ai API** (`console.vast.ai/api/v0/`) — requires API key (`--key`). Endpoints used: `bundles`, `machines`, `instances`, `users/current/invoices`.
- **MaxMind GeoIP** (optional, `--maxmind-key=USERID:KEY`) — web service for IP geolocation. Results are cached to `{state-dir}/.vastai_geo_cache` to avoid repeated lookups.

## Things to Know

- **All source is in one Go package** (`package main` in `src/`). No sub-packages.
- **JSON serialization uses `go-json-experiment/json` v2** (not `encoding/json`) for offers/machines — see `parallel_json.go`. Hosts and gpu-stats use standard `encoding/json`.
- **Offers are deduplicated by ID**, keeping the copy with the highest `score` field (the API sometimes returns duplicates with different scores).
- **Hosts are sorted by TFLOPS descending**, with lowest machine_id as tie-breaker for determinism.
- **The geo cache** is persisted to disk so MaxMind isn't re-queried for known IPs across restarts.
- **`--master-url`** allows slave instances to fetch offer data from a master exporter instead of hitting Vast.ai directly, reducing API load.
- **State files** are stored in `--state-dir` (default `$HOME`): `.vastai_geo_cache`, `.vastai_last_payouts`.