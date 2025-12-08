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

	heartbeatInterval time.Duration
	heartbeatCallback func() *Frame
	heartbeatEnabled  bool
	heartbeatControl  chan struct{}

	// Error handling
	writeErr      error
	errOnce       sync.Once
	onWriteError  func(error) // Callback for write errors

	// Adaptive flushing
	adaptiveFlush       bool // Enable adaptive flush based on queue depth
	lowConcurrencyThreshold int // Queue depth threshold for immediate flush
}

func NewFrameWriter(conn io.Writer) *FrameWriter {
	w := NewFrameWriterWithConfig(conn, 256, 2*time.Millisecond, 4096)
	w.EnableAdaptiveFlush(16)
	return w
}

func NewFrameWriterWithConfig(conn io.Writer, maxBatch int, maxBatchWait time.Duration, queueSize int) *FrameWriter {
	w := &FrameWriter{
		conn:             conn,
		queue:            make(chan *Frame, queueSize),
		batch:            make([]*Frame, 0, maxBatch),
		maxBatch:         maxBatch,
		maxBatchWait:     maxBatchWait,
		done:             make(chan struct{}),
		heartbeatControl: make(chan struct{}, 1),
	}
	go w.writeLoop()
	return w
}

func (w *FrameWriter) writeLoop() {
	batchTicker := time.NewTicker(w.maxBatchWait)
	defer batchTicker.Stop()

	var heartbeatTicker *time.Ticker
	var heartbeatCh <-chan time.Time

	w.mu.Lock()
	if w.heartbeatEnabled && w.heartbeatInterval > 0 {
		heartbeatTicker = time.NewTicker(w.heartbeatInterval)
		heartbeatCh = heartbeatTicker.C
	}
	w.mu.Unlock()

	defer func() {
		if heartbeatTicker != nil {
			heartbeatTicker.Stop()
		}
	}()

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

			shouldFlushNow := len(w.batch) >= w.maxBatch ||
				(w.adaptiveFlush && len(w.queue) <= w.lowConcurrencyThreshold)

			if shouldFlushNow {
				w.flushBatchLocked()
			}
			w.mu.Unlock()

		case <-batchTicker.C:
			w.mu.Lock()
			if len(w.batch) > 0 {
				w.flushBatchLocked()
			}
			w.mu.Unlock()

		case <-heartbeatCh:
			w.mu.Lock()
			if w.heartbeatCallback != nil {
				if frame := w.heartbeatCallback(); frame != nil {
					w.batch = append(w.batch, frame)
					w.flushBatchLocked()
				}
			}
			w.mu.Unlock()

		case <-w.heartbeatControl:
			w.mu.Lock()
			if heartbeatTicker != nil {
				heartbeatTicker.Stop()
				heartbeatTicker = nil
				heartbeatCh = nil
			}
			if w.heartbeatEnabled && w.heartbeatInterval > 0 {
				heartbeatTicker = time.NewTicker(w.heartbeatInterval)
				heartbeatCh = heartbeatTicker.C
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
		if err := WriteFrame(w.conn, frame); err != nil {
			w.errOnce.Do(func() {
				w.writeErr = err
				if w.onWriteError != nil {
					go w.onWriteError(err)
				}
				w.closed = true
			})
		}
		frame.Release()
	}

	w.batch = w.batch[:0]
}

func (w *FrameWriter) WriteFrame(frame *Frame) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		if w.writeErr != nil {
			return w.writeErr
		}
		return errors.New("writer closed")
	}
	w.mu.Unlock()

	select {
	case w.queue <- frame:
		return nil
	case <-w.done:
		w.mu.Lock()
		err := w.writeErr
		w.mu.Unlock()
		if err != nil {
			return err
		}
		return errors.New("writer closed")
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

	for frame := range w.queue {
		frame.Release()
	}

	close(w.done)

	return nil
}

func (w *FrameWriter) Flush() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}

	for {
		select {
		case frame, ok := <-w.queue:
			if !ok {
				break
			}
			w.batch = append(w.batch, frame)
		default:
			goto done
		}
	}
done:
	w.flushBatchLocked()
	w.mu.Unlock()
}

func (w *FrameWriter) EnableHeartbeat(interval time.Duration, callback func() *Frame) {
	w.mu.Lock()
	w.heartbeatInterval = interval
	w.heartbeatCallback = callback
	w.heartbeatEnabled = true
	w.mu.Unlock()

	select {
	case w.heartbeatControl <- struct{}{}:
	default:
	}
}

func (w *FrameWriter) DisableHeartbeat() {
	w.mu.Lock()
	w.heartbeatEnabled = false
	w.mu.Unlock()

	select {
	case w.heartbeatControl <- struct{}{}:
	default:
	}
}

func (w *FrameWriter) SetWriteErrorHandler(handler func(error)) {
	w.mu.Lock()
	w.onWriteError = handler
	w.mu.Unlock()
}

func (w *FrameWriter) EnableAdaptiveFlush(lowConcurrencyThreshold int) {
	w.mu.Lock()
	w.adaptiveFlush = true
	w.lowConcurrencyThreshold = lowConcurrencyThreshold
	w.mu.Unlock()
}

func (w *FrameWriter) DisableAdaptiveFlush() {
	w.mu.Lock()
	w.adaptiveFlush = false
	w.mu.Unlock()
}
