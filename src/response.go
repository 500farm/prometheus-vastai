package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type JsonResponse struct {
	Content      []byte
	LastModified time.Time
	ETag         string
}

type cachedResponse struct {
	ts      time.Time
	etag    string
	raw     []byte
	gzipped []byte
}

type endpointCache struct {
	mu      sync.Mutex
	current *cachedResponse
}

var (
	registryMu    sync.Mutex
	cacheRegistry = make(map[string]*endpointCache)
)

func (cache *OfferCacheSnapshot) etag(endpoint string) string {
	hash := sha256.Sum256([]byte(cache.ts.Format(time.RFC3339Nano) + "|" + endpoint))
	return fmt.Sprintf(`"%x"`, hash[:8])
}

func jsonHandler(w http.ResponseWriter, r *http.Request, currentTs time.Time, generate func() JsonResponse) {
	endpoint := r.URL.Path

	cached := getEndpointCache(endpoint).get(currentTs, generate)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
	w.Header().Set("Last-Modified", cached.ts.UTC().Format(http.TimeFormat))
	w.Header().Set("ETag", cached.etag)

	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == cached.etag {
			w.WriteHeader(http.StatusNotModified)
			if metrics != nil {
				metrics.ObserveServerNotModified(endpoint)
			}
			return
		}

	} else if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		if t, err := http.ParseTime(ims); err == nil && !cached.ts.Truncate(time.Second).After(t) {
			w.WriteHeader(http.StatusNotModified)
			if metrics != nil {
				metrics.ObserveServerNotModified(endpoint)
			}
			return
		}
	}

	written := 0
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Length", strconv.Itoa(len(cached.gzipped)))
		w.Write(cached.gzipped)
		written = len(cached.gzipped)

	} else {
		w.Header().Set("Content-Length", strconv.Itoa(len(cached.raw)))
		w.Write(cached.raw)
		written = len(cached.raw)
	}

	if metrics != nil {
		metrics.ObserveServerResponse(endpoint, written)
	}
}

func (ec *endpointCache) get(currentTs time.Time, generate func() JsonResponse) *cachedResponse {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	if ec.current != nil && ec.current.ts.Equal(currentTs) {
		return ec.current
	}

	resp := generate()

	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	gz.Write(resp.Content)
	gz.Close()

	ec.current = &cachedResponse{
		ts:      currentTs,
		etag:    resp.ETag,
		raw:     resp.Content,
		gzipped: buf.Bytes(),
	}

	return ec.current
}

func getEndpointCache(endpoint string) *endpointCache {
	registryMu.Lock()
	defer registryMu.Unlock()

	ec := cacheRegistry[endpoint]
	if ec == nil {
		ec = &endpointCache{}
		cacheRegistry[endpoint] = ec
	}

	return ec
}
