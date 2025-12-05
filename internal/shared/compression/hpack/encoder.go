package hpack

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

const (
	// DefaultDynamicTableSize is the default size of the dynamic table (4KB)
	DefaultDynamicTableSize = 4096

	// IndexedHeaderField represents a fully indexed header field
	indexedHeaderField = 0x80 // 10xxxxxx

	// LiteralHeaderFieldWithIndexing represents a literal with incremental indexing
	literalHeaderFieldWithIndexing = 0x40 // 01xxxxxx
)

// Encoder compresses HTTP headers using HPACK
// Each connection MUST have its own encoder instance to avoid state corruption
type Encoder struct {
	mu            sync.Mutex
	dynamicTable  *DynamicTable
	staticTable   *StaticTable
	maxTableSize  uint32
}

// NewEncoder creates a new HPACK encoder with the specified dynamic table size
// This encoder is NOT thread-safe and should be used by a single connection
func NewEncoder(maxTableSize uint32) *Encoder {
	if maxTableSize == 0 {
		maxTableSize = DefaultDynamicTableSize
	}

	return &Encoder{
		dynamicTable: NewDynamicTable(maxTableSize),
		staticTable:  GetStaticTable(),
		maxTableSize: maxTableSize,
	}
}

// Encode encodes HTTP headers into HPACK binary format
// This method is safe to call concurrently within the same encoder instance
func (e *Encoder) Encode(headers http.Header) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if headers == nil {
		return nil, errors.New("headers cannot be nil")
	}

	buf := &bytes.Buffer{}

	for name, values := range headers {
		for _, value := range values {
			if err := e.encodeHeaderField(buf, name, value); err != nil {
				return nil, fmt.Errorf("encode header %s: %w", name, err)
			}
		}
	}

	return buf.Bytes(), nil
}

// encodeHeaderField encodes a single header field
func (e *Encoder) encodeHeaderField(buf *bytes.Buffer, name, value string) error {
	// HTTP/2 requires header names to be lowercase (RFC 7540 Section 8.1.2)
	// Convert to lowercase for table lookups and storage
	nameLower := strings.ToLower(name)

	// Try to find in static table first
	if index, found := e.staticTable.FindExact(nameLower, value); found {
		return e.writeIndexedHeader(buf, index+1)
	}

	// Check if name exists in static table (for literal with name reference)
	if index, found := e.staticTable.FindName(nameLower); found {
		return e.writeLiteralWithIndexing(buf, index+1, value, true)
	}

	// Try dynamic table
	if index, found := e.dynamicTable.FindExact(nameLower, value); found {
		// Dynamic table indices start after static table
		dynamicIndex := uint32(e.staticTable.Size()) + index + 1
		return e.writeIndexedHeader(buf, dynamicIndex)
	}

	if index, found := e.dynamicTable.FindName(nameLower); found {
		dynamicIndex := uint32(e.staticTable.Size()) + index + 1
		return e.writeLiteralWithIndexing(buf, dynamicIndex, value, true)
	}

	// Not found anywhere - literal with indexing and new name
	// Write literal flag
	buf.WriteByte(literalHeaderFieldWithIndexing)

	// Write name as literal string (must come before value)
	// Use lowercase name for consistency
	if err := e.writeString(buf, nameLower, false); err != nil {
		return err
	}

	// Write value as literal string
	if err := e.writeString(buf, value, false); err != nil {
		return err
	}

	// Add to dynamic table with lowercase name
	e.dynamicTable.Add(nameLower, value)

	return nil
}

// writeIndexedHeader writes an indexed header field (10xxxxxx)
func (e *Encoder) writeIndexedHeader(buf *bytes.Buffer, index uint32) error {
	return e.writeInteger(buf, index, 7, indexedHeaderField)
}

// writeLiteralWithIndexing writes a literal header with incremental indexing (01xxxxxx)
func (e *Encoder) writeLiteralWithIndexing(buf *bytes.Buffer, nameIndex uint32, value string, hasIndex bool) error {
	if hasIndex {
		// Write name as index
		if err := e.writeInteger(buf, nameIndex, 6, literalHeaderFieldWithIndexing); err != nil {
			return err
		}
	} else {
		// Write literal flag
		buf.WriteByte(literalHeaderFieldWithIndexing)
	}

	// Write value as literal string
	return e.writeString(buf, value, false)
}

// writeInteger writes an integer using HPACK integer representation
func (e *Encoder) writeInteger(buf *bytes.Buffer, value uint32, prefixBits int, prefix byte) error {
	if prefixBits < 1 || prefixBits > 8 {
		return fmt.Errorf("invalid prefix bits: %d", prefixBits)
	}

	maxPrefix := uint32((1 << prefixBits) - 1)

	if value < maxPrefix {
		buf.WriteByte(prefix | byte(value))
		return nil
	}

	// Value >= maxPrefix, need multiple bytes
	buf.WriteByte(prefix | byte(maxPrefix))
	value -= maxPrefix

	for value >= 128 {
		buf.WriteByte(byte(value%128) | 0x80)
		value /= 128
	}
	buf.WriteByte(byte(value))

	return nil
}

// writeString writes a string using HPACK string representation
func (e *Encoder) writeString(buf *bytes.Buffer, str string, huffmanEncode bool) error {
	// For simplicity, we don't use Huffman encoding in this implementation
	// Huffman flag is bit 7, followed by length in remaining 7 bits

	length := uint32(len(str))
	if huffmanEncode {
		// TODO: Implement Huffman encoding if needed
		return errors.New("huffman encoding not implemented")
	}

	// Write length with H=0 (no Huffman)
	if err := e.writeInteger(buf, length, 7, 0x00); err != nil {
		return err
	}

	// Write string bytes
	buf.WriteString(str)
	return nil
}

// SetMaxTableSize updates the dynamic table size
func (e *Encoder) SetMaxTableSize(size uint32) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.maxTableSize = size
	e.dynamicTable.SetMaxSize(size)
}

// Reset clears the dynamic table
func (e *Encoder) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dynamicTable = NewDynamicTable(e.maxTableSize)
}
