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
	if cached == nil {
		http.NotFound(w, r)
		return
	}

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

	written := 0
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") && len(cached.gzipped) > 0 {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Length", strconv.Itoa(len(cached.gzipped)))
		_, _ = w.Write(cached.gzipped)
		written = len(cached.gzipped)

	} else {
		w.Header().Set("Content-Length", strconv.Itoa(len(cached.raw)))
		_, _ = w.Write(cached.raw)
		written = len(cached.raw)
	}

	if metrics != nil {
		metrics.ObserveServerResponse(endpoint, written)
	}
}