package protocol

// MessageType defines the type of tunnel message
type MessageType string

const (
	// TypeRegister is sent when a client connects and gets a subdomain assigned
	TypeRegister MessageType = "register"
	// TypeRequest is sent from server to client when an HTTP request arrives
	TypeRequest MessageType = "request"
	// TypeResponse is sent from client to server with the HTTP response
	TypeResponse MessageType = "response"
	// TypeHeartbeat is sent periodically to keep the connection alive
	TypeHeartbeat MessageType = "heartbeat"
	// TypeError is sent when an error occurs
	TypeError MessageType = "error"
)

// Message represents a tunnel protocol message
type Message struct {
	Type      MessageType            `json:"type"`
	ID        string                 `json:"id,omitempty"`
	Subdomain string                 `json:"subdomain,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// HTTPRequest represents an HTTP request to be forwarded
type HTTPRequest struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body,omitempty"`
}

// HTTPResponse represents an HTTP response from the local service
type HTTPResponse struct {
	StatusCode int                 `json:"status_code"`
	Status     string              `json:"status"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body,omitempty"`
}

// RegisterData contains information sent when a tunnel is registered
type RegisterData struct {
	Subdomain string `json:"subdomain"`
	URL       string `json:"url"`
	Message   string `json:"message"`
}

// ErrorData contains error information
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
