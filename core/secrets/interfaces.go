package secrets

// Interface for generating secret IDs
// This interface is used to generate unique IDs for secrets.
type SecretIdGenerator interface {
	GenerateSecretId() string
}

type SecretRepository interface {
	FindById(secretId string) (*Secret, error)
	FindByUserId(userId string) ([]*Secret, error)
	FindByIdForUser(userId, secretId string) (*Secret, error)
	Add(secret *Secret) (bool, error)
	Remove(secretId string) (bool, error)
	Update(secret *Secret) (bool, error)
}

// SecretManager interface for managing secrets
type SecretManager interface {
	CreateSecret(userId string, request UpsertSecretRequest, dataProtector DataProtector) (CreateSecretResponse, error)
	GetSecret(userId string, secretId string, dataProtector DataProtector) (*SecretDto, error)
	GetSecrets(userId string, request GetSecretsRequest, dataProtector DataProtector) (PaginatedSecretResponse, error)
	UpdateSecret(userId string, secretId string, request UpsertSecretRequest, dataProtector DataProtector) (bool, error)
}

type DataProtector interface {
	Protect(data string) (protectedData string, err error)
	Unprotect(protectedData string) (data string, err error)
}
