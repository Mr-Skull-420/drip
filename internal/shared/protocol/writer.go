package protocol

import (
	"errors"
	"io"
	"sync"
	"time"
)

type FrameWriter struct {
	conn   io.Writer
	queue  chan *Frame
	batch  []*Frame
	mu     sync.Mutex
	done   chan struct{}
	closed bool

	maxBatch     int
	maxBatchWait time.Duration
}

func NewFrameWriter(conn io.Writer) *FrameWriter {
	return NewFrameWriterWithConfig(conn, 128, 2*time.Millisecond, 1024)
}

func NewFrameWriterWithConfig(conn io.Writer, maxBatch int, maxBatchWait time.Duration, queueSize int) *FrameWriter {
	w := &FrameWriter{
		conn:         conn,
		queue:        make(chan *Frame, queueSize),
		batch:        make([]*Frame, 0, maxBatch),
		maxBatch:     maxBatch,
		maxBatchWait: maxBatchWait,
		done:         make(chan struct{}),
	}
	go w.writeLoop()
	return w
}

func (w *FrameWriter) writeLoop() {
	ticker := time.NewTicker(w.maxBatchWait)
	defer ticker.Stop()

	for {
		select {
		case frame, ok := <-w.queue:
			if !ok {
				w.mu.Lock()
				w.flushBatchLocked()
				w.mu.Unlock()
				return
			}

			w.mu.Lock()
			w.batch = append(w.batch, frame)

			if len(w.batch) >= w.maxBatch {
				w.flushBatchLocked()
			}
			w.mu.Unlock()

		case <-ticker.C:
			w.mu.Lock()
			if len(w.batch) > 0 {
				w.flushBatchLocked()
			}
			w.mu.Unlock()

		case <-w.done:
			w.mu.Lock()
			w.flushBatchLocked()
			w.mu.Unlock()
			return
		}
	}
}

func (w *FrameWriter) flushBatchLocked() {
	if len(w.batch) == 0 {
		return
	}

	for _, frame := range w.batch {
		_ = WriteFrame(w.conn, frame)
	}

	w.batch = w.batch[:0]
}

func (w *FrameWriter) WriteFrame(frame *Frame) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return errors.New("writer closed")
	}
	w.mu.Unlock()

	select {
	case w.queue <- frame:
		return nil
	case <-w.done:
		return errors.New("writer closed")
	default:
		return WriteFrame(w.conn, frame)
	}
}

func (w *FrameWriter) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()

	close(w.queue)
	close(w.done)

	return nil
}

func (w *FrameWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushBatchLocked()
}
