package main

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"

	pgzip "github.com/klauspost/pgzip"

	jsonv2 "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

var numWorkers = func() int {
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

// shared buffers to reduce allocations
var jsonBufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 128*1024*1024))
	},
}

var elemBufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 8*1024))
	},
}

func jsonMarshalV2(v any) ([]byte, error) {
	jsonBuf := jsonBufPool.Get().(*bytes.Buffer)
	defer jsonBufPool.Put(jsonBuf)
	jsonBuf.Reset()

	enc := jsontext.NewEncoder(jsonBuf,
		jsontext.WithIndent("    "),
		jsontext.AllowDuplicateNames(true),
	)
	if err := jsonv2.MarshalEncode(enc, v, jsonv2.Deterministic(true)); err != nil {
		return nil, err
	}
	return append([]byte(nil), jsonBuf.Bytes()...), nil
}

func (s *VastAiOffers) MarshalJSONTo(enc *jsontext.Encoder) error {
	if s == nil {
		return marshalArray(enc, 0, nil)
	}
	offers := *s
	return marshalArray(enc, len(offers), func(i int) any {
		return offers[i].Raw
	})
}

func (s *VastAiMachineOffers) MarshalJSONTo(enc *jsontext.Encoder) error {
	if s == nil {
		return marshalArray(enc, 0, nil)
	}
	machines := *s
	return marshalArray(enc, len(machines), func(i int) any {
		return machines[i].asRaw()
	})
}

func marshalArray(enc *jsontext.Encoder, count int, get func(i int) any) error {
	if count == 0 {
		if err := enc.WriteToken(jsontext.BeginArray); err != nil {
			return err
		}
		return enc.WriteToken(jsontext.EndArray)
	}

	workers := 1
	if count >= 100 {
		workers = numWorkers()
	}

	type marshaledElement struct {
		data []byte
		err  error
	}

	elements := make([]marshaledElement, count)

	parallelDo(count, workers, func(w, start, end int) {
		buf := elemBufPool.Get().(*bytes.Buffer)
		defer elemBufPool.Put(buf)
		for i := start; i < end; i++ {
			buf.Reset()
			elemEnc := jsontext.NewEncoder(buf, jsontext.AllowDuplicateNames(true))
			err := jsonv2.MarshalEncode(elemEnc, get(i), jsonv2.Deterministic(true))
			elements[i] = marshaledElement{
				data: append([]byte(nil), buf.Bytes()...),
				err:  err,
			}
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

func (s *VastAiRawOffers) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	if tok.Kind() != '[' {
		return fmt.Errorf("expected '[', got %q", tok.Kind())
	}

	var rawElements []jsontext.Value
	for dec.PeekKind() != ']' {
		val, err := dec.ReadValue()
		if err != nil {
			return err
		}
		rawElements = append(rawElements, val)
	}

	if _, err := dec.ReadToken(); err != nil {
		return err
	}

	offers := make(VastAiRawOffers, len(rawElements))

	workers := 1
	if len(rawElements) >= 100 {
		workers = numWorkers()
	}

	var firstErr error
	var errOnce sync.Once

	parallelDo(len(rawElements), workers, func(w, start, end int) {
		for i := start; i < end; i++ {
			var m VastAiRawOffer
			if err := jsonv2.Unmarshal(rawElements[i], &m,
				jsontext.AllowDuplicateNames(true),
			); err != nil {
				errOnce.Do(func() { firstErr = err })
				return
			}
			offers[i] = m
		}
	})

	if firstErr != nil {
		return firstErr
	}

	*s = offers
	return nil
}

// shared buffers to reduce allocations
var gzipBufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 16*1024*1024))
	},
}

var gzipWriterPool = sync.Pool{
	New: func() any {
		buf := bytes.NewBuffer(nil)
		w, _ := pgzip.NewWriterLevel(buf, pgzip.DefaultCompression)
		w.SetConcurrency(1<<20, numWorkers())
		return w
	},
}

func gzip(data []byte) []byte {
	buf := gzipBufPool.Get().(*bytes.Buffer)
	defer gzipBufPool.Put(buf)
	buf.Reset()

	gz := gzipWriterPool.Get().(*pgzip.Writer)
	defer gzipWriterPool.Put(gz)
	gz.Reset(buf)

	gz.Write(data)
	gz.Close()
	return append([]byte(nil), buf.Bytes()...)
}
