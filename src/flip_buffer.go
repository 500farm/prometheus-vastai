package main

import (
	"bytes"

	pgzip "github.com/klauspost/pgzip"
)

// double-buffered bytes.Buffer with optional pre-bound gzip writers
type FlipBuffer struct {
	bufs    [2]*bytes.Buffer
	writers [2]*pgzip.Writer // nil for plain (non-gzip) buffers
	cur     int
}

func NewFlipBuffer(initialSize int) *FlipBuffer {
	return &FlipBuffer{
		bufs: [2]*bytes.Buffer{
			bytes.NewBuffer(make([]byte, 0, initialSize)),
			bytes.NewBuffer(make([]byte, 0, initialSize)),
		},
	}
}

func NewFlipGzipBuffer(initialSize int, concurrency int) *FlipBuffer {
	f := &FlipBuffer{
		bufs: [2]*bytes.Buffer{
			bytes.NewBuffer(make([]byte, 0, initialSize)),
			bytes.NewBuffer(make([]byte, 0, initialSize)),
		},
	}
	for i := range 2 {
		w, _ := pgzip.NewWriterLevel(f.bufs[i], pgzip.DefaultCompression)
		w.SetConcurrency(1<<20, concurrency)
		f.writers[i] = w
	}
	return f
}

// switches to the inactive buffer, resets it, and returns it ready for writing
func (f *FlipBuffer) Flip() *bytes.Buffer {
	f.cur ^= 1
	f.bufs[f.cur].Reset()
	return f.bufs[f.cur]
}

// switches to the inactive buffer, resets it, resets the bound gzip writer, and returns the writer ready for use
func (f *FlipBuffer) FlipGzip() *pgzip.Writer {
	f.cur ^= 1
	f.bufs[f.cur].Reset()
	f.writers[f.cur].Reset(f.bufs[f.cur])
	return f.writers[f.cur]
}

// returns the contents of the current buffer
func (f *FlipBuffer) Bytes() []byte {
	return f.bufs[f.cur].Bytes()
}

// returns total capacity of both underlying buffers in bytes
func (f *FlipBuffer) Cap() int {
	return f.bufs[0].Cap() + f.bufs[1].Cap()
}
