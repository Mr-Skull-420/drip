package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMessageType_Values(t *testing.T) {
	tests := []struct {
		name string
		mt   MessageType
		want string
	}{
		{
			name: "register",
			mt:   TypeRegister,
			want: "register",
		},
		{
			name: "request",
			mt:   TypeRequest,
			want: "request",
		},
		{
			name: "response",
			mt:   TypeResponse,
			want: "response",
		},
		{
			name: "heartbeat",
			mt:   TypeHeartbeat,
			want: "heartbeat",
		},
		{
			name: "error",
			mt:   TypeError,
			want: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(tt.mt)
			if got != tt.want {
				t.Errorf("MessageType value = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_JSON(t *testing.T) {
	tests := []struct {
		name    string
		message *Message
		wantErr bool
	}{
		{
			name: "simple message",
			message: &Message{
				Type: TypeRegister,
				ID:   "test-id-123",
			},
			wantErr: false,
		},
		{
			name: "message with subdomain",
			message: &Message{
				Type:      TypeRegister,
				ID:        "test-id-456",
				Subdomain: "abc123",
			},
			wantErr: false,
		},
		{
			name: "message with data",
			message: &Message{
				Type: TypeRequest,
				ID:   "test-id-789",
				Data: map[string]interface{}{
					"method": "GET",
					"path":   "/test",
					"headers": map[string]interface{}{
						"User-Agent": "Test",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "error message",
			message: &Message{
				Type: TypeError,
				ID:   "test-id-error",
				Data: map[string]interface{}{
					"error": "something went wrong",
					"code":  float64(500), // JSON unmarshals numbers as float64
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(tt.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Unmarshal back
			var decoded Message
			err = json.Unmarshal(data, &decoded)
			if err != nil {
				t.Errorf("json.Unmarshal() error = %v", err)
				return
			}

			// Compare
			if decoded.Type != tt.message.Type {
				t.Errorf("Type = %v, want %v", decoded.Type, tt.message.Type)
			}
			if decoded.ID != tt.message.ID {
				t.Errorf("ID = %v, want %v", decoded.ID, tt.message.ID)
			}
			if decoded.Subdomain != tt.message.Subdomain {
				t.Errorf("Subdomain = %v, want %v", decoded.Subdomain, tt.message.Subdomain)
			}

			// Deep compare Data if present
			if tt.message.Data != nil {
				if !reflect.DeepEqual(decoded.Data, tt.message.Data) {
					t.Errorf("Data = %v, want %v", decoded.Data, tt.message.Data)
				}
			}
		})
	}
}

func TestHTTPRequest_JSON(t *testing.T) {
	req := &HTTPRequest{
		Method: "POST",
		URL:    "http://localhost:3000/api/test",
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
			"User-Agent":   {"Test Agent"},
		},
		Body: []byte(`{"key":"value"}`),
	}

	// Marshal
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal
	var decoded HTTPRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Compare
	if decoded.Method != req.Method {
		t.Errorf("Method = %v, want %v", decoded.Method, req.Method)
	}
	if decoded.URL != req.URL {
		t.Errorf("URL = %v, want %v", decoded.URL, req.URL)
	}
	if !reflect.DeepEqual(decoded.Headers, req.Headers) {
		t.Errorf("Headers = %v, want %v", decoded.Headers, req.Headers)
	}
	if string(decoded.Body) != string(req.Body) {
		t.Errorf("Body = %v, want %v", string(decoded.Body), string(req.Body))
	}
}

func TestHTTPResponse_JSON(t *testing.T) {
	resp := &HTTPResponse{
		StatusCode: 200,
		Status:     "200 OK",
		Headers: map[string][]string{
			"Content-Type": {"text/html"},
		},
		Body: []byte("<html>Test</html>"),
	}

	// Marshal
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal
	var decoded HTTPResponse
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Compare
	if decoded.StatusCode != resp.StatusCode {
		t.Errorf("StatusCode = %v, want %v", decoded.StatusCode, resp.StatusCode)
	}
	if decoded.Status != resp.Status {
		t.Errorf("Status = %v, want %v", decoded.Status, resp.Status)
	}
	if !reflect.DeepEqual(decoded.Headers, resp.Headers) {
		t.Errorf("Headers = %v, want %v", decoded.Headers, resp.Headers)
	}
	if string(decoded.Body) != string(resp.Body) {
		t.Errorf("Body = %v, want %v", string(decoded.Body), string(resp.Body))
	}
}

func TestMessage_ToMap(t *testing.T) {
	msg := &Message{
		Type:      TypeRequest,
		ID:        "test-123",
		Subdomain: "abc",
		Data: map[string]interface{}{
			"test": "value",
		},
	}

	// Convert to map (simulated by marshaling and unmarshaling)
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Verify fields exist
	if result["type"] == nil {
		t.Error("Map missing 'type' field")
	}
	if result["id"] == nil {
		t.Error("Map missing 'id' field")
	}
}

func TestNewMessage(t *testing.T) {
	msgType := TypeRegister
	id := "test-id"

	msg := &Message{
		Type: msgType,
		ID:   id,
	}

	if msg.Type != msgType {
		t.Errorf("Type = %v, want %v", msg.Type, msgType)
	}
	if msg.ID != id {
		t.Errorf("ID = %v, want %v", msg.ID, id)
	}
}

// Benchmark tests
func BenchmarkMessageMarshal(b *testing.B) {
	msg := &Message{
		Type:      TypeRequest,
		ID:        "test-id-123",
		Subdomain: "abc123",
		Data: map[string]interface{}{
			"method": "GET",
			"path":   "/test",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(msg)
	}
}

func BenchmarkMessageUnmarshal(b *testing.B) {
	msg := &Message{
		Type:      TypeRequest,
		ID:        "test-id-123",
		Subdomain: "abc123",
		Data: map[string]interface{}{
			"method": "GET",
			"path":   "/test",
		},
	}

	data, _ := json.Marshal(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decoded Message
		json.Unmarshal(data, &decoded)
	}
}
