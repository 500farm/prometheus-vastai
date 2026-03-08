package main

import (
	"bytes"
	"runtime"
	"sync"

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

// Marshaler with preallocated buffers to avoid a lot of allocations when producing big JSON documents
type Marshaler struct {
	workerBufs []*bytes.Buffer
	rawBuf *FlipBuffer
	gzipBuf *FlipBuffer
}

const (
	initialWorkerBufSize = 1024 * 1024 * 8  // 8 MB per-worker scratch buffer
	initialRawBufSize    = 1024 * 1024 * 32 // 32 MB result buffer
	initialGzipBufSize   = 1024 * 1024 * 4  // 4 MB gzip buffer
)

func NewMarshaler() *Marshaler {
	nw := numWorkers()

	s := &Marshaler{
		workerBufs: make([]*bytes.Buffer, nw),
		rawBuf:     NewFlipBuffer(initialRawBufSize),
		gzipBuf:    NewFlipGzipBuffer(initialGzipBufSize, nw),
	}

	for i := range nw {
		s.workerBufs[i] = bytes.NewBuffer(make([]byte, 0, initialWorkerBufSize))
	}

	return s
}

// returned slices are owned by the marshaler's FlipBuffers and are valid until the next-but-one Marshal call
func (s *Marshaler) Marshal(v any) (raw []byte, gzipped []byte, err error) {
	rawBuf := s.rawBuf.Flip()

	// produce raw value
	enc := jsontext.NewEncoder(rawBuf,
		jsontext.WithIndent("    "),
		jsontext.AllowDuplicateNames(true),
	)
	if err := jsonv2.MarshalEncode(enc, v, jsonv2.Deterministic(true)); err != nil {
		return nil, nil, err
	}
	raw = rawBuf.Bytes()

	// produce gzipped value
	gzipWriter := s.gzipBuf.FlipGzip()
	gzipWriter.Write(raw)
	gzipWriter.Close()
	gzipped = s.gzipBuf.Bytes()

	return raw, gzipped, nil
}

type SerializableCollection struct {
	marshaler *Marshaler
	count     int
	get       func(i int) any
}

func (a *SerializableCollection) MarshalJSONTo(enc *jsontext.Encoder) error {
	count := a.count
	m := a.marshaler

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

	var firstErr error
	var errOnce sync.Once

	// phase 1: parallel encoding — each worker encodes its chunk of elements sequentially into its own buffer
	parallelDo(count, workers, func(w, start, end int) {
		buf := m.workerBufs[w]
		buf.Reset()
		enc := jsontext.NewEncoder(buf, jsontext.AllowDuplicateNames(true))

		for i := start; i < end; i++ {
			if err := jsonv2.MarshalEncode(enc, a.get(i), jsonv2.Deterministic(true)); err != nil {
				errOnce.Do(func() { firstErr = err })
				return
			}
		}
	})

	if firstErr != nil {
		return firstErr
	}

	// phase 2: sequential assembly — stream output from each worker buffer using a jsontext.Decoder to find value boundaries,
	// and feed them to the indenting encoder
	if err := enc.WriteToken(jsontext.BeginArray); err != nil {
		return err
	}

	for w := range workers {
		dec := jsontext.NewDecoder(m.workerBufs[w], jsontext.AllowDuplicateNames(true))
		for dec.PeekKind() != 0 {
			val, err := dec.ReadValue()
			if err != nil {
				return err
			}
			if err := enc.WriteValue(val); err != nil {
				return err
			}
		}
	}

	return enc.WriteToken(jsontext.EndArray)
}
