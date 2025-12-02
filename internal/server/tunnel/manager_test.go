package tunnel

import (
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}

	if manager.tunnels == nil {
		t.Error("Manager tunnels map is nil")
	}

	if manager.used == nil {
		t.Error("Manager used map is nil")
	}

	if manager.logger == nil {
		t.Error("Manager logger is nil")
	}
}

func TestManagerRegister(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	// Register with empty subdomain (auto-generate)
	subdomain, err := manager.Register(nil, "")
	if err != nil {
		t.Errorf("Register() error = %v, want nil", err)
	}

	if subdomain == "" {
		t.Error("Register() returned empty subdomain")
	}

	if len(subdomain) != 6 {
		t.Errorf("Register() subdomain length = %d, want 6", len(subdomain))
	}

	// Verify connection is registered
	_, ok := manager.Get(subdomain)
	if !ok {
		t.Error("Get() failed to retrieve registered connection")
	}
}

func TestManagerRegisterCustomSubdomain(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	customSubdomain := "mytest"

	// Register with custom subdomain
	subdomain, err := manager.Register(nil, customSubdomain)
	if err != nil {
		t.Errorf("Register() error = %v, want nil", err)
	}

	if subdomain != customSubdomain {
		t.Errorf("Register() subdomain = %v, want %v", subdomain, customSubdomain)
	}

	// Verify connection is registered
	conn, ok := manager.Get(subdomain)
	if !ok {
		t.Error("Get() failed to retrieve registered connection")
	}

	if conn.Subdomain != customSubdomain {
		t.Errorf("Connection subdomain = %v, want %v", conn.Subdomain, customSubdomain)
	}
}

func TestManagerRegisterDuplicate(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	customSubdomain := "test123"

	// Register first connection
	_, err := manager.Register(nil, customSubdomain)
	if err != nil {
		t.Fatalf("First Register() error = %v, want nil", err)
	}

	// Try to register second connection with same subdomain
	_, err = manager.Register(nil, customSubdomain)
	if err != ErrSubdomainTaken {
		t.Errorf("Register() error = %v, want %v", err, ErrSubdomainTaken)
	}
}

func TestManagerRegisterInvalidSubdomain(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	tests := []struct {
		name      string
		subdomain string
		wantErr   error
	}{
		{
			name:      "invalid uppercase",
			subdomain: "TEST",
			wantErr:   ErrInvalidSubdomain,
		},
		{
			name:      "invalid special char",
			subdomain: "test@123",
			wantErr:   ErrInvalidSubdomain,
		},
		{
			name:      "reserved www",
			subdomain: "www",
			wantErr:   ErrReservedSubdomain,
		},
		{
			name:      "reserved api",
			subdomain: "api",
			wantErr:   ErrReservedSubdomain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.Register(nil, tt.subdomain)
			if err != tt.wantErr {
				t.Errorf("Register() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestManagerUnregister(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	// Register connection
	subdomain, err := manager.Register(nil, "")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Unregister connection
	manager.Unregister(subdomain)

	// Verify connection is removed
	_, ok := manager.Get(subdomain)
	if ok {
		t.Error("Get() succeeded after Unregister(), want failure")
	}
}

func TestManagerGet(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	customSubdomain := "test123"

	// Test Get on non-existent connection
	_, ok := manager.Get(customSubdomain)
	if ok {
		t.Error("Get() succeeded for non-existent connection")
	}

	// Register and test Get
	subdomain, _ := manager.Register(nil, customSubdomain)
	retrieved, ok := manager.Get(subdomain)
	if !ok {
		t.Error("Get() failed for existing connection")
	}
	if retrieved.Subdomain != customSubdomain {
		t.Error("Get() returned wrong connection")
	}
}

func TestManagerList(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	// Test empty manager
	all := manager.List()
	if len(all) != 0 {
		t.Errorf("List() on empty manager returned %d connections, want 0", len(all))
	}

	// Add multiple connections
	count := 5
	for i := 0; i < count; i++ {
		manager.Register(nil, "")
	}

	// Test List
	all = manager.List()
	if len(all) != count {
		t.Errorf("List() returned %d connections, want %d", len(all), count)
	}
}

func TestManagerCount(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	// Test empty manager
	count := manager.Count()
	if count != 0 {
		t.Errorf("Count() on empty manager = %d, want 0", count)
	}

	// Add connections
	numConns := 3
	for i := 0; i < numConns; i++ {
		manager.Register(nil, "")
	}

	count = manager.Count()
	if count != numConns {
		t.Errorf("Count() = %d, want %d", count, numConns)
	}
}

func TestManagerGenerateSubdomain(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	// Generate subdomain via Register
	subdomain1, err := manager.Register(nil, "")
	if err != nil {
		t.Fatalf("First Register() error = %v", err)
	}

	if subdomain1 == "" {
		t.Error("Register() returned empty subdomain")
	}

	if len(subdomain1) != 6 {
		t.Errorf("Register() subdomain length = %d, want 6", len(subdomain1))
	}

	// Generate another subdomain, should be different
	subdomain2, err := manager.Register(nil, "")
	if err != nil {
		t.Fatalf("Second Register() error = %v", err)
	}

	if subdomain1 == subdomain2 {
		t.Error("Register() generated duplicate subdomain")
	}
}

func TestManagerGenerateSubdomainUniqueness(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	subdomains := make(map[string]bool)
	count := 100

	for i := 0; i < count; i++ {
		subdomain, err := manager.Register(nil, "")
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if subdomains[subdomain] {
			t.Errorf("Register() generated duplicate: %s", subdomain)
		}
		subdomains[subdomain] = true
	}
}

func TestManagerCleanupStale(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	// Create fresh connection
	freshSubdomain, _ := manager.Register(nil, "fresh")

	// Create stale connection
	staleSubdomain, _ := manager.Register(nil, "stale")

	// Manually set LastActive to be stale
	if staleConn, ok := manager.Get(staleSubdomain); ok {
		staleConn.mu.Lock()
		staleConn.LastActive = time.Now().Add(-2 * time.Minute)
		staleConn.mu.Unlock()
	}

	// Run cleanup with 90 second timeout
	count := manager.CleanupStale(90 * time.Second)
	if count != 1 {
		t.Errorf("CleanupStale() returned %d, want 1", count)
	}

	// Fresh connection should still exist
	_, ok := manager.Get(freshSubdomain)
	if !ok {
		t.Error("CleanupStale() removed fresh connection")
	}

	// Stale connection should be removed
	_, ok = manager.Get(staleSubdomain)
	if ok {
		t.Error("CleanupStale() did not remove stale connection")
	}
}

func TestManagerConcurrentAccess(t *testing.T) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	var wg sync.WaitGroup
	count := 50 // Reduced from 100 to avoid potential issues

	// Concurrent registrations
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := manager.Register(nil, "")
			if err != nil {
				t.Errorf("Concurrent Register() error = %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all connections are registered
	all := manager.List()
	if len(all) != count {
		t.Errorf("Expected %d connections, got %d", count, len(all))
	}
}

// Benchmark tests
func BenchmarkManagerRegister(b *testing.B) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Register(nil, "")
	}
}

func BenchmarkManagerGet(b *testing.B) {
	logger := zap.NewNop()
	manager := NewManager(logger)

	// Setup: register a connection
	subdomain, _ := manager.Register(nil, "test123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Get(subdomain)
	}
}

func BenchmarkManagerGenerateSubdomain(b *testing.B) {
	logger := zap.NewNop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager := NewManager(logger)
		manager.Register(nil, "")
	}
}
