package protocol

// TunnelType defines the type of tunnel
type TunnelType string

const (
	// TunnelTypeHTTP is for HTTP traffic
	TunnelTypeHTTP TunnelType = "http"
	// TunnelTypeHTTPS is for HTTPS traffic
	TunnelTypeHTTPS TunnelType = "https"
	// TunnelTypeTCP is for generic TCP traffic
	TunnelTypeTCP TunnelType = "tcp"
	// TunnelTypeUDP is for UDP traffic (future support)
	TunnelTypeUDP TunnelType = "udp"
)

// String returns the string representation
func (t TunnelType) String() string {
	return string(t)
}

// IsValid checks if tunnel type is valid
func (t TunnelType) IsValid() bool {
	switch t {
	case TunnelTypeHTTP, TunnelTypeHTTPS, TunnelTypeTCP, TunnelTypeUDP:
		return true
	default:
		return false
	}
}
