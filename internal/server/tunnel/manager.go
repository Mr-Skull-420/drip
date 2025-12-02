package tunnel

import (
	"sync"
	"time"

	"drip/internal/shared/utils"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Manager manages all active tunnel connections
type Manager struct {
	tunnels map[string]*Connection // subdomain -> connection
	mu      sync.RWMutex
	used    map[string]bool // track used subdomains
	logger  *zap.Logger
}

// NewManager creates a new tunnel manager
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		tunnels: make(map[string]*Connection),
		used:    make(map[string]bool),
		logger:  logger,
	}
}

// Register registers a new tunnel connection
// Returns the assigned subdomain and any error
func (m *Manager) Register(conn *websocket.Conn, customSubdomain string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var subdomain string

	if customSubdomain != "" {
		// Validate custom subdomain
		if !utils.ValidateSubdomain(customSubdomain) {
			return "", ErrInvalidSubdomain
		}
		if utils.IsReserved(customSubdomain) {
			return "", ErrReservedSubdomain
		}
		if m.used[customSubdomain] {
			return "", ErrSubdomainTaken
		}
		subdomain = customSubdomain
	} else {
		// Generate unique random subdomain
		subdomain = m.generateUniqueSubdomain()
	}

	// Create connection
	tc := NewConnection(subdomain, conn, m.logger)
	m.tunnels[subdomain] = tc
	m.used[subdomain] = true

	// Start write pump in background
	go tc.StartWritePump()

	m.logger.Info("Tunnel registered",
		zap.String("subdomain", subdomain),
		zap.Int("total_tunnels", len(m.tunnels)),
	)

	return subdomain, nil
}

// Unregister removes a tunnel connection
func (m *Manager) Unregister(subdomain string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tc, ok := m.tunnels[subdomain]; ok {
		tc.Close()
		delete(m.tunnels, subdomain)
		delete(m.used, subdomain)

		m.logger.Info("Tunnel unregistered",
			zap.String("subdomain", subdomain),
			zap.Int("total_tunnels", len(m.tunnels)),
		)
	}
}

// Get retrieves a tunnel connection by subdomain
func (m *Manager) Get(subdomain string) (*Connection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tc, ok := m.tunnels[subdomain]
	return tc, ok
}

// List returns all active tunnel connections
func (m *Manager) List() []*Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	connections := make([]*Connection, 0, len(m.tunnels))
	for _, tc := range m.tunnels {
		connections = append(connections, tc)
	}
	return connections
}

// Count returns the number of active tunnels
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tunnels)
}

// CleanupStale removes stale connections that haven't been active
func (m *Manager) CleanupStale(timeout time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	staleSubdomains := []string{}

	for subdomain, tc := range m.tunnels {
		if !tc.IsAlive(timeout) {
			staleSubdomains = append(staleSubdomains, subdomain)
		}
	}

	for _, subdomain := range staleSubdomains {
		if tc, ok := m.tunnels[subdomain]; ok {
			tc.Close()
			delete(m.tunnels, subdomain)
			delete(m.used, subdomain)
		}
	}

	if len(staleSubdomains) > 0 {
		m.logger.Info("Cleaned up stale tunnels",
			zap.Int("count", len(staleSubdomains)),
		)
	}

	return len(staleSubdomains)
}

// StartCleanupTask starts a background task to clean up stale connections
func (m *Manager) StartCleanupTask(interval, timeout time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			m.CleanupStale(timeout)
		}
	}()
}

// generateUniqueSubdomain generates a unique random subdomain
func (m *Manager) generateUniqueSubdomain() string {
	const maxAttempts = 10

	for i := 0; i < maxAttempts; i++ {
		subdomain := utils.GenerateSubdomain(6)
		if !m.used[subdomain] && !utils.IsReserved(subdomain) {
			return subdomain
		}
	}

	// Fallback: use longer subdomain if collision persists
	return utils.GenerateSubdomain(8)
}

// Shutdown gracefully shuts down all tunnels
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("Shutting down tunnel manager",
		zap.Int("active_tunnels", len(m.tunnels)),
	)

	for _, tc := range m.tunnels {
		tc.Close()
	}

	m.tunnels = make(map[string]*Connection)
	m.used = make(map[string]bool)
}
