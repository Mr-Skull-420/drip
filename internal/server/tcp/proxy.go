package tcp

import (
	"fmt"
	"net"
	"sync"
	"time"

	"drip/internal/shared/pool"
	"drip/internal/shared/protocol"
	"go.uber.org/zap"
)

// TunnelProxy handles TCP connections for a specific tunnel
type TunnelProxy struct {
	port        int
	subdomain   string
	tcpConn     net.Conn // The tunnel control connection
	listener    net.Listener
	logger      *zap.Logger
	stopCh      chan struct{}
	wg          sync.WaitGroup
	clientAddr  string
	streams     map[string]*proxyStream // streamID -> stream info
	streamMu    sync.RWMutex
	frameWriter *protocol.FrameWriter
	bufferPool  *pool.BufferPool
}

// proxyStream holds connection info with close state
type proxyStream struct {
	conn   net.Conn
	closed bool
	mu     sync.Mutex
}

// NewTunnelProxy creates a new TCP tunnel proxy
func NewTunnelProxy(port int, subdomain string, tcpConn net.Conn, logger *zap.Logger) *TunnelProxy {
	return &TunnelProxy{
		port:        port,
		subdomain:   subdomain,
		tcpConn:     tcpConn,
		logger:      logger,
		stopCh:      make(chan struct{}),
		clientAddr:  tcpConn.RemoteAddr().String(),
		streams:     make(map[string]*proxyStream),
		bufferPool:  pool.NewBufferPool(),
		frameWriter: protocol.NewFrameWriter(tcpConn),
	}
}

// Start starts listening on the allocated port
func (p *TunnelProxy) Start() error {
	addr := fmt.Sprintf("0.0.0.0:%d", p.port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", p.port, err)
	}

	p.listener = listener

	p.logger.Info("TCP proxy started",
		zap.Int("port", p.port),
		zap.String("subdomain", p.subdomain),
	)

	p.wg.Add(1)
	go p.acceptLoop()

	return nil
}

// acceptLoop accepts incoming TCP connections
func (p *TunnelProxy) acceptLoop() {
	defer p.wg.Done()

	for {
		select {
		case <-p.stopCh:
			return
		default:
		}

		p.listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))

		conn, err := p.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-p.stopCh:
				return
			default:
				continue
			}
		}

		p.wg.Add(1)
		go p.handleConnection(conn)
	}
}

func (p *TunnelProxy) handleConnection(conn net.Conn) {
	defer p.wg.Done()
	defer conn.Close()

	streamID := fmt.Sprintf("%d-%d", time.Now().UnixNano(), p.port)

	stream := &proxyStream{
		conn:   conn,
		closed: false,
	}

	p.streamMu.Lock()
	p.streams[streamID] = stream
	p.streamMu.Unlock()

	defer func() {
		p.streamMu.Lock()
		delete(p.streams, streamID)
		p.streamMu.Unlock()
	}()

	bufPtr := p.bufferPool.Get(pool.SizeMedium)
	defer p.bufferPool.Put(bufPtr)

	buffer := (*bufPtr)[:pool.SizeMedium]

	for {
		// Check if stream is closed
		stream.mu.Lock()
		closed := stream.closed
		stream.mu.Unlock()
		if closed {
			break
		}

		n, err := conn.Read(buffer)
		if err != nil {
			break
		}

		if n > 0 {
			if err := p.sendDataToTunnel(streamID, buffer[:n]); err != nil {
				p.logger.Debug("Send to tunnel failed", zap.Error(err))
				break
			}
		}
	}

	select {
	case <-p.stopCh:
	default:
		p.sendCloseToTunnel(streamID)
	}
}

func (p *TunnelProxy) sendDataToTunnel(streamID string, data []byte) error {
	select {
	case <-p.stopCh:
		return fmt.Errorf("tunnel proxy stopped")
	default:
	}

	header := protocol.DataHeader{
		StreamID:  streamID,
		RequestID: streamID,
		Type:      protocol.DataTypeData,
		IsLast:    false,
	}

	payload, poolBuffer, err := protocol.EncodeDataPayloadPooled(header, data)
	if err != nil {
		return fmt.Errorf("failed to encode payload: %w", err)
	}

	frame := protocol.NewFramePooled(protocol.FrameTypeData, payload, poolBuffer)

	err = p.frameWriter.WriteFrame(frame)
	if err != nil {
		return fmt.Errorf("failed to write frame: %w", err)
	}

	return nil
}

func (p *TunnelProxy) sendCloseToTunnel(streamID string) {
	header := protocol.DataHeader{
		StreamID:  streamID,
		RequestID: streamID,
		Type:      protocol.DataTypeClose,
		IsLast:    true,
	}

	payload, poolBuffer, err := protocol.EncodeDataPayloadPooled(header, nil)
	if err != nil {
		return
	}

	frame := protocol.NewFramePooled(protocol.FrameTypeData, payload, poolBuffer)
	p.frameWriter.WriteFrame(frame)
}

func (p *TunnelProxy) HandleResponse(streamID string, data []byte) error {
	p.streamMu.RLock()
	stream, ok := p.streams[streamID]
	p.streamMu.RUnlock()

	if !ok {
		// Stream may have been closed by client, this is normal
		return nil
	}

	// Check if stream is closed
	stream.mu.Lock()
	if stream.closed {
		stream.mu.Unlock()
		return nil
	}
	stream.mu.Unlock()

	if _, err := stream.conn.Write(data); err != nil {
		p.logger.Debug("Write to client failed", zap.Error(err))
		return err
	}

	return nil
}

// CloseStream closes a stream
func (p *TunnelProxy) CloseStream(streamID string) {
	p.streamMu.RLock()
	stream, ok := p.streams[streamID]
	p.streamMu.RUnlock()

	if !ok {
		return
	}

	// Mark as closed first
	stream.mu.Lock()
	if stream.closed {
		stream.mu.Unlock()
		return
	}
	stream.closed = true
	stream.mu.Unlock()

	// Now close the connection
	stream.conn.Close()
}

func (p *TunnelProxy) Stop() {
	p.logger.Info("Stopping TCP proxy",
		zap.Int("port", p.port),
		zap.String("subdomain", p.subdomain),
	)

	close(p.stopCh)

	if p.listener != nil {
		p.listener.Close()
	}

	p.streamMu.Lock()
	for _, stream := range p.streams {
		stream.mu.Lock()
		stream.closed = true
		stream.mu.Unlock()
		stream.conn.Close()
	}
	p.streams = make(map[string]*proxyStream)
	p.streamMu.Unlock()

	p.wg.Wait()

	if p.frameWriter != nil {
		p.frameWriter.Close()
	}

	p.logger.Info("TCP proxy stopped", zap.Int("port", p.port))
}
