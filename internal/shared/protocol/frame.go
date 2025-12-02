package protocol

import (
	"encoding/binary"
	"fmt"
	"io"

	"drip/internal/shared/pool"
)

const (
	FrameHeaderSize = 5
	MaxFrameSize    = 10 * 1024 * 1024
)

// FrameType defines the type of frame
type FrameType byte

const (
	FrameTypeRegister     FrameType = 0x01
	FrameTypeRegisterAck  FrameType = 0x02
	FrameTypeHeartbeat    FrameType = 0x03
	FrameTypeHeartbeatAck FrameType = 0x04
	FrameTypeData         FrameType = 0x05
	FrameTypeClose        FrameType = 0x06
	FrameTypeError        FrameType = 0x07
)

// String returns the string representation of frame type
func (t FrameType) String() string {
	switch t {
	case FrameTypeRegister:
		return "Register"
	case FrameTypeRegisterAck:
		return "RegisterAck"
	case FrameTypeHeartbeat:
		return "Heartbeat"
	case FrameTypeHeartbeatAck:
		return "HeartbeatAck"
	case FrameTypeData:
		return "Data"
	case FrameTypeClose:
		return "Close"
	case FrameTypeError:
		return "Error"
	default:
		return fmt.Sprintf("Unknown(%d)", t)
	}
}

type Frame struct {
	Type       FrameType
	Payload    []byte
	poolBuffer *[]byte
}

func WriteFrame(w io.Writer, frame *Frame) error {
	payloadLen := len(frame.Payload)
	if payloadLen > MaxFrameSize {
		return fmt.Errorf("payload too large: %d bytes (max %d)", payloadLen, MaxFrameSize)
	}

	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(payloadLen))
	if _, err := w.Write(lengthBuf); err != nil {
		return fmt.Errorf("failed to write length: %w", err)
	}

	if _, err := w.Write([]byte{byte(frame.Type)}); err != nil {
		return fmt.Errorf("failed to write type: %w", err)
	}

	if payloadLen > 0 {
		if _, err := w.Write(frame.Payload); err != nil {
			return fmt.Errorf("failed to write payload: %w", err)
		}
	}

	return nil
}

func ReadFrame(r io.Reader) (*Frame, error) {
	header := make([]byte, FrameHeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("failed to read frame header: %w", err)
	}

	payloadLen := binary.BigEndian.Uint32(header[0:4])
	if payloadLen > MaxFrameSize {
		return nil, fmt.Errorf("payload too large: %d bytes (max %d)", payloadLen, MaxFrameSize)
	}

	frameType := FrameType(header[4])

	var payload []byte
	var poolBuf *[]byte

	if payloadLen > 0 {
		if payloadLen > pool.SizeLarge {
			payload = make([]byte, payloadLen)
			if _, err := io.ReadFull(r, payload); err != nil {
				return nil, fmt.Errorf("failed to read payload: %w", err)
			}
		} else {
			poolBuf = pool.GetBuffer(int(payloadLen))
			payload = (*poolBuf)[:payloadLen]

			if _, err := io.ReadFull(r, payload); err != nil {
				pool.PutBuffer(poolBuf)
				return nil, fmt.Errorf("failed to read payload: %w", err)
			}
		}
	}

	return &Frame{
		Type:       frameType,
		Payload:    payload,
		poolBuffer: poolBuf,
	}, nil
}

func (f *Frame) Release() {
	if f.poolBuffer != nil {
		pool.PutBuffer(f.poolBuffer)
		f.poolBuffer = nil
		f.Payload = nil
	}
}

// NewFrame creates a new frame
func NewFrame(frameType FrameType, payload []byte) *Frame {
	return &Frame{
		Type:    frameType,
		Payload: payload,
	}
}
