package hpack

import (
	"fmt"
)

// DynamicTable implements the HPACK dynamic table (RFC 7541 Section 2.3.2)
// The dynamic table is a FIFO queue where new entries are added at the beginning
// and old entries are evicted when the table size exceeds the maximum
type DynamicTable struct {
	entries  []HeaderField
	size     uint32 // Current size in bytes
	maxSize  uint32 // Maximum size in bytes
}

// HeaderField represents a header name-value pair
type HeaderField struct {
	Name  string
	Value string
}

// Size returns the size of this header field in bytes
// RFC 7541: size = len(name) + len(value) + 32
func (h *HeaderField) Size() uint32 {
	return uint32(len(h.Name) + len(h.Value) + 32)
}

// NewDynamicTable creates a new dynamic table with the specified maximum size
func NewDynamicTable(maxSize uint32) *DynamicTable {
	return &DynamicTable{
		entries: make([]HeaderField, 0, 32),
		size:    0,
		maxSize: maxSize,
	}
}

// Add adds a header field to the dynamic table
// New entries are added at the beginning (index 0)
func (dt *DynamicTable) Add(name, value string) {
	field := HeaderField{Name: name, Value: value}
	fieldSize := field.Size()

	// If the field is larger than maxSize, don't add it
	if fieldSize > dt.maxSize {
		dt.evictAll()
		return
	}

	// Evict entries if necessary to make room
	for dt.size+fieldSize > dt.maxSize && len(dt.entries) > 0 {
		dt.evictOldest()
	}

	// Add new entry at the beginning
	dt.entries = append([]HeaderField{field}, dt.entries...)
	dt.size += fieldSize
}

// Get retrieves a header field by index (0-based)
// Index 0 is the most recently added entry
func (dt *DynamicTable) Get(index uint32) (string, string, error) {
	if index >= uint32(len(dt.entries)) {
		return "", "", fmt.Errorf("index %d out of range (table size: %d)", index, len(dt.entries))
	}

	field := dt.entries[index]
	return field.Name, field.Value, nil
}

// FindExact searches for an exact match (name and value)
// Returns the index (0-based) and true if found
func (dt *DynamicTable) FindExact(name, value string) (uint32, bool) {
	for i, field := range dt.entries {
		if field.Name == name && field.Value == value {
			return uint32(i), true
		}
	}
	return 0, false
}

// FindName searches for a name match
// Returns the index (0-based) and true if found
func (dt *DynamicTable) FindName(name string) (uint32, bool) {
	for i, field := range dt.entries {
		if field.Name == name {
			return uint32(i), true
		}
	}
	return 0, false
}

// SetMaxSize updates the maximum table size
// If the new size is smaller, entries are evicted
func (dt *DynamicTable) SetMaxSize(maxSize uint32) {
	dt.maxSize = maxSize

	// Evict entries if current size exceeds new max
	for dt.size > dt.maxSize && len(dt.entries) > 0 {
		dt.evictOldest()
	}
}

// CurrentSize returns the current size of the table in bytes
func (dt *DynamicTable) CurrentSize() uint32 {
	return dt.size
}

// evictOldest removes the oldest entry (last in the slice)
func (dt *DynamicTable) evictOldest() {
	if len(dt.entries) == 0 {
		return
	}

	lastIndex := len(dt.entries) - 1
	evicted := dt.entries[lastIndex]
	dt.entries = dt.entries[:lastIndex]
	dt.size -= evicted.Size()
}

// evictAll removes all entries
func (dt *DynamicTable) evictAll() {
	dt.entries = dt.entries[:0]
	dt.size = 0
}
