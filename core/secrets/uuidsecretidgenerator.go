package secrets

import (
	"github.com/google/uuid"
)

// UuidSecretIdGenerator implements the SecretIdGenerator interface using UUIDs.
type UuidSecretIdGenerator struct{}

// NewUuidSecretIdGenerator creates a new instance of UuidSecretIdGenerator.
func NewUuidSecretIdGenerator() *UuidSecretIdGenerator {
	return &UuidSecretIdGenerator{}
}

// GenerateSecretId generates a new UUID and returns it as a string.
func (g *UuidSecretIdGenerator) GenerateSecretId() string {
	return uuid.NewString()
}
