package utils

import (
	"strings"
	"testing"
)

func TestGenerateSubdomain(t *testing.T) {
	tests := []struct {
		name   string
		length int
		want   int // expected length
	}{
		{
			name:   "default length 6",
			length: 6,
			want:   6,
		},
		{
			name:   "length 8",
			length: 8,
			want:   8,
		},
		{
			name:   "length 10",
			length: 10,
			want:   10,
		},
		{
			name:   "minimum length 4",
			length: 4,
			want:   4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateSubdomain(tt.length)

			// Check length
			if len(got) != tt.want {
				t.Errorf("GenerateSubdomain() length = %v, want %v", len(got), tt.want)
			}

			// Check that it only contains alphanumeric characters
			for _, char := range got {
				if !isAlphanumeric(char) {
					t.Errorf("GenerateSubdomain() contains non-alphanumeric character: %c", char)
				}
			}

			// Check that it's lowercase
			if got != strings.ToLower(got) {
				t.Errorf("GenerateSubdomain() is not lowercase: %s", got)
			}
		})
	}
}

func TestGenerateSubdomainUniqueness(t *testing.T) {
	// Generate 1000 subdomains and check for uniqueness
	subdomains := make(map[string]bool)
	count := 1000
	length := 6

	for i := 0; i < count; i++ {
		subdomain := GenerateSubdomain(length)
		if subdomains[subdomain] {
			t.Errorf("GenerateSubdomain() generated duplicate: %s", subdomain)
		}
		subdomains[subdomain] = true
	}

	if len(subdomains) != count {
		t.Errorf("Expected %d unique subdomains, got %d", count, len(subdomains))
	}
}

func TestValidateSubdomain(t *testing.T) {
	tests := []struct {
		name      string
		subdomain string
		want      bool
	}{
		{
			name:      "valid lowercase",
			subdomain: "abc123",
			want:      true,
		},
		{
			name:      "valid all letters",
			subdomain: "abcdef",
			want:      true,
		},
		{
			name:      "valid all numbers",
			subdomain: "123456",
			want:      true,
		},
		{
			name:      "invalid uppercase",
			subdomain: "ABC123",
			want:      false,
		},
		{
			name:      "valid with hyphen",
			subdomain: "abc-123",
			want:      true,
		},
		{
			name:      "invalid starting with hyphen",
			subdomain: "-abc123",
			want:      false,
		},
		{
			name:      "invalid ending with hyphen",
			subdomain: "abc123-",
			want:      false,
		},
		{
			name:      "invalid with underscore",
			subdomain: "abc_123",
			want:      false,
		},
		{
			name:      "invalid with dot",
			subdomain: "abc.123",
			want:      false,
		},
		{
			name:      "invalid with space",
			subdomain: "abc 123",
			want:      false,
		},
		{
			name:      "invalid empty",
			subdomain: "",
			want:      false,
		},
		{
			name:      "invalid special characters",
			subdomain: "abc@123",
			want:      false,
		},
		{
			name:      "valid minimum length",
			subdomain: "abc",
			want:      true,
		},
		{
			name:      "invalid too short",
			subdomain: "ab",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateSubdomain(tt.subdomain)
			if got != tt.want {
				t.Errorf("ValidateSubdomain(%q) = %v, want %v", tt.subdomain, got, tt.want)
			}
		})
	}
}

func TestIsReserved(t *testing.T) {
	tests := []struct {
		name      string
		subdomain string
		want      bool
	}{
		{
			name:      "reserved www",
			subdomain: "www",
			want:      true,
		},
		{
			name:      "reserved api",
			subdomain: "api",
			want:      true,
		},
		{
			name:      "reserved admin",
			subdomain: "admin",
			want:      true,
		},
		{
			name:      "reserved mail",
			subdomain: "mail",
			want:      true,
		},
		{
			name:      "reserved ftp",
			subdomain: "ftp",
			want:      true,
		},
		{
			name:      "reserved health",
			subdomain: "health",
			want:      true,
		},
		{
			name:      "reserved test",
			subdomain: "test",
			want:      true,
		},
		{
			name:      "reserved dev",
			subdomain: "dev",
			want:      true,
		},
		{
			name:      "reserved staging",
			subdomain: "staging",
			want:      true,
		},
		{
			name:      "not reserved random",
			subdomain: "abc123",
			want:      false,
		},
		{
			name:      "not reserved user",
			subdomain: "myapp",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsReserved(tt.subdomain)
			if got != tt.want {
				t.Errorf("IsReserved(%q) = %v, want %v", tt.subdomain, got, tt.want)
			}
		})
	}
}

// Helper function to check if a character is alphanumeric
func isAlphanumeric(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')
}

// Benchmark tests
func BenchmarkGenerateSubdomain(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateSubdomain(6)
	}
}

func BenchmarkValidateSubdomain(b *testing.B) {
	subdomain := "abc123"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateSubdomain(subdomain)
	}
}

func BenchmarkIsReserved(b *testing.B) {
	subdomain := "www"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsReserved(subdomain)
	}
}
