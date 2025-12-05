package hpack

import (
	"fmt"
	"sync"
)

// StaticTable implements the HPACK static table (RFC 7541 Appendix A)
// The static table is predefined and never changes
type StaticTable struct {
	entries []HeaderField
	nameMap map[string][]uint32 // Maps name to list of indices
}

var (
	staticTableInstance *StaticTable
	staticTableOnce     sync.Once
)

// GetStaticTable returns the singleton static table instance
func GetStaticTable() *StaticTable {
	staticTableOnce.Do(func() {
		staticTableInstance = newStaticTable()
	})
	return staticTableInstance
}

// newStaticTable creates and initializes the static table
func newStaticTable() *StaticTable {
	// RFC 7541 Appendix A - Static Table Definition
	// We include the most common headers for HTTP
	entries := []HeaderField{
		{Name: ":authority", Value: ""},
		{Name: ":method", Value: "GET"},
		{Name: ":method", Value: "POST"},
		{Name: ":path", Value: "/"},
		{Name: ":path", Value: "/index.html"},
		{Name: ":scheme", Value: "http"},
		{Name: ":scheme", Value: "https"},
		{Name: ":status", Value: "200"},
		{Name: ":status", Value: "204"},
		{Name: ":status", Value: "206"},
		{Name: ":status", Value: "304"},
		{Name: ":status", Value: "400"},
		{Name: ":status", Value: "404"},
		{Name: ":status", Value: "500"},
		{Name: "accept-charset", Value: ""},
		{Name: "accept-encoding", Value: "gzip, deflate"},
		{Name: "accept-language", Value: ""},
		{Name: "accept-ranges", Value: ""},
		{Name: "accept", Value: ""},
		{Name: "access-control-allow-origin", Value: ""},
		{Name: "age", Value: ""},
		{Name: "allow", Value: ""},
		{Name: "authorization", Value: ""},
		{Name: "cache-control", Value: ""},
		{Name: "content-disposition", Value: ""},
		{Name: "content-encoding", Value: ""},
		{Name: "content-language", Value: ""},
		{Name: "content-length", Value: ""},
		{Name: "content-location", Value: ""},
		{Name: "content-range", Value: ""},
		{Name: "content-type", Value: ""},
		{Name: "cookie", Value: ""},
		{Name: "date", Value: ""},
		{Name: "etag", Value: ""},
		{Name: "expect", Value: ""},
		{Name: "expires", Value: ""},
		{Name: "from", Value: ""},
		{Name: "host", Value: ""},
		{Name: "if-match", Value: ""},
		{Name: "if-modified-since", Value: ""},
		{Name: "if-none-match", Value: ""},
		{Name: "if-range", Value: ""},
		{Name: "if-unmodified-since", Value: ""},
		{Name: "last-modified", Value: ""},
		{Name: "link", Value: ""},
		{Name: "location", Value: ""},
		{Name: "max-forwards", Value: ""},
		{Name: "proxy-authenticate", Value: ""},
		{Name: "proxy-authorization", Value: ""},
		{Name: "range", Value: ""},
		{Name: "referer", Value: ""},
		{Name: "refresh", Value: ""},
		{Name: "retry-after", Value: ""},
		{Name: "server", Value: ""},
		{Name: "set-cookie", Value: ""},
		{Name: "strict-transport-security", Value: ""},
		{Name: "transfer-encoding", Value: ""},
		{Name: "user-agent", Value: ""},
		{Name: "vary", Value: ""},
		{Name: "via", Value: ""},
		{Name: "www-authenticate", Value: ""},
	}

	// Build name index map
	nameMap := make(map[string][]uint32)
	for i, entry := range entries {
		nameMap[entry.Name] = append(nameMap[entry.Name], uint32(i))
	}

	return &StaticTable{
		entries: entries,
		nameMap: nameMap,
	}
}

// Get retrieves a header field by index (0-based)
func (st *StaticTable) Get(index uint32) (string, string, error) {
	if index >= uint32(len(st.entries)) {
		return "", "", fmt.Errorf("index %d out of range (static table size: %d)", index, len(st.entries))
	}

	field := st.entries[index]
	return field.Name, field.Value, nil
}

// FindExact searches for an exact match (name and value)
// Returns the index (0-based) and true if found
func (st *StaticTable) FindExact(name, value string) (uint32, bool) {
	indices, exists := st.nameMap[name]
	if !exists {
		return 0, false
	}

	for _, index := range indices {
		field := st.entries[index]
		if field.Value == value {
			return index, true
		}
	}

	return 0, false
}

// FindName searches for a name match
// Returns the first matching index (0-based) and true if found
func (st *StaticTable) FindName(name string) (uint32, bool) {
	indices, exists := st.nameMap[name]
	if !exists || len(indices) == 0 {
		return 0, false
	}

	return indices[0], true
}

// Size returns the number of entries in the static table
func (st *StaticTable) Size() int {
	return len(st.entries)
}
