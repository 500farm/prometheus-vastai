package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type JsonResponse struct {
	Content      []byte
	LastModified time.Time
	ETag         string
}

func computeETag(ts time.Time) string {
	hash := sha256.Sum256([]byte(ts.Format(time.RFC3339Nano)))
	return fmt.Sprintf(`"%x"`, hash[:8])
}

func jsonHandler(w http.ResponseWriter, r *http.Request, resp JsonResponse) {
	written := 0
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
	w.Header().Set("Last-Modified", resp.LastModified.UTC().Format(http.TimeFormat))
	w.Header().Set("ETag", resp.ETag)

	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == resp.ETag {
			w.WriteHeader(http.StatusNotModified)
			if metrics != nil {
				metrics.ObserveServerNotModified(r.URL.Path)
			}
			return
		}

	} else if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		if t, err := http.ParseTime(ims); err == nil && !resp.LastModified.Truncate(time.Second).After(t) {
			w.WriteHeader(http.StatusNotModified)
			if metrics != nil {
				metrics.ObserveServerNotModified(r.URL.Path)
			}
			return
		}
	}

	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		var buffer bytes.Buffer
		writer, _ := gzip.NewWriterLevel(&buffer, gzip.BestSpeed)
		writer.Write(resp.Content)
		writer.Close()
		gzipped := buffer.Bytes()

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Length", strconv.Itoa(len(gzipped)))
		w.Write(gzipped)
		written = len(gzipped)

	} else {
		w.Header().Set("Content-Length", strconv.Itoa(len(resp.Content)))
		w.Write(resp.Content)
		written = len(resp.Content)
	}

	if metrics != nil {
		endpoint := r.URL.Path
		metrics.ObserveServerResponse(endpoint, written)
	}
}
