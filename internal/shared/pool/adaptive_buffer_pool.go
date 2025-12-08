package pool

import (
	"sync"
)

// AdaptiveBufferPool manages reusable buffers of different sizes
// This eliminates the massive memory allocation overhead seen in profiling
type AdaptiveBufferPool struct {
	// Large buffers for streaming threshold (1MB)
	largePool *sync.Pool

	// Medium buffers for temporary reads (32KB)
	mediumPool *sync.Pool
}

const (
	// LargeBufferSize is 1MB for streaming threshold
	LargeBufferSize = 1 * 1024 * 1024

	// MediumBufferSize is 32KB for temporary reads
	MediumBufferSize = 32 * 1024
)

// NewAdaptiveBufferPool creates a new adaptive buffer pool
func NewAdaptiveBufferPool() *AdaptiveBufferPool {
	return &AdaptiveBufferPool{
		largePool: &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, LargeBufferSize)
				return &buf
			},
		},
		mediumPool: &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, MediumBufferSize)
				return &buf
			},
		},
	}
}

// GetLarge returns a large buffer (1MB) from the pool
// The returned buffer should be returned via PutLarge when done
func (p *AdaptiveBufferPool) GetLarge() *[]byte {
	return p.largePool.Get().(*[]byte)
}

// PutLarge returns a large buffer to the pool for reuse
func (p *AdaptiveBufferPool) PutLarge(buf *[]byte) {
	if buf == nil {
		return
	}
	// Reset to full capacity to allow reuse
	*buf = (*buf)[:cap(*buf)]
	p.largePool.Put(buf)
}

// GetMedium returns a medium buffer (32KB) from the pool
// The returned buffer should be returned via PutMedium when done
func (p *AdaptiveBufferPool) GetMedium() *[]byte {
	return p.mediumPool.Get().(*[]byte)
}

// PutMedium returns a medium buffer to the pool for reuse
func (p *AdaptiveBufferPool) PutMedium(buf *[]byte) {
	if buf == nil {
		return
	}
	// Reset to full capacity to allow reuse
	*buf = (*buf)[:cap(*buf)]
	p.mediumPool.Put(buf)
}
