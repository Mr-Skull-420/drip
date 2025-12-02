package utils

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// GenerateID generates a random unique ID
func GenerateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return generateFallbackID()
	}
	return hex.EncodeToString(b)
}

// GenerateShortID generates a shorter random ID (8 chars)
func GenerateShortID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return generateFallbackID()[:8]
	}
	return hex.EncodeToString(b)
}

func generateFallbackID() string {
	// Simple fallback using timestamp
	return hex.EncodeToString([]byte(time.Now().String()))
}
