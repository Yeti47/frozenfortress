package secrets

import (
	"errors"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
)

type DefaultSecretManager struct {
	secretRepository  SecretRepository
	secretIdGenerator SecretIdGenerator
	userRepository    auth.UserRepository
}

func NewDefaultSecretManager(secretRepository SecretRepository, secretIdGenerator SecretIdGenerator, userRepository auth.UserRepository) *DefaultSecretManager {
	return &DefaultSecretManager{
		secretRepository:  secretRepository,
		secretIdGenerator: secretIdGenerator,
		userRepository:    userRepository,
	}
}
func (m *DefaultSecretManager) CreateSecret(userId string, request UpsertSecretRequest, dataProtector DataProtector) (CreateSecretResponse, error) {

	// Validate the request
	if request.SecretName == "" {
		return CreateSecretResponse{}, errors.New("secret name cannot be empty")
	}
	if request.SecretValue == "" {
		return CreateSecretResponse{}, errors.New("secret value cannot be empty")
	}

	// Check if the user exists
	user, err := m.userRepository.FindById(userId)

	if err != nil {
		return CreateSecretResponse{}, err
	}

	if user == nil {
		return CreateSecretResponse{}, errors.New("user not found")
	}

	// Check if the secret already exists
	existingSecret, err := m.secretRepository.FindByName(userId, request.SecretName)

	if err != nil {
		return CreateSecretResponse{}, err
	}

	if existingSecret != nil {
		return CreateSecretResponse{}, errors.New("secret with this name already exists")
	}

	// Generate a new secret ID
	secretId := m.secretIdGenerator.GenerateSecretId()

	// Encrypt the secret value
	encryptedValue, err := dataProtector.Protect(request.SecretValue)
	if err != nil {
		return CreateSecretResponse{}, err
	}

	// Create a new secret object
	secret := &Secret{
		Id:     secretId,
		UserId: userId,
		Name:   request.SecretName,
		Value:  encryptedValue,
	}

	// Add the secret to the repository
	success, err := m.secretRepository.Add(secret)
	if err != nil || !success {
		return CreateSecretResponse{}, err
	}

	return CreateSecretResponse{SecretId: secretId}, nil
}
