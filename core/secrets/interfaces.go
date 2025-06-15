package secrets

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/core/dataprotection"
)

// Interface for generating secret IDs
// This interface is used to generate unique IDs for secrets.
type SecretIdGenerator interface {
	GenerateId() string
}

type SecretRepository interface {
	FindById(secretId string) (*Secret, error)
	FindByUserId(userId string) ([]*Secret, error)
	FindByIdForUser(userId, secretId string) (*Secret, error)
	FindByNameForUser(userId, name string) (*Secret, error)
	Add(secret *Secret) (bool, error)
	Remove(secretId string) (bool, error)
	Update(secret *Secret) (bool, error)
}

// SecretManager interface for managing secrets
type SecretManager interface {
	CreateSecret(userId string, request UpsertSecretRequest, dataProtector dataprotection.DataProtector) (CreateSecretResponse, error)
	GetSecret(userId string, secretId string, dataProtector dataprotection.DataProtector) (*SecretDto, error)
	GetSecretByName(userId string, secretName string, dataProtector dataprotection.DataProtector) (*SecretDto, error)
	GetSecrets(userId string, request GetSecretsRequest, dataProtector dataprotection.DataProtector) (PaginatedSecretResponse, error)
	UpdateSecret(userId string, secretId string, request UpsertSecretRequest, dataProtector dataprotection.DataProtector) (bool, error)
	DeleteSecret(userId string, secretId string) (bool, error)
}
