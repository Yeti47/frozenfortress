package auth

import (
	"github.com/google/uuid"
)

// UuidUserIdGenerator generates UUIDs for user IDs
type UuidUserIdGenerator struct {
}

// NewUuidUserIdGenerator creates a new instance of UuidUserIdGenerator
func NewUuidUserIdGenerator() *UuidUserIdGenerator {
	return &UuidUserIdGenerator{}
}

// GenerateUserId implements the UserIdGenerator interface
// It generates a UUID and returns it as a string
func (g *UuidUserIdGenerator) GenerateUserId() string {
	return uuid.NewString()
}
