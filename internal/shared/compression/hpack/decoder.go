package hpack

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"sync"
)

// Decoder decompresses HPACK-encoded headers
// Each connection MUST have its own decoder instance to maintain correct state
type Decoder struct {
	mu           sync.Mutex
	dynamicTable *DynamicTable
	staticTable  *StaticTable
	maxTableSize uint32
}

// NewDecoder creates a new HPACK decoder
func NewDecoder(maxTableSize uint32) *Decoder {
	if maxTableSize == 0 {
		maxTableSize = DefaultDynamicTableSize
	}

	return &Decoder{
		dynamicTable: NewDynamicTable(maxTableSize),
		staticTable:  GetStaticTable(),
		maxTableSize: maxTableSize,
	}
}

// Decode decodes HPACK-encoded headers
func (d *Decoder) Decode(data []byte) (http.Header, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(data) == 0 {
		return http.Header{}, nil
	}

	headers := make(http.Header)
	buf := bytes.NewReader(data)

	for buf.Len() > 0 {
		b, err := buf.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read header byte: %w", err)
		}

		// Unread the byte so we can process it properly
		if err := buf.UnreadByte(); err != nil {
			return nil, err
		}

		var name, value string

		if b&0x80 != 0 {
			// Indexed header field (10xxxxxx)
			name, value, err = d.decodeIndexedHeader(buf)
		} else if b&0x40 != 0 {
			// Literal with incremental indexing (01xxxxxx)
			name, value, err = d.decodeLiteralWithIndexing(buf)
		} else {
			// Literal without indexing (0000xxxx)
			name, value, err = d.decodeLiteralWithoutIndexing(buf)
		}

		if err != nil {
			return nil, err
		}

		headers.Add(name, value)
	}

	return headers, nil
}

// decodeIndexedHeader decodes an indexed header field
func (d *Decoder) decodeIndexedHeader(buf *bytes.Reader) (string, string, error) {
	index, err := d.readInteger(buf, 7)
	if err != nil {
		return "", "", fmt.Errorf("read index: %w", err)
	}

	if index == 0 {
		return "", "", errors.New("invalid index: 0")
	}

	staticSize := uint32(d.staticTable.Size())

	if index <= staticSize {
		// Static table
		return d.staticTable.Get(index - 1)
	}

	// Dynamic table (indices start after static table)
	dynamicIndex := index - staticSize - 1
	return d.dynamicTable.Get(dynamicIndex)
}

// decodeLiteralWithIndexing decodes a literal header with incremental indexing
func (d *Decoder) decodeLiteralWithIndexing(buf *bytes.Reader) (string, string, error) {
	nameIndex, err := d.readInteger(buf, 6)
	if err != nil {
		return "", "", err
	}

	var name string
	if nameIndex == 0 {
		// Name is literal
		name, err = d.readString(buf)
		if err != nil {
			return "", "", fmt.Errorf("read name: %w", err)
		}
	} else {
		// Name is indexed
		staticSize := uint32(d.staticTable.Size())
		if nameIndex <= staticSize {
			name, _, err = d.staticTable.Get(nameIndex - 1)
		} else {
			dynamicIndex := nameIndex - staticSize - 1
			name, _, err = d.dynamicTable.Get(dynamicIndex)
		}
		if err != nil {
			return "", "", fmt.Errorf("get indexed name: %w", err)
		}
	}

	// Value is always literal
	value, err := d.readString(buf)
	if err != nil {
		return "", "", fmt.Errorf("read value: %w", err)
	}

	// Add to dynamic table
	d.dynamicTable.Add(name, value)

	return name, value, nil
}

// decodeLiteralWithoutIndexing decodes a literal header without indexing
func (d *Decoder) decodeLiteralWithoutIndexing(buf *bytes.Reader) (string, string, error) {
	nameIndex, err := d.readInteger(buf, 4)
	if err != nil {
		return "", "", err
	}

	var name string
	if nameIndex == 0 {
		// Name is literal
		name, err = d.readString(buf)
		if err != nil {
			return "", "", fmt.Errorf("read name: %w", err)
		}
	} else {
		// Name is indexed
		staticSize := uint32(d.staticTable.Size())
		if nameIndex <= staticSize {
			name, _, err = d.staticTable.Get(nameIndex - 1)
		} else {
			dynamicIndex := nameIndex - staticSize - 1
			name, _, err = d.dynamicTable.Get(dynamicIndex)
		}
		if err != nil {
			return "", "", fmt.Errorf("get indexed name: %w", err)
		}
	}

	// Value is always literal
	value, err := d.readString(buf)
	if err != nil {
		return "", "", fmt.Errorf("read value: %w", err)
	}

	// Do NOT add to dynamic table

	return name, value, nil
}

// readInteger reads an HPACK integer
func (d *Decoder) readInteger(buf *bytes.Reader, prefixBits int) (uint32, error) {
	if prefixBits < 1 || prefixBits > 8 {
		return 0, fmt.Errorf("invalid prefix bits: %d", prefixBits)
	}

	b, err := buf.ReadByte()
	if err != nil {
		return 0, err
	}

	maxPrefix := uint32((1 << prefixBits) - 1)
	mask := byte(maxPrefix)

	value := uint32(b & mask)
	if value < maxPrefix {
		return value, nil
	}

	// Multi-byte integer
	m := uint32(0)
	for {
		b, err := buf.ReadByte()
		if err != nil {
			return 0, err
		}

		value += uint32(b&0x7f) << m
		m += 7

		if b&0x80 == 0 {
			break
		}

		if m > 28 {
			return 0, errors.New("integer overflow")
		}
	}

	return value, nil
}

// readString reads an HPACK string
func (d *Decoder) readString(buf *bytes.Reader) (string, error) {
	b, err := buf.ReadByte()
	if err != nil {
		return "", err
	}

	if err := buf.UnreadByte(); err != nil {
		return "", err
	}

	huffmanEncoded := (b & 0x80) != 0

	length, err := d.readInteger(buf, 7)
	if err != nil {
		return "", fmt.Errorf("read string length: %w", err)
	}

	if length == 0 {
		return "", nil
	}

	if length > uint32(buf.Len()) {
		return "", fmt.Errorf("string length %d exceeds buffer size %d", length, buf.Len())
	}

	data := make([]byte, length)
	n, err := buf.Read(data)
	if err != nil {
		return "", err
	}
	if n != int(length) {
		return "", fmt.Errorf("expected %d bytes, read %d", length, n)
	}

	if huffmanEncoded {
		// TODO: Implement Huffman decoding if needed
		return "", errors.New("huffman decoding not implemented")
	}

	return string(data), nil
}

// SetMaxTableSize updates the dynamic table size
func (d *Decoder) SetMaxTableSize(size uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.maxTableSize = size
	d.dynamicTable.SetMaxSize(size)
}

// Reset clears the dynamic table
func (d *Decoder) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dynamicTable = NewDynamicTable(d.maxTableSize)
}
