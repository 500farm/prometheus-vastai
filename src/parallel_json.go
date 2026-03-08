package main

import (
	"bytes"
	"compress/gzip"
	"runtime"
	"sync"

	jsonv2 "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
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

func jsonMarshalV2(v any) ([]byte, error) {
	return jsonv2.Marshal(v,
		jsontext.WithIndent("    "),
		jsonv2.Deterministic(true),
		jsontext.AllowDuplicateNames(true),
	)
}

func (s *VastAiRawOffers) MarshalJSONTo(enc *jsontext.Encoder) error {
	offers := *s

	if s == nil || len(offers) == 0 {
		if err := enc.WriteToken(jsontext.BeginArray); err != nil {
			return err
		}
		return enc.WriteToken(jsontext.EndArray)
	}

	workers := 1
	if len(offers) >= 100 {
		workers = numWorkers()
	}

	type marshaledElement struct {
		data []byte
		err  error
	}

	elements := make([]marshaledElement, len(offers))

	parallelDo(len(offers), workers, func(w, start, end int) {
		for i := start; i < end; i++ {
			data, err := jsonv2.Marshal(offers[i],
				jsonv2.Deterministic(true),
				jsontext.AllowDuplicateNames(true),
			)
			elements[i] = marshaledElement{data: data, err: err}
		}
	})

	if err := enc.WriteToken(jsontext.BeginArray); err != nil {
		return err
	}

	for i := range elements {
		if elements[i].err != nil {
			return elements[i].err
		}
		if err := enc.WriteValue(elements[i].data); err != nil {
			return err
		}
	}

	return enc.WriteToken(jsontext.EndArray)
}

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
		return emptyGzip()
	}
	return out
}

func emptyGzip() []byte {
	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	gz.Close()
	return buf.Bytes()
}
