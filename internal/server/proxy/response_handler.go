package proxy

import (
	"sync"
	"time"

	"drip/internal/shared/protocol"
	"go.uber.org/zap"
)

// responseChanEntry holds a response channel and its creation time
type responseChanEntry struct {
	ch        chan *protocol.HTTPResponse
	createdAt time.Time
}

// ResponseHandler manages response channels for HTTP requests over TCP/Frame protocol
type ResponseHandler struct {
	channels map[string]*responseChanEntry
	mu       sync.RWMutex
	logger   *zap.Logger
	stopCh   chan struct{}
}

// NewResponseHandler creates a new response handler
func NewResponseHandler(logger *zap.Logger) *ResponseHandler {
	h := &ResponseHandler{
		channels: make(map[string]*responseChanEntry),
		logger:   logger,
		stopCh:   make(chan struct{}),
	}

	// Start single cleanup goroutine instead of one per request
	go h.cleanupLoop()

	return h
}

// CreateResponseChan creates a response channel for a request ID
func (h *ResponseHandler) CreateResponseChan(requestID string) chan *protocol.HTTPResponse {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan *protocol.HTTPResponse, 1)
	h.channels[requestID] = &responseChanEntry{
		ch:        ch,
		createdAt: time.Now(),
	}

	return ch
}

// GetResponseChan gets the response channel for a request ID
func (h *ResponseHandler) GetResponseChan(requestID string) <-chan *protocol.HTTPResponse {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if entry := h.channels[requestID]; entry != nil {
		return entry.ch
	}
	return nil
}

// SendResponse sends a response to the waiting channel
func (h *ResponseHandler) SendResponse(requestID string, resp *protocol.HTTPResponse) {
	h.mu.RLock()
	entry, exists := h.channels[requestID]
	h.mu.RUnlock()

	if !exists || entry == nil {
		h.logger.Warn("Response channel not found",
			zap.String("request_id", requestID),
		)
		return
	}

	select {
	case entry.ch <- resp:
		h.logger.Debug("Response sent to channel",
			zap.String("request_id", requestID),
		)
	case <-time.After(5 * time.Second):
		h.logger.Warn("Timeout sending response to channel",
			zap.String("request_id", requestID),
		)
	}
}

// CleanupResponseChan removes and closes a response channel
func (h *ResponseHandler) CleanupResponseChan(requestID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if entry, exists := h.channels[requestID]; exists {
		close(entry.ch)
		delete(h.channels, requestID)
	}
}

// GetPendingCount returns the number of pending responses
func (h *ResponseHandler) GetPendingCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.channels)
}

// cleanupLoop periodically cleans up expired response channels
// This replaces the per-request goroutine approach with a single cleanup goroutine
func (h *ResponseHandler) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.cleanupExpiredChannels()
		case <-h.stopCh:
			return
		}
	}
}

// cleanupExpiredChannels removes channels older than 30 seconds
func (h *ResponseHandler) cleanupExpiredChannels() {
	now := time.Now()
	timeout := 30 * time.Second

	h.mu.Lock()
	defer h.mu.Unlock()

	expiredCount := 0
	for requestID, entry := range h.channels {
		if now.Sub(entry.createdAt) > timeout {
			close(entry.ch)
			delete(h.channels, requestID)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		h.logger.Debug("Cleaned up expired response channels",
			zap.Int("count", expiredCount),
			zap.Int("remaining", len(h.channels)),
		)
	}
}

// Close stops the cleanup loop
func (h *ResponseHandler) Close() {
	close(h.stopCh)

	h.mu.Lock()
	defer h.mu.Unlock()

	// Close all remaining channels
	for _, entry := range h.channels {
		close(entry.ch)
	}
	h.channels = make(map[string]*responseChanEntry)
}
