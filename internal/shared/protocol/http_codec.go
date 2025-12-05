package protocol

import (
	"errors"

	json "github.com/goccy/go-json"

	"github.com/vmihailenco/msgpack/v5"
)

// EncodeHTTPRequest encodes HTTPRequest using msgpack encoding (optimized)
func EncodeHTTPRequest(req *HTTPRequest) ([]byte, error) {
	return msgpack.Marshal(req)
}

// DecodeHTTPRequest decodes HTTPRequest with automatic version detection
// Detects based on first byte: '{' = JSON, else = msgpack
func DecodeHTTPRequest(data []byte) (*HTTPRequest, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	var req HTTPRequest

	// Auto-detect: JSON starts with '{', msgpack starts with 0x80-0x8f (fixmap)
	if data[0] == '{' {
		// v1: JSON
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, err
		}
	} else {
		// v2: msgpack
		if err := msgpack.Unmarshal(data, &req); err != nil {
			return nil, err
		}
	}

	return &req, nil
}

// EncodeHTTPRequestHead encodes HTTP request headers for streaming
func EncodeHTTPRequestHead(head *HTTPRequestHead) ([]byte, error) {
	return msgpack.Marshal(head)
}

// DecodeHTTPRequestHead decodes HTTP request headers for streaming
func DecodeHTTPRequestHead(data []byte) (*HTTPRequestHead, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	var head HTTPRequestHead
	if data[0] == '{' {
		if err := json.Unmarshal(data, &head); err != nil {
			return nil, err
		}
	} else {
		if err := msgpack.Unmarshal(data, &head); err != nil {
			return nil, err
		}
	}

	return &head, nil
}

// EncodeHTTPResponse encodes HTTPResponse using msgpack encoding (optimized)
func EncodeHTTPResponse(resp *HTTPResponse) ([]byte, error) {
	return msgpack.Marshal(resp)
}

// DecodeHTTPResponse decodes HTTPResponse with automatic version detection
// Detects based on first byte: '{' = JSON, else = msgpack
func DecodeHTTPResponse(data []byte) (*HTTPResponse, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	var resp HTTPResponse

	// Auto-detect: JSON starts with '{', msgpack starts with 0x80-0x8f (fixmap)
	if data[0] == '{' {
		// v1: JSON
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
	} else {
		// v2: msgpack
		if err := msgpack.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
	}

	return &resp, nil
}

// EncodeHTTPResponseHead encodes HTTP response headers for streaming
func EncodeHTTPResponseHead(head *HTTPResponseHead) ([]byte, error) {
	return msgpack.Marshal(head)
}

// DecodeHTTPResponseHead decodes HTTP response headers for streaming
func DecodeHTTPResponseHead(data []byte) (*HTTPResponseHead, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	var head HTTPResponseHead
	if data[0] == '{' {
		if err := json.Unmarshal(data, &head); err != nil {
			return nil, err
		}
	} else {
		if err := msgpack.Unmarshal(data, &head); err != nil {
			return nil, err
		}
	}

	return &head, nil
}
