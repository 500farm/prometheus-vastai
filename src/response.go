package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

type CachedResponse struct {
	ts      time.Time
	etag    string
	raw     []byte
	gzipped []byte
}

func jsonHandler(w http.ResponseWriter, r *http.Request, cached *CachedResponse) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if cached == nil {
		http.NotFound(w, r)
		return
	}

	isHead := r.Method == http.MethodHead
	endpoint := r.URL.Path

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

	var body []byte
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") && len(cached.gzipped) > 0 {
		w.Header().Set("Content-Encoding", "gzip")
		body = cached.gzipped
	} else {
		body = cached.raw
	}

	size := len(body)
	w.Header().Set("Content-Length", strconv.Itoa(size))
	if !isHead {
		_, _ = w.Write(body)
	}

	if metrics != nil {
		metrics.ObserveServerResponse(endpoint, size)
	}
}