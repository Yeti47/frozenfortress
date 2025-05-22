package secrets

// Interface for generating secret IDs
// This interface is used to generate unique IDs for secrets.
type SecretIdGenerator interface {
	GenerateSecretId() string
}

type SecretRepository interface {
	FindById(secretId string) (*Secret, error)
	FindByUserId(userId string) ([]*Secret, error)
	FindByName(userId, secretName string) (*Secret, error)
	Filter(filter SecretFilter) (secrets []*Secret, totalCount int, err error)
	Add(secret Secret) (bool, error)
	Remove(secretId string) (bool, error)
	Update(secret *Secret) (bool, error)
}
