package scanhandoff

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// tokenByteLength is the amount of randomness backing a handoff token (256 bits).
const tokenByteLength = 32

// GenerateToken creates a new cryptographically random, URL-safe handoff token.
// It is a bearer credential for the session-less upload endpoint, not an entity id,
// so it deliberately doesn't go through ccc.UuidGenerator.
func GenerateToken() (string, error) {
	tokenBytes := make([]byte, tokenByteLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate scan handoff token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(tokenBytes), nil
}
