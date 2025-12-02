package tunnel

import (
	"sync"
	"time"

	"drip/internal/shared/protocol"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Transport represents the control channel to the client.
// It is implemented by the TCP control connection so the HTTP proxy
// can push frames directly to the client without depending on WebSockets.
type Transport interface {
	SendFrame(frame *protocol.Frame) error
}

// Connection represents a tunnel connection from a client
type Connection struct {
	Subdomain  string
	Conn       *websocket.Conn
	SendCh     chan []byte
	CloseCh    chan struct{}
	LastActive time.Time
	mu         sync.RWMutex
	logger     *zap.Logger
	closed     bool
	transport  Transport
	tunnelType protocol.TunnelType
}

// NewConnection creates a new tunnel connection
func NewConnection(subdomain string, conn *websocket.Conn, logger *zap.Logger) *Connection {
	return &Connection{
		Subdomain:  subdomain,
		Conn:       conn,
		SendCh:     make(chan []byte, 256),
		CloseCh:    make(chan struct{}),
		LastActive: time.Now(),
		logger:     logger,
		closed:     false,
	}
}

// Send sends data through the WebSocket connection
func (c *Connection) Send(data []byte) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrConnectionClosed
	}
	c.mu.RUnlock()

	select {
	case c.SendCh <- data:
		return nil
	case <-time.After(5 * time.Second):
		return ErrSendTimeout
	}
}

// UpdateActivity updates the last activity timestamp
func (c *Connection) UpdateActivity() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LastActive = time.Now()
}

// IsAlive checks if the connection is still alive based on last activity
func (c *Connection) IsAlive(timeout time.Duration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.LastActive) < timeout
}

// Close closes the connection and all associated channels
func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	c.closed = true
	close(c.CloseCh)
	close(c.SendCh)

	if c.Conn != nil {
		// Send close message
		c.Conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.Conn.Close()
	}

	c.logger.Info("Connection closed",
		zap.String("subdomain", c.Subdomain),
	)
}

// IsClosed returns whether the connection is closed
func (c *Connection) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// SetTransport attaches the control transport and tunnel type.
func (c *Connection) SetTransport(t Transport, tType protocol.TunnelType) {
	c.mu.Lock()
	c.transport = t
	c.tunnelType = tType
	c.mu.Unlock()
}

// GetTransport returns the attached transport (if any).
func (c *Connection) GetTransport() Transport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.transport
}

// SetTunnelType sets the tunnel type.
func (c *Connection) SetTunnelType(tType protocol.TunnelType) {
	c.mu.Lock()
	c.tunnelType = tType
	c.mu.Unlock()
}

// GetTunnelType returns the tunnel type.
func (c *Connection) GetTunnelType() protocol.TunnelType {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tunnelType
}

// StartWritePump starts the write pump for sending messages
func (c *Connection) StartWritePump() {
	// Skip write pump for TCP-only connections (no WebSocket)
	if c.Conn == nil {
		c.logger.Debug("Skipping WritePump for TCP connection",
			zap.String("subdomain", c.Subdomain),
		)
		// Still need to drain SendCh to prevent blocking
		go func() {
			for {
				select {
				case <-c.SendCh:
					// Discard messages for TCP mode
				case <-c.CloseCh:
					return
				}
			}
		}()
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message, ok := <-c.SendCh:
			if !ok {
				return
			}

			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.logger.Error("Write error",
					zap.String("subdomain", c.Subdomain),
					zap.Error(err),
				)
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.CloseCh:
			return
		}
	}
}
