package secrets

import (
	"sort"
	"strings"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/dataprotection"
)

type DefaultSecretManager struct {
	secretRepository  SecretRepository
	secretIdGenerator SecretIdGenerator
	userRepository    auth.UserRepository
	logger            ccc.Logger
}

func NewDefaultSecretManager(secretRepository SecretRepository, secretIdGenerator SecretIdGenerator, userRepository auth.UserRepository, logger ccc.Logger) *DefaultSecretManager {
	if logger == nil {
		logger = ccc.NopLogger
	}

	return &DefaultSecretManager{
		secretRepository:  secretRepository,
		secretIdGenerator: secretIdGenerator,
		userRepository:    userRepository,
		logger:            logger,
	}
}

// validateSecretRequest validates the secret name and value lengths
func (m *DefaultSecretManager) validateSecretRequest(request UpsertSecretRequest) error {
	const (
		maxSecretNameLength  = 200
		maxSecretValueLength = 1000
	)

	if request.SecretName == "" {
		return ccc.NewInvalidInputError("secret name", "cannot be empty")
	}
	if len(request.SecretName) > maxSecretNameLength {
		return ccc.NewInvalidInputErrorWithMessage(
			"secret name",
			"exceeds maximum length of 200 characters",
			"Secret name cannot be longer than 200 characters",
		)
	}
	if request.SecretValue == "" {
		return ccc.NewInvalidInputError("secret value", "cannot be empty")
	}
	if len(request.SecretValue) > maxSecretValueLength {
		return ccc.NewInvalidInputErrorWithMessage(
			"secret value",
			"exceeds maximum length of 1000 characters",
			"Secret value cannot be longer than 1000 characters",
		)
	}
	return nil
}

func (m *DefaultSecretManager) CreateSecret(userId string, request UpsertSecretRequest, dataProtector dataprotection.DataProtector) (CreateSecretResponse, error) {
	m.logger.Info("Creating secret", "user_id", userId, "secret_name", request.SecretName)

	// Trim whitespace from input
	request.SecretName = strings.TrimSpace(request.SecretName)
	request.SecretValue = strings.TrimSpace(request.SecretValue)

	// Validate the request
	if err := m.validateSecretRequest(request); err != nil {
		m.logger.Warn("Secret creation failed: validation error", "user_id", userId, "error", err)
		return CreateSecretResponse{}, err
	}

	// Check if the user exists
	user, err := m.userRepository.FindById(userId)

	if err != nil {
		m.logger.Error("Failed to find user during secret creation", "user_id", userId, "error", err)
		return CreateSecretResponse{}, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		m.logger.Warn("User not found for secret creation", "user_id", userId)
		return CreateSecretResponse{}, ccc.NewResourceNotFoundError(userId, "User")
	}

	m.logger.Debug("User verified for secret creation", "user_id", userId, "username", user.UserName)

	// Check if the secret already exists
	existingSecret, err := m.GetSecretByName(userId, request.SecretName, dataProtector)
	if err != nil && !ccc.IsNotFound(err) {
		// If the error is anything other than "Resource Not Found", it's an actual error.
		m.logger.Error("Failed to check for existing secret", "user_id", userId, "secret_name", request.SecretName, "error", err)
		return CreateSecretResponse{}, ccc.NewDatabaseError("find existing secret by name", err)
	}
	// If it is ResourceNotFoundError, it means the secret doesn't exist, which is good.
	if existingSecret != nil {
		m.logger.Warn("Secret already exists, cannot create duplicate", "user_id", userId, "secret_name", request.SecretName)
		return CreateSecretResponse{}, ccc.NewResourceAlreadyExistsError(request.SecretName, "Secret")
	}

	// Generate a new secret ID
	secretId := m.secretIdGenerator.GenerateId()
	m.logger.Debug("Generated secret ID", "secret_id", secretId, "user_id", userId)

	// Encrypt the secret name and value
	encryptedName, err := dataProtector.Protect(request.SecretName)
	if err != nil {
		m.logger.Error("Failed to encrypt secret name", "user_id", userId, "secret_name", request.SecretName, "error", err)
		return CreateSecretResponse{}, ccc.NewInternalError("failed to encrypt secret name", err)
	}
	encryptedValue, err := dataProtector.Protect(request.SecretValue)
	if err != nil {
		m.logger.Error("Failed to encrypt secret value", "user_id", userId, "secret_name", request.SecretName, "error", err)
		return CreateSecretResponse{}, ccc.NewInternalError("failed to encrypt secret value", err)
	}

	m.logger.Debug("Secret data encrypted successfully", "user_id", userId, "secret_id", secretId)

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
		m.logger.Error("Failed to add secret to repository", "user_id", userId, "secret_id", secretId, "success", success, "error", err)
		return CreateSecretResponse{}, ccc.NewDatabaseError("add secret", err)
	}

	m.logger.Info("Secret created successfully", "user_id", userId, "secret_id", secretId, "secret_name", request.SecretName)
	return CreateSecretResponse{SecretId: secretId}, nil
}

// GetSecret retrieves a secret by its ID and decrypts it using the provided data protector.
func (m *DefaultSecretManager) GetSecret(userId string, secretId string, dataProtector dataprotection.DataProtector) (*SecretDto, error) {
	m.logger.Debug("Retrieving secret by ID", "user_id", userId, "secret_id", secretId)

	// Retrieve the secret from the repository
	secret, err := m.secretRepository.FindByIdForUser(userId, secretId)
	if err != nil {
		m.logger.Error("Failed to find secret by ID", "user_id", userId, "secret_id", secretId, "error", err)
		return nil, ccc.NewDatabaseError("find secret by ID", err)
	}

	if secret == nil {
		m.logger.Warn("Secret not found", "user_id", userId, "secret_id", secretId)
		return nil, ccc.NewResourceNotFoundError(secretId, "Secret")
	}

	// Decrypt the secret name and value
	decryptedName, err := dataProtector.Unprotect(secret.Name)
	if err != nil {
		m.logger.Error("Failed to decrypt secret name", "user_id", userId, "secret_id", secretId, "error", err)
		return nil, ccc.NewInternalError("failed to decrypt secret name", err)
	}
	decryptedValue, err := dataProtector.Unprotect(secret.Value)
	if err != nil {
		m.logger.Error("Failed to decrypt secret value", "user_id", userId, "secret_id", secretId, "error", err)
		return nil, ccc.NewInternalError("failed to decrypt secret value", err)
	}

	m.logger.Debug("Secret retrieved and decrypted successfully", "user_id", userId, "secret_id", secretId, "secret_name", decryptedName)

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

// GetSecretByName retrieves a secret by its name for a specific user and decrypts it.
func (m *DefaultSecretManager) GetSecretByName(userId string, secretName string, dataProtector dataprotection.DataProtector) (*SecretDto, error) {
	m.logger.Debug("Retrieving secret by name", "user_id", userId, "secret_name", secretName)

	// Encrypt the secret name to search for it in the repository
	encryptedName, err := dataProtector.Protect(secretName)
	if err != nil {
		m.logger.Error("Failed to encrypt secret name for lookup", "user_id", userId, "secret_name", secretName, "error", err)
		return nil, ccc.NewInternalError("failed to encrypt secret name for lookup", err)
	}

	// Retrieve the secret from the repository using the encrypted name
	secret, err := m.secretRepository.FindByNameForUser(userId, encryptedName)
	if err != nil {
		m.logger.Error("Failed to find secret by name", "user_id", userId, "secret_name", secretName, "error", err)
		return nil, ccc.NewDatabaseError("find secret by name", err)
	}

	if secret == nil {
		m.logger.Debug("Secret not found by name", "user_id", userId, "secret_name", secretName)
		return nil, ccc.NewResourceNotFoundError(secretName, "Secret")
	}

	// Decrypt the secret value
	decryptedValue, err := dataProtector.Unprotect(secret.Value)
	if err != nil {
		m.logger.Error("Failed to decrypt secret value", "user_id", userId, "secret_name", secretName, "error", err)
		return nil, ccc.NewInternalError("failed to decrypt secret value", err)
	}

	m.logger.Debug("Secret retrieved by name and decrypted successfully", "user_id", userId, "secret_name", secretName)

	// Map the secret to a DTO
	secretDto := &SecretDto{
		Id:         secret.Id,
		UserId:     secret.UserId,
		Name:       secretName, // Use the original, unencrypted name for the DTO
		Value:      decryptedValue,
		CreatedAt:  secret.CreatedAt.Format("2006-01-02 15:04:05"),
		ModifiedAt: secret.ModifiedAt.Format("2006-01-02 15:04:05"),
	}

	return secretDto, nil
}

// GetSecrets retrieves all secrets for a user with optional filtering and pagination
func (m *DefaultSecretManager) GetSecrets(userId string, request GetSecretsRequest, dataProtector dataprotection.DataProtector) (PaginatedSecretResponse, error) {
	m.logger.Info("Retrieving secrets for user", "user_id", userId, "name_filter", request.Name, "page", request.Page, "page_size", request.PageSize)

	// Get all secrets for the user from repository
	allSecrets, err := m.secretRepository.FindByUserId(userId)
	if err != nil {
		m.logger.Error("Failed to find secrets by user ID", "user_id", userId, "error", err)
		return PaginatedSecretResponse{}, ccc.NewDatabaseError("find secrets by user ID", err)
	}

	m.logger.Debug("Retrieved secrets from repository", "user_id", userId, "total_secrets", len(allSecrets))

	// Decrypt names and filter in memory
	var filteredSecrets []*Secret
	decryptionErrors := 0
	for _, secret := range allSecrets {
		// Decrypt the name to check if it matches the filter
		decryptedName, err := dataProtector.Unprotect(secret.Name)
		if err != nil {
			// Skip secrets we can't decrypt
			decryptionErrors++
			m.logger.Warn("Failed to decrypt secret name during filtering, skipping", "user_id", userId, "secret_id", secret.Id, "error", err)
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

	if decryptionErrors > 0 {
		m.logger.Warn("Some secrets could not be decrypted and were skipped", "user_id", userId, "decryption_errors", decryptionErrors)
	}

	// Sort the filtered secrets
	m.sortSecrets(filteredSecrets, request.SortBy, request.SortAsc)

	totalCount := len(filteredSecrets)
	m.logger.Debug("Secrets filtered and sorted", "user_id", userId, "filtered_count", totalCount, "sort_by", request.SortBy, "sort_asc", request.SortAsc)

	// Apply pagination
	startIndex := 0
	endIndex := totalCount
	if request.PageSize > 0 && request.Page > 0 {
		startIndex = min((request.Page-1)*request.PageSize, totalCount)
		endIndex = min(startIndex+request.PageSize, totalCount)
	}

	paginatedSecrets := filteredSecrets[startIndex:endIndex]
	m.logger.Debug("Applied pagination", "user_id", userId, "start_index", startIndex, "end_index", endIndex, "paginated_count", len(paginatedSecrets))

	// Convert to DTOs
	secretDtos := make([]*SecretDto, 0, len(paginatedSecrets))
	valueDecryptionErrors := 0
	for _, secret := range paginatedSecrets {
		// Decrypt the value for the DTO
		decryptedValue, err := dataProtector.Unprotect(secret.Value)
		if err != nil {
			// Skip secrets we can't decrypt
			valueDecryptionErrors++
			m.logger.Warn("Failed to decrypt secret value during DTO conversion, skipping", "user_id", userId, "secret_id", secret.Id, "error", err)
			continue
		}

		secretDtos = append(secretDtos, &SecretDto{
			Id:         secret.Id,
			UserId:     secret.UserId,
			Name:       secret.Name, // Already decrypted above
			Value:      decryptedValue,
			CreatedAt:  secret.CreatedAt.Format("2006-01-02 15:04:05"),
			ModifiedAt: secret.ModifiedAt.Format("2006-01-02 15:04:05"),
		})
	}

	if valueDecryptionErrors > 0 {
		m.logger.Warn("Some secret values could not be decrypted during DTO conversion", "user_id", userId, "value_decryption_errors", valueDecryptionErrors)
	}

	m.logger.Info("Successfully retrieved secrets for user", "user_id", userId, "returned_count", len(secretDtos), "total_count", totalCount)

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
func (m *DefaultSecretManager) UpdateSecret(userId string, secretId string, request UpsertSecretRequest, dataProtector dataprotection.DataProtector) (bool, error) {
	m.logger.Info("Updating secret", "user_id", userId, "secret_id", secretId, "new_secret_name", request.SecretName)

	// Trim whitespace from input
	request.SecretName = strings.TrimSpace(request.SecretName)
	request.SecretValue = strings.TrimSpace(request.SecretValue)

	// Validate the request
	if err := m.validateSecretRequest(request); err != nil {
		m.logger.Warn("Secret update failed: validation error", "user_id", userId, "secret_id", secretId, "error", err)
		return false, err
	}

	// Get the existing secret
	existingSecret, err := m.secretRepository.FindByIdForUser(userId, secretId)
	if err != nil {
		m.logger.Error("Failed to find existing secret for update", "user_id", userId, "secret_id", secretId, "error", err)
		return false, ccc.NewDatabaseError("find secret by ID", err)
	}
	if existingSecret == nil {
		m.logger.Warn("Secret not found for update", "user_id", userId, "secret_id", secretId)
		return false, ccc.NewResourceNotFoundError(secretId, "Secret")
	}

	// Decrypt the existing secret's name to check if it's being changed.
	decryptedCurrentName, err := dataProtector.Unprotect(existingSecret.Name)
	if err != nil {
		m.logger.Error("Failed to decrypt current secret name during update", "user_id", userId, "secret_id", secretId, "error", err)
		return false, ccc.NewInternalError("failed to decrypt current secret name", err)
	}

	m.logger.Debug("Current secret name decrypted", "user_id", userId, "secret_id", secretId, "current_name", decryptedCurrentName)

	// If the name is being changed, check if the new name conflicts with another existing secret.
	if decryptedCurrentName != request.SecretName {
		m.logger.Debug("Secret name is being changed, checking for conflicts", "user_id", userId, "secret_id", secretId, "old_name", decryptedCurrentName, "new_name", request.SecretName)
		conflictingSecret, err := m.GetSecretByName(userId, request.SecretName, dataProtector)
		if err != nil && !ccc.IsNotFound(err) {
			// If the error is anything other than "Resource Not Found", it's an actual error.
			m.logger.Error("Failed to check for conflicting secret name during update", "user_id", userId, "secret_id", secretId, "new_name", request.SecretName, "error", err)
			return false, ccc.NewDatabaseError("find conflicting secret by name", err)
		}
		// If it is ResourceNotFoundError, it means no conflicting secret exists, which is good.
		if conflictingSecret != nil && conflictingSecret.Id != secretId { // Ensure it's not the same secret
			m.logger.Warn("Secret name conflict detected during update", "user_id", userId, "secret_id", secretId, "conflicting_secret_id", conflictingSecret.Id, "new_name", request.SecretName)
			return false, ccc.NewResourceAlreadyExistsError(request.SecretName, "Secret")
		}
	}

	// Encrypt the new name and value
	encryptedName, err := dataProtector.Protect(request.SecretName)
	if err != nil {
		m.logger.Error("Failed to encrypt new secret name during update", "user_id", userId, "secret_id", secretId, "new_name", request.SecretName, "error", err)
		return false, ccc.NewInternalError("failed to encrypt secret name", err)
	}
	encryptedValue, err := dataProtector.Protect(request.SecretValue)
	if err != nil {
		m.logger.Error("Failed to encrypt new secret value during update", "user_id", userId, "secret_id", secretId, "error", err)
		return false, ccc.NewInternalError("failed to encrypt secret value", err)
	}

	m.logger.Debug("New secret data encrypted successfully", "user_id", userId, "secret_id", secretId)

	// Update the existing secret with new values
	existingSecret.Name = encryptedName
	existingSecret.Value = encryptedValue
	existingSecret.ModifiedAt = time.Now()
	// Update the secret in the repository
	success, err := m.secretRepository.Update(existingSecret)
	if err != nil {
		m.logger.Error("Failed to update secret in repository", "user_id", userId, "secret_id", secretId, "error", err)
		return false, ccc.NewDatabaseError("update secret", err)
	}
	if !success {
		m.logger.Error("Secret update reported no success but no error", "user_id", userId, "secret_id", secretId)
		return false, ccc.NewInternalError("update secret reported no success but no error", nil)
	}

	m.logger.Info("Secret updated successfully", "user_id", userId, "secret_id", secretId, "secret_name", request.SecretName)
	return true, nil
}

// DeleteSecret deletes a secret by its ID
func (m *DefaultSecretManager) DeleteSecret(userId string, secretId string) (bool, error) {
	m.logger.Info("Deleting secret", "user_id", userId, "secret_id", secretId)

	// Delete the secret from the repository
	success, err := m.secretRepository.Remove(secretId)
	if err != nil {
		m.logger.Error("Failed to delete secret from repository", "user_id", userId, "secret_id", secretId, "error", err)
		return false, ccc.NewDatabaseError("remove secret by ID", err)
	}

	if success {
		m.logger.Info("Secret deleted successfully", "user_id", userId, "secret_id", secretId)
	} else {
		m.logger.Debug("Secret deletion was idempotent (secret may not have existed)", "user_id", userId, "secret_id", secretId)
	}

	// Secret deletion is idempotent. We indicate success, but never return an error if the secret doesn't exist.
	// We simply return the success status.
	return success, nil
}
