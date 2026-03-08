package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"runtime"
	"sync"
)

func numWorkers() int {
	return min(runtime.NumCPU(), 16)
}

func parallelDo(total, workers int, fn func(w, start, end int)) {
	chunkSize := (total + workers - 1) / workers
	var wg sync.WaitGroup
	for w := range workers {
		start := w * chunkSize
		if start >= total {
			break
		}
		end := min(start+chunkSize, total)
		wg.Add(1)
		go func(w, start, end int) {
			defer wg.Done()
			fn(w, start, end)
		}(w, start, end)
	}
	wg.Wait()
}

func (s VastAiRawOffers) MarshalJSON() ([]byte, error) {
	if len(s) == 0 {
		return []byte("[]"), nil
	}

	workers := 1
	if len(s) >= 100 {
		workers = numWorkers()
	}

	chunks := make([][]byte, workers)
	errs := make([]error, workers)

	parallelDo(len(s), workers, func(w, start, end int) {
		var buf bytes.Buffer
		for i := start; i < end; i++ {
			if i > start {
				buf.WriteByte(',')
			}
			b, err := json.Marshal(s[i])
			if err != nil {
				errs[w] = err
				return
			}
			buf.Write(b)
		}
		chunks[w] = buf.Bytes()
	})

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	inner := bytes.Join(chunks, []byte{','})
	out := make([]byte, 0, len(inner)+2)
	out = append(out, '[')
	out = append(out, inner...)
	out = append(out, ']')
	return out, nil
}

// Gzip streams are concatenable per RFC 1952
func parallelGzip(data []byte) []byte {
	if len(data) == 0 {
		return emptyGzip()
	}

	workers := 1
	if len(data) >= 100*1024 {
		workers = numWorkers()
	}

	chunks := make([][]byte, workers)

	parallelDo(len(data), workers, func(w, start, end int) {
		var buf bytes.Buffer
		gz, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
		gz.Write(data[start:end])
		gz.Close()
		chunks[w] = buf.Bytes()
	})

	out := bytes.Join(chunks, nil)
	if len(out) == 0 {
		var buf bytes.Buffer
		gz, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
		gz.Close()
		return buf.Bytes()
	}
	return out
}

func emptyGzip() []byte {
	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	gz.Close()
	return buf.Bytes()
}
