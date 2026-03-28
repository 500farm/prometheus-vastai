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

Go version: see `go.mod` (currently Go 1.26).

### Docker

The Docker image uses a multi-stage build: Debian 13 (trixie) Go image for compilation, [distroless](https://github.com/GoogleContainerTools/distroless) static image for runtime. The binary is statically compiled (`CGO_ENABLED=0`) and cross-compiled for multi-platform support via BuildKit args (`TARGETOS`/`TARGETARCH`).

```sh
# Build locally
docker build -t vastai-exporter .

# Run
docker run --rm -p 8622:8622 vastai-exporter --key=YOUR_API_KEY

# Run with persistent state directory
docker run --rm -p 8622:8622 -v ./state:/state vastai-exporter --key=YOUR_API_KEY --state-dir=/state
```

The distroless image has no shell — you cannot `docker exec` into it. For debugging, temporarily switch the runtime stage to `golang:1.26-trixie` or use `gcr.io/distroless/static-debian13:debug`.

### CI/CD

GitHub Actions (`.github/workflows/docker-image.yml`) builds and pushes `500farm/vastai-exporter:latest` to Docker Hub on every push to `main`. PRs trigger a build-only check (no push). The image is built for both `linux/amd64` and `linux/arm64` using Go's native cross-compilation (no QEMU). The Dockerfile splits `go.mod`/`go.sum` into a separate `COPY` + `RUN go mod download` layer so that module downloads are cached when only source files change.

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
- `offers.json`, `machines.json`, `hosts.json`, `gpu-stats.json`, `gpu-stats-v2.json`
- `host-map-data.json`, `host-map-data-dc.json`, `host-map-data-non-dc.json`, `host-map-data-top-10.json`, `host-map-data-top-100.json`  (one file per `?filter=` value)
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
    ├──► /gpu-stats      (per-GPU-model price/count statistics, V1 nested format)
    ├──► /gpu-stats/v2      (per-GPU-model categorized statistics, flat categories)
    └──► /host-map-data     (lat/long data for Grafana map panels)
                               (no ?filter)     all hosts, no zero point
                               ?filter=all      all hosts + zero point
                               ?filter=dc       datacenter hosts only + zero point
                               ?filter=non-dc   non-datacenter hosts only + zero point
                               ?filter=top-10   top 10 hosts by TFLOPS + zero point
                               ?filter=top-100  top 100 hosts by TFLOPS + zero point
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
| `HostMapItem` | `host_map_data.go` | Grafana map item for host map visualization |
| `CategorizedStats_Category` | `machine_stats_v2.go` | V2 per-GPU stats category with dimensions (datacenter, gpu_count_range, verified) and nested rented/available/all stats |
| `CategorizedStats_CategoryStats` | `machine_stats_v2.go` | Nested stats within a category: `Rented`, `Available`, `All` (each a `MachineStats`) |
| `CategorizedStatsGroup` | `machine_stats_v2.go` | Categorized stats grouped by GPU name, with total count for sorting |
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
| `offer_marshal.go` | Pre-allocated JSON marshalling with gzip, using `go-json-experiment/json` v2 |
| `offer_unmarshal.go` | Parallel JSON unmarshalling |
| `flip_buffer.go` | Double-buffered gzip writer for zero-allocation serialization |
| `response.go` | `CachedResponse` struct, HTTP handler with ETag/gzip support |
| `hosts.go` | Host grouping by host_id + geolocation merge key |
| `host_map_data.go` | Grafana-compatible host map data |
| `machine_stats.go` | V1 per-GPU-model price statistics (median, percentiles, counts) |
| `machine_stats_v2.go` | V2 categorized stats: `categorizedStats()` (flat sorted list), `categorizedStatsByGpu()` (grouped + sorted by popularity), custom JSON marshaler |
| `collector_global.go` | Prometheus collector for global GPU stats (embeds V1 + V2 price stats) |
| `collector_account.go` | Prometheus collector for per-account machine/instance stats (embeds V1 + V2 price stats) |
| `collector_price_stats_v1.go` | V1 price statistics Prometheus collector (labels: gpu_name, verified, rented) |
| `collector_price_stats_v2.go` | V2 price statistics Prometheus collector (labels: gpu_name, verified, rented, datacenter, gpu_count_range) |
| `metrics.go` | Internal exporter metrics (API latency, response sizes, processing time) |
| `maxmind.go` | MaxMind GeoIP integration, geo cache (persisted to `{state-dir}/.vastai_geo_cache`) |
| `test_mode.go` | `--download-test-data` and `--test-parsing` implementation |

## HTTP Endpoints

| Path | Content | Description |
|------|---------|-------------|
| `/offers` | JSON | All individual offers (raw API fields), ~25k items, ~95 MB |
| `/machines` | JSON | Machine-level view (one per machine), ~5.5k items, ~25 MB |
| `/hosts` | JSON | Hosts grouped by host_id + location, ~1.2k items |
| `/gpu-stats` | JSON | Per-GPU-model statistics, V1 nested format (rented/available × verified/unverified) |
| `/gpu-stats/v2` | JSON | Per-GPU-model statistics, V2 categories (datacenter, gpu_count_range, verified) with nested rented/available/all stats |
| `/host-map-data` | JSON | Lat/long + GPU info for map visualization. Without `?filter`: all hosts, no extras. With `?filter=all\|dc\|non-dc\|top-10\|top-100`: filtered subset with a zero-size reference point appended as the last item |
| `/metrics` | Prometheus | Account metrics (or global if no API key) |
| `/metrics/global` | Prometheus | Global per-GPU-model metrics |

All JSON endpoints support `Accept-Encoding: gzip`, `If-None-Match` (ETag), and `If-Modified-Since`.

## Prometheus Metrics

### V1 metrics (labels: `gpu_name`, `verified`, `rented`)

- `vastai_gpu_count` — GPU count per model
- `vastai_ondemand_price_median_dollars` — median price per GPU
- `vastai_ondemand_price_10th_percentile_dollars` — 10th percentile price
- `vastai_ondemand_price_90th_percentile_dollars` — 90th percentile price
- `vastai_ondemand_price_per_100dlperf_*` — same but normalized per 100 DLPerf points (global only, labels: `verified`, `rented`)

V1 `verified` and `rented` labels use `"yes"`, `"no"`, `"any"` values. `"any"` aggregates are only emitted for combinations where it's useful (no separate count).

### V2 metrics (labels: `gpu_name`, `verified`, `rented`, `datacenter`, `gpu_count_range`)

- `vastai_v2_gpu_count` — GPU count per category
- `vastai_v2_ondemand_price_median_dollars` — median price per category
- `vastai_v2_ondemand_price_10th_percentile_dollars` — 10th percentile price
- `vastai_v2_ondemand_price_90th_percentile_dollars` — 90th percentile price

V2 `verified` and `datacenter` labels use `"yes"`/`"no"` values. `gpu_count_range` uses `"1-3"`/`"4-7"`/`"8+"`. The `rented` label has three values: `"yes"` (rented GPUs), `"no"` (available GPUs), and `"any"` (all GPUs combined). For each category, the collector emits all three `rented` variants with their respective stats.

The `gpu_count_range` is determined by the machine's total GPU count (not the rented/available portion). For example, a machine with 10 GPUs always falls in `"8+"` regardless of how many are rented.

Both V1 and V2 collectors are embedded in `VastAiGlobalCollector` and `VastAiAccountCollector`, so both metric sets are served on both `/metrics` and `/metrics/global`.

## External Dependencies

- **Vast.ai API** (`console.vast.ai/api/v0/`) — requires API key (`--key`). Endpoints used: `bundles`, `machines`, `instances`, `users/current/invoices`.
- **MaxMind GeoIP** (optional, `--maxmind-key=USERID:KEY`) — web service for IP geolocation. Results are cached to `{state-dir}/.vastai_geo_cache` to avoid repeated lookups.

## Things to Know

- **All source is in one Go package** (`package main` in `src/`). No sub-packages.
- **Docker image is distroless** (`gcr.io/distroless/static-debian13`). No shell, no package manager. The binary is statically linked. Multi-platform: `linux/amd64` + `linux/arm64` via Go cross-compilation (no QEMU).
- **JSON serialization uses `go-json-experiment/json` v2** (not `encoding/json`) for offers/machines — see `marshaler_prealloc.go`. Hosts and gpu-stats use standard `encoding/json`.
- **Offers are deduplicated by ID**, keeping the copy with the highest `score` field (the API sometimes returns duplicates with different scores).
- **Hosts are sorted by TFLOPS descending**, with lowest machine_id as tie-breaker for determinism.
- **The geo cache** is persisted to disk so MaxMind isn't re-queried for known IPs across restarts.
- **`--master-url`** allows slave instances to fetch offer data from a master exporter instead of hitting Vast.ai directly, reducing API load. The slave sends `If-Modified-Since` on subsequent requests; if the master returns 304, the slave keeps its cached data and skips reprocessing. The default `--update-interval` is 5s in master mode (vs 1m when hitting the Vast.ai API directly).
- **State files** are stored in `--state-dir` (default `$HOME`): `.vastai_geo_cache`, `.vastai_last_payouts`.
- **Static analysis**: the project passes `golangci-lint run ./...` cleanly. Keep it that way.