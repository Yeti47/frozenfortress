package ccc

import (
	"github.com/google/uuid"
)

// UuidGenerator is a unified UUID generator that implements all entity-specific ID generator interfaces.
// This single implementation can be used for generating IDs for users, secrets, documents, etc.
type UuidGenerator struct{}

// NewUuidGenerator creates a new instance of UuidGenerator.
func NewUuidGenerator() *UuidGenerator {
	return &UuidGenerator{}
}

// GenerateId generates a new UUID and returns it as a string.
// This method implements the common ID generation functionality for all entity types.
func (g *UuidGenerator) GenerateId() string {
	return uuid.NewString()
}
