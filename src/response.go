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
	etag         string
	lastModified time.Time
	raw          []byte
	gzipped      []byte
}

var (
	jsonCacheMu sync.Mutex
	jsonCache   = make(map[string]*cachedResponse)
)

func computeETag(ts time.Time) string {
	hash := sha256.Sum256([]byte(ts.Format(time.RFC3339Nano)))
	return fmt.Sprintf(`"%x"`, hash[:8])
}

func jsonHandler(w http.ResponseWriter, r *http.Request, generate func() JsonResponse) {
	endpoint := r.URL.Path
	etag := offerCache.etag
	lastModified := offerCache.ts

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
	w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	w.Header().Set("ETag", etag)

	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == etag {
			w.WriteHeader(http.StatusNotModified)
			if metrics != nil {
				metrics.ObserveServerNotModified(endpoint)
			}
			return
		}
	} else if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		if t, err := http.ParseTime(ims); err == nil && !lastModified.Truncate(time.Second).After(t) {
			w.WriteHeader(http.StatusNotModified)
			if metrics != nil {
				metrics.ObserveServerNotModified(endpoint)
			}
			return
		}
	}

	cached := getCachedResponse(endpoint, etag, generate)

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

func getCachedResponse(endpoint string, etag string, generate func() JsonResponse) *cachedResponse {
	jsonCacheMu.Lock()
	defer jsonCacheMu.Unlock()

	if entry, ok := jsonCache[endpoint]; ok && entry.etag == etag {
		return entry
	}

	resp := generate()

	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	gz.Write(resp.Content)
	gz.Close()

	entry := &cachedResponse{
		etag:         resp.ETag,
		lastModified: resp.LastModified,
		raw:          resp.Content,
		gzipped:      buf.Bytes(),
	}
	jsonCache[endpoint] = entry
	return entry
}