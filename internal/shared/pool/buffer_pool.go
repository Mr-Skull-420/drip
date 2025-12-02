package pool

import "sync"

const (
	SizeSmall  = 4 * 1024    // 4KB
	SizeMedium = 32 * 1024   // 32KB
	SizeLarge  = 256 * 1024  // 256KB
)

type BufferPool struct {
	small  sync.Pool
	medium sync.Pool
	large  sync.Pool
}

func NewBufferPool() *BufferPool {
	return &BufferPool{
		small: sync.Pool{
			New: func() interface{} {
				b := make([]byte, SizeSmall)
				return &b
			},
		},
		medium: sync.Pool{
			New: func() interface{} {
				b := make([]byte, SizeMedium)
				return &b
			},
		},
		large: sync.Pool{
			New: func() interface{} {
				b := make([]byte, SizeLarge)
				return &b
			},
		},
	}
}

func (p *BufferPool) Get(size int) *[]byte {
	switch {
	case size <= SizeSmall:
		return p.small.Get().(*[]byte)
	case size <= SizeMedium:
		return p.medium.Get().(*[]byte)
	default:
		return p.large.Get().(*[]byte)
	}
}

func (p *BufferPool) Put(buf *[]byte) {
	if buf == nil {
		return
	}

	size := cap(*buf)
	*buf = (*buf)[:cap(*buf)]

	switch size {
	case SizeSmall:
		p.small.Put(buf)
	case SizeMedium:
		p.medium.Put(buf)
	case SizeLarge:
		p.large.Put(buf)
	}
}

var globalBufferPool = NewBufferPool()

func GetBuffer(size int) *[]byte {
	return globalBufferPool.Get(size)
}

func PutBuffer(buf *[]byte) {
	globalBufferPool.Put(buf)
}
