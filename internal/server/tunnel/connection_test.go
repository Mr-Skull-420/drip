package tunnel

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewConnection(t *testing.T) {
	subdomain := "test123"
	logger := zap.NewNop()

	// We can't create a real WebSocket connection in unit tests,
	// so we'll just test with nil
	conn := NewConnection(subdomain, nil, logger)

	if conn == nil {
		t.Fatal("NewConnection() returned nil")
	}

	if conn.Subdomain != subdomain {
		t.Errorf("Subdomain = %v, want %v", conn.Subdomain, subdomain)
	}

	if conn.SendCh == nil {
		t.Error("SendCh is nil")
	}

	if conn.CloseCh == nil {
		t.Error("CloseCh is nil")
	}

	// Check that LastActive is recent (within last second)
	now := time.Now()
	if now.Sub(conn.LastActive) > time.Second {
		t.Error("LastActive is not recent")
	}
}

func TestConnectionUpdateActivity(t *testing.T) {
	logger := zap.NewNop()
	conn := NewConnection("test", nil, logger)

	// Get initial LastActive
	initial := conn.LastActive

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Update activity
	conn.UpdateActivity()

	// Check that LastActive was updated
	if !conn.LastActive.After(initial) {
		t.Error("UpdateActivity() did not update LastActive")
	}

	// Check that it's recent
	now := time.Now()
	if now.Sub(conn.LastActive) > time.Second {
		t.Error("UpdateActivity() did not set recent timestamp")
	}
}

func TestConnectionIsAlive(t *testing.T) {
	tests := []struct {
		name       string
		lastActive time.Time
		timeout    time.Duration
		want       bool
	}{
		{
			name:       "fresh connection is alive",
			lastActive: time.Now(),
			timeout:    90 * time.Second,
			want:       true,
		},
		{
			name:       "stale connection is not alive",
			lastActive: time.Now().Add(-2 * time.Minute),
			timeout:    90 * time.Second,
			want:       false,
		},
		{
			name:       "exactly at timeout is not alive",
			lastActive: time.Now().Add(-90 * time.Second),
			timeout:    90 * time.Second,
			want:       false,
		},
		{
			name:       "just before timeout is alive",
			lastActive: time.Now().Add(-89 * time.Second),
			timeout:    90 * time.Second,
			want:       true,
		},
	}
	logger := zap.NewNop()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := NewConnection("test", nil, logger)
			conn.mu.Lock()
			conn.LastActive = tt.lastActive
			conn.mu.Unlock()

			got := conn.IsAlive(tt.timeout)
			if got != tt.want {
				t.Errorf("IsAlive() = %v, want %v (age: %v, timeout: %v)",
					got, tt.want, time.Since(tt.lastActive), tt.timeout)
			}
		})
	}
}

func TestConnectionSend(t *testing.T) {
	logger := zap.NewNop()
	conn := NewConnection("test", nil, logger)

	data := []byte("test message")

	// Test successful send
	err := conn.Send(data)
	if err != nil {
		t.Errorf("Send() error = %v, want nil", err)
	}

	// Verify data was sent to channel
	select {
	case received := <-conn.SendCh:
		if string(received) != string(data) {
			t.Errorf("Received data = %v, want %v", string(received), string(data))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Send() did not send data to channel")
	}
}

func TestConnectionSendTimeout(t *testing.T) {
	logger := zap.NewNop()
	conn := NewConnection("test", nil, logger)

	// Fill the channel
	for i := 0; i < 256; i++ {
		conn.SendCh <- []byte("fill")
	}

	// Try to send when channel is full
	data := []byte("test message")
	err := conn.Send(data)

	if err != ErrSendTimeout {
		t.Errorf("Send() on full channel error = %v, want %v", err, ErrSendTimeout)
	}
}

func TestConnectionClose(t *testing.T) {
	logger := zap.NewNop()
	conn := NewConnection("test", nil, logger)

	// Close the connection
	conn.Close()

	// Verify CloseCh is closed
	select {
	case <-conn.CloseCh:
		// Successfully received from closed channel
	case <-time.After(100 * time.Millisecond):
		t.Error("Close() did not close CloseCh")
	}

	// Try to close again (should not panic)
	defer func() {
		if r := recover(); r != nil {
			t.Error("Close() panicked on second call")
		}
	}()
	conn.Close()
}

func TestConnectionConcurrentUpdateActivity(t *testing.T) {
	logger := zap.NewNop()
	conn := NewConnection("test", nil, logger)

	// Update activity concurrently
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			conn.UpdateActivity()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Verify LastActive is recent
	now := time.Now()
	if now.Sub(conn.LastActive) > time.Second {
		t.Error("Concurrent UpdateActivity() failed")
	}
}

func TestConnectionConcurrentIsAlive(t *testing.T) {
	logger := zap.NewNop()
	conn := NewConnection("test", nil, logger)

	// Check IsAlive concurrently
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			conn.IsAlive(90 * time.Second)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

// Benchmark tests
func BenchmarkConnectionSend(b *testing.B) {
	logger := zap.NewNop()
	conn := NewConnection("test", nil, logger)

	// Drain channel in background
	go func() {
		for range conn.SendCh {
		}
	}()

	data := []byte("test message")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		conn.Send(data)
	}
}

func BenchmarkConnectionUpdateActivity(b *testing.B) {
	logger := zap.NewNop()
	conn := NewConnection("test", nil, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.UpdateActivity()
	}
}

func BenchmarkConnectionIsAlive(b *testing.B) {
	logger := zap.NewNop()
	conn := NewConnection("test", nil, logger)
	timeout := 90 * time.Second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.IsAlive(timeout)
	}
}
