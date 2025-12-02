package utils

import (
	"strings"
	"testing"
)

func TestGenerateID(t *testing.T) {
	tests := []struct {
		name       string
		wantLength int // expected minimum length
	}{
		{
			name:       "generate valid ID",
			wantLength: 16, // At least 16 characters for hex-encoded random bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateID()

			// Check that ID is not empty
			if got == "" {
				t.Error("GenerateID() returned empty string")
			}

			// Check minimum length
			if len(got) < tt.wantLength {
				t.Errorf("GenerateID() length = %v, want at least %v", len(got), tt.wantLength)
			}

			// Check that it's a valid hex string
			for _, char := range got {
				if !isHexChar(char) {
					t.Errorf("GenerateID() contains non-hex character: %c", char)
				}
			}
		})
	}
}

func TestGenerateIDUniqueness(t *testing.T) {
	// Generate 10000 IDs and check for uniqueness
	ids := make(map[string]bool)
	count := 10000

	for i := 0; i < count; i++ {
		id := GenerateID()
		if ids[id] {
			t.Errorf("GenerateID() generated duplicate: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != count {
		t.Errorf("Expected %d unique IDs, got %d", count, len(ids))
	}
}

func TestGenerateIDFormat(t *testing.T) {
	id := GenerateID()

	// Check that it's lowercase
	if id != strings.ToLower(id) {
		t.Errorf("GenerateID() is not lowercase: %s", id)
	}

	// Check that it doesn't contain special characters
	for _, char := range id {
		if !isHexChar(char) {
			t.Errorf("GenerateID() contains invalid character: %c in %s", char, id)
		}
	}
}

func TestGenerateIDConsistency(t *testing.T) {
	// Generate multiple IDs and ensure they all follow the same format
	count := 100
	firstID := GenerateID()
	firstLen := len(firstID)

	for i := 0; i < count; i++ {
		id := GenerateID()

		// All IDs should have the same length
		if len(id) != firstLen {
			t.Errorf("ID length inconsistency: first=%d, current=%d", firstLen, len(id))
		}

		// All IDs should be hex strings
		for _, char := range id {
			if !isHexChar(char) {
				t.Errorf("Invalid hex character %c in ID: %s", char, id)
			}
		}
	}
}

func TestGenerateIDNotEmpty(t *testing.T) {
	// Generate 1000 IDs and ensure none are empty
	for i := 0; i < 1000; i++ {
		id := GenerateID()
		if id == "" {
			t.Error("GenerateID() returned empty string")
		}
		if len(id) == 0 {
			t.Error("GenerateID() returned zero-length string")
		}
	}
}

// Helper function to check if a character is a valid hex character
func isHexChar(char rune) bool {
	return (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')
}

// Benchmark tests
func BenchmarkGenerateID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateID()
	}
}

func BenchmarkGenerateIDParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			GenerateID()
		}
	})
}

// Test for concurrent ID generation
func TestGenerateIDConcurrent(t *testing.T) {
	count := 1000
	ch := make(chan string, count)

	// Generate IDs concurrently
	for i := 0; i < count; i++ {
		go func() {
			ch <- GenerateID()
		}()
	}

	// Collect all IDs
	ids := make(map[string]bool)
	for i := 0; i < count; i++ {
		id := <-ch
		if ids[id] {
			t.Errorf("Concurrent GenerateID() generated duplicate: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != count {
		t.Errorf("Expected %d unique IDs, got %d", count, len(ids))
	}
}
