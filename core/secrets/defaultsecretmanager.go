package secrets

import (
	"errors"
	"sort"
	"strings"
	"time"

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
	existingSecret, err := m.findSecretByNameForUser(userId, request.SecretName, dataProtector)

	if err != nil {
		return CreateSecretResponse{}, err
	}

	if existingSecret != nil {
		return CreateSecretResponse{}, errors.New("secret with this name already exists")
	}

	// Generate a new secret ID
	secretId := m.secretIdGenerator.GenerateSecretId()

	// Encrypt the secret name and value
	encryptedName, err := dataProtector.Protect(request.SecretName)
	if err != nil {
		return CreateSecretResponse{}, err
	}
	encryptedValue, err := dataProtector.Protect(request.SecretValue)
	if err != nil {
		return CreateSecretResponse{}, err
	}

	// Create a new secret object
	secret := &Secret{
		Id:         secretId,
		UserId:     userId,
		Name:       encryptedName,
		Value:      encryptedValue,
		CreatedAt:  time.Now(),
		ModifiedAt: time.Now(),
	}

	// Add the secret to the repository
	success, err := m.secretRepository.Add(secret)
	if err != nil || !success {
		return CreateSecretResponse{}, err
	}

	return CreateSecretResponse{SecretId: secretId}, nil
}

// GetSecret retrieves a secret by its ID and decrypts it using the provided data protector.
func (m *DefaultSecretManager) GetSecret(userId string, secretId string, dataProtector DataProtector) (*SecretDto, error) {

	// Retrieve the secret from the repository
	secret, err := m.secretRepository.FindByIdForUser(userId, secretId)
	if err != nil {
		return nil, err
	}

	if secret == nil {
		return nil, errors.New("secret not found")
	}

	// Decrypt the secret name and value
	decryptedName, err := dataProtector.Unprotect(secret.Name)
	if err != nil {
		return nil, err
	}
	decryptedValue, err := dataProtector.Unprotect(secret.Value)
	if err != nil {
		return nil, err
	}

	// Map the secret to a DTO
	secretDto := &SecretDto{
		Id:         secret.Id,
		UserId:     secret.UserId,
		Name:       decryptedName,
		Value:      decryptedValue,
		CreatedAt:  secret.CreatedAt.Format("2006-01-02 15:04:05"),
		ModifiedAt: secret.ModifiedAt.Format("2006-01-02 15:04:05"),
	}

	return secretDto, nil
}

// GetSecrets retrieves all secrets for a user with optional filtering and pagination
func (m *DefaultSecretManager) GetSecrets(userId string, request GetSecretsRequest, dataProtector DataProtector) (PaginatedSecretResponse, error) {
	// Get all secrets for the user from repository
	allSecrets, err := m.secretRepository.FindByUserId(userId)
	if err != nil {
		return PaginatedSecretResponse{}, err
	}

	// Decrypt names and filter in memory
	var filteredSecrets []*Secret
	for _, secret := range allSecrets {
		// Decrypt the name to check if it matches the filter
		decryptedName, err := dataProtector.Unprotect(secret.Name)
		if err != nil {
			// Skip secrets we can't decrypt
			continue
		}

		// Apply name filter if specified (case-insensitive partial match)
		if request.Name != "" {
			nameMatches := strings.Contains(strings.ToLower(decryptedName), strings.ToLower(request.Name))
			if !nameMatches {
				continue
			}
		}

		// Create a copy with decrypted name for sorting/pagination
		secretCopy := *secret
		secretCopy.Name = decryptedName
		filteredSecrets = append(filteredSecrets, &secretCopy)
	}

	// Sort the filtered secrets
	m.sortSecrets(filteredSecrets, request.SortBy, request.SortAsc)

	totalCount := len(filteredSecrets)

	// Apply pagination
	startIndex := 0
	endIndex := totalCount
	if request.PageSize > 0 && request.Page > 0 {
		startIndex = min((request.Page-1)*request.PageSize, totalCount)
		endIndex = min(startIndex+request.PageSize, totalCount)
	}

	paginatedSecrets := filteredSecrets[startIndex:endIndex]

	// Convert to DTOs
	secretDtos := make([]*SecretDto, len(paginatedSecrets))
	for i, secret := range paginatedSecrets {
		// Decrypt the value for the DTO
		decryptedValue, err := dataProtector.Unprotect(secret.Value)
		if err != nil {
			// Skip secrets we can't decrypt
			continue
		}

		secretDtos[i] = &SecretDto{
			Id:         secret.Id,
			UserId:     secret.UserId,
			Name:       secret.Name, // Already decrypted above
			Value:      decryptedValue,
			CreatedAt:  secret.CreatedAt.Format("2006-01-02 15:04:05"),
			ModifiedAt: secret.ModifiedAt.Format("2006-01-02 15:04:05"),
		}
	}

	return PaginatedSecretResponse{
		TotalCount: totalCount,
		Page:       request.Page,
		PageSize:   request.PageSize,
		Secrets:    secretDtos,
	}, nil
}

// sortSecrets sorts the slice of secrets based on the provided criteria
func (m *DefaultSecretManager) sortSecrets(secrets []*Secret, sortBy string, sortAsc bool) {
	if len(secrets) <= 1 {
		return
	}

	// Sort function
	sort.Slice(secrets, func(i, j int) bool {
		var comparison int
		switch sortBy {
		case "Id":
			comparison = strings.Compare(secrets[i].Id, secrets[j].Id)
		case "Name":
			comparison = strings.Compare(strings.ToLower(secrets[i].Name), strings.ToLower(secrets[j].Name))
		case "CreatedAt":
			if secrets[i].CreatedAt.Before(secrets[j].CreatedAt) {
				comparison = -1
			} else if secrets[i].CreatedAt.After(secrets[j].CreatedAt) {
				comparison = 1
			} else {
				comparison = 0
			}
		case "ModifiedAt":
			if secrets[i].ModifiedAt.Before(secrets[j].ModifiedAt) {
				comparison = -1
			} else if secrets[i].ModifiedAt.After(secrets[j].ModifiedAt) {
				comparison = 1
			} else {
				comparison = 0
			}
		default:
			// Default sort by CreatedAt desc
			if secrets[i].CreatedAt.After(secrets[j].CreatedAt) {
				comparison = -1
			} else if secrets[i].CreatedAt.Before(secrets[j].CreatedAt) {
				comparison = 1
			} else {
				comparison = 0
			}
		}

		if sortAsc {
			return comparison < 0
		} else {
			return comparison > 0
		}
	})
}

// UpdateSecret updates an existing secret
func (m *DefaultSecretManager) UpdateSecret(userId string, secretId string, request UpsertSecretRequest, dataProtector DataProtector) (bool, error) {
	// Validate the request
	if request.SecretName == "" {
		return false, errors.New("secret name cannot be empty")
	}
	if request.SecretValue == "" {
		return false, errors.New("secret value cannot be empty")
	}

	// Get the existing secret
	existingSecret, err := m.secretRepository.FindByIdForUser(userId, secretId)
	if err != nil {
		return false, err
	}
	if existingSecret == nil {
		return false, errors.New("secret not found")
	}

	// Check if another secret with the new name already exists (excluding current secret)
	conflictingSecret, err := m.findSecretByNameForUser(userId, request.SecretName, dataProtector)
	if err != nil {
		return false, err
	}
	if conflictingSecret != nil && conflictingSecret.Id != secretId {
		return false, errors.New("secret with this name already exists")
	}

	// Encrypt the new name and value
	encryptedName, err := dataProtector.Protect(request.SecretName)
	if err != nil {
		return false, err
	}
	encryptedValue, err := dataProtector.Protect(request.SecretValue)
	if err != nil {
		return false, err
	}

	// Update the secret
	existingSecret.Name = encryptedName
	existingSecret.Value = encryptedValue
	existingSecret.ModifiedAt = time.Now()

	return m.secretRepository.Update(existingSecret)
}

// findSecretByNameForUser is a helper method to find a secret by decrypting names in memory
func (m *DefaultSecretManager) findSecretByNameForUser(userId, secretName string, dataProtector DataProtector) (*Secret, error) {
	// Get all secrets for the user
	allSecrets, err := m.secretRepository.FindByUserId(userId)
	if err != nil {
		return nil, err
	}

	// Search through secrets by decrypting names
	for _, secret := range allSecrets {
		decryptedName, err := dataProtector.Unprotect(secret.Name)
		if err != nil {
			// Skip secrets we can't decrypt
			continue
		}
		if decryptedName == secretName {
			return secret, nil
		}
	}

	return nil, nil // Not found
}
