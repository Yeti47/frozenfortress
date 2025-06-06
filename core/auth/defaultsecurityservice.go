package auth

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
)

type DefaultSecurityService struct {
	userRepository    UserRepository
	encryptionService encryption.EncryptionService
	logger            ccc.Logger
}

func NewDefaultSecurityService(userRepository UserRepository, encryptionService encryption.EncryptionService, logger ccc.Logger) *DefaultSecurityService {
	if logger == nil {
		logger = ccc.NopLogger
	}

	return &DefaultSecurityService{
		userRepository:    userRepository,
		encryptionService: encryptionService,
		logger:            logger,
	}
}

// LockUser locks the user account.
func (s *DefaultSecurityService) LockUser(user User) (bool, error) {
	s.logger.Info("Locking user account", "user_id", user.Id, "username", user.UserName)

	user.IsLocked = true

	// TODO: Add logic to log the lock action

	// Update the user in the repository
	updated, err := s.userRepository.Update(&user)
	if err != nil {
		s.logger.Error("Failed to lock user account", "user_id", user.Id, "username", user.UserName, "error", err)
		return false, err
	}

	if updated {
		s.logger.Info("User account locked successfully", "user_id", user.Id, "username", user.UserName)
	} else {
		s.logger.Warn("User account lock operation returned false", "user_id", user.Id, "username", user.UserName)
	}

	return updated, nil
}

// UnlockUser unlocks the user account.
func (s *DefaultSecurityService) UnlockUser(user User) (bool, error) {
	s.logger.Info("Unlocking user account", "user_id", user.Id, "username", user.UserName)

	user.IsLocked = false

	// Update the user in the repository
	updated, err := s.userRepository.Update(&user)
	if err != nil {
		s.logger.Error("Failed to unlock user account", "user_id", user.Id, "username", user.UserName, "error", err)
		return false, err
	}

	if updated {
		s.logger.Info("User account unlocked successfully", "user_id", user.Id, "username", user.UserName)
	} else {
		s.logger.Warn("User account unlock operation returned false", "user_id", user.Id, "username", user.UserName)
	}

	return updated, nil
}

// VerifyUserPassword verifies the user's password.
func (s *DefaultSecurityService) VerifyUserPassword(user User, password string) (bool, error) {
	s.logger.Debug("Verifying user password", "user_id", user.Id, "username", user.UserName)

	isValid, err := s.encryptionService.VerifyHash(password, user.PasswordHash, user.PasswordSalt)
	if err != nil {
		s.logger.Error("Failed to verify user password", "user_id", user.Id, "username", user.UserName, "error", err)
		return false, err
	}

	if isValid {
		s.logger.Debug("User password verification successful", "user_id", user.Id, "username", user.UserName)
	} else {
		s.logger.Warn("User password verification failed", "user_id", user.Id, "username", user.UserName)
	}

	return isValid, nil
}

// UncoverMek reads the user's MEK (Master Encryption Key) from the database.
func (s *DefaultSecurityService) UncoverMek(user User, password string) (string, error) {
	s.logger.Debug("Uncovering MEK for user", "user_id", user.Id, "username", user.UserName)

	// Verify the user's password
	isValid, err := s.VerifyUserPassword(user, password)
	if err != nil {
		s.logger.Error("Failed to verify password during MEK uncovering", "user_id", user.Id, "username", user.UserName, "error", err)
		return "", err
	}
	if !isValid {
		s.logger.Warn("Invalid password provided for MEK uncovering", "user_id", user.Id, "username", user.UserName)
		return "", nil
	}

	// Restore the PDK (Password-Derived Key) via the given password and stored salt
	pdk, err := s.encryptionService.GenerateKeyFromPassword(password, user.PdkSalt)
	if err != nil {
		s.logger.Error("Failed to generate PDK during MEK uncovering", "user_id", user.Id, "username", user.UserName, "error", err)
		return "", err
	}

	// Decrypt the MEK (Master Encryption Key) using the PDK
	mek, err := s.encryptionService.Decrypt(user.Mek, pdk)
	if err != nil {
		s.logger.Error("Failed to decrypt MEK", "user_id", user.Id, "username", user.UserName, "error", err)
		return "", err
	}

	s.logger.Debug("MEK uncovered successfully", "user_id", user.Id, "username", user.UserName)
	return mek, nil
}

// EncryptMek encrypts the user's MEK (Master Encryption Key) using the provided password.
func (s *DefaultSecurityService) EncryptMek(plainMek string, password string) (ecnryptedMek string, salt string, err error) {
	s.logger.Debug("Encrypting MEK with password")

	// Generate a random salt
	_, salt, err = s.encryptionService.GenerateSalt()
	if err != nil {
		s.logger.Error("Failed to generate salt for MEK encryption", "error", err)
		return "", "", err
	}

	// Generate the PDK (Password-Derived Key) using the password and salt
	pdk, err := s.encryptionService.GenerateKeyFromPassword(password, salt)
	if err != nil {
		s.logger.Error("Failed to generate PDK for MEK encryption", "error", err)
		return "", "", err
	}

	// Encrypt the MEK using the PDK
	ecnryptedMek, err = s.encryptionService.Encrypt(plainMek, pdk)
	if err != nil {
		s.logger.Error("Failed to encrypt MEK", "error", err)
		return "", "", err
	}

	s.logger.Debug("MEK encrypted successfully")
	return ecnryptedMek, salt, nil
}

// GenerateEncryptedMek generates an encrypted MEK using the user's password.
func (s *DefaultSecurityService) GenerateEncryptedMek(password string) (encryptedMek string, salt string, err error) {
	s.logger.Debug("Generating new encrypted MEK")

	// Generate a random salt
	_, salt, err = s.encryptionService.GenerateSalt()
	if err != nil {
		s.logger.Error("Failed to generate salt for new MEK", "error", err)
		return "", "", err
	}

	// Generate the PDK (Password-Derived Key) using the password and salt
	pdk, err := s.encryptionService.GenerateKeyFromPassword(password, salt)
	if err != nil {
		s.logger.Error("Failed to generate PDK for new MEK", "error", err)
		return "", "", err
	}

	// Generate a random MEK (Master Encryption Key)
	mek, err := s.encryptionService.GenerateKey()
	if err != nil {
		s.logger.Error("Failed to generate new MEK", "error", err)
		return "", "", err
	}

	// Encrypt the MEK using the PDK
	encryptedMek, err = s.encryptionService.Encrypt(mek, pdk)
	if err != nil {
		s.logger.Error("Failed to encrypt new MEK", "error", err)
		return "", "", err
	}

	s.logger.Debug("New encrypted MEK generated successfully")
	return encryptedMek, salt, nil
}

// GenerateRecoveryCode generates a new recovery code for a user.
func (s *DefaultSecurityService) GenerateRecoveryCode() (recoveryCode string, hash string, salt string, err error) {
	s.logger.Debug("Generating new recovery code")

	// Generate a 32-character recovery code using alphanumeric characters
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const codeLength = 32

	// Generate random bytes for the recovery code
	randomBytes, err := s.encryptionService.GenerateRandomBytes(codeLength)
	if err != nil {
		s.logger.Error("Failed to generate random bytes for recovery code", "error", err)
		return "", "", "", err
	}

	// Convert random bytes to recovery code format
	recoveryCodeBytes := make([]byte, codeLength)
	for i := range codeLength {
		recoveryCodeBytes[i] = charset[randomBytes[i]%byte(len(charset))]
	}
	recoveryCode = string(recoveryCodeBytes)

	// Hash the recovery code
	hash, salt, err = s.encryptionService.Hash(recoveryCode)
	if err != nil {
		s.logger.Error("Failed to hash recovery code", "error", err)
		return "", "", "", err
	}

	s.logger.Debug("Recovery code generated successfully")
	return recoveryCode, hash, salt, nil
}

// VerifyRecoveryCode verifies a recovery code against the stored hash.
func (s *DefaultSecurityService) VerifyRecoveryCode(user User, recoveryCode string) (bool, error) {
	s.logger.Debug("Verifying recovery code", "user_id", user.Id, "username", user.UserName)

	// Check if user has a recovery code
	if user.RecoveryCodeHash == "" || user.RecoveryCodeSalt == "" {
		s.logger.Warn("User has no recovery code set", "user_id", user.Id, "username", user.UserName)
		return false, nil
	}

	// Verify the recovery code
	isValid, err := s.encryptionService.VerifyHash(recoveryCode, user.RecoveryCodeHash, user.RecoveryCodeSalt)
	if err != nil {
		s.logger.Error("Failed to verify recovery code", "user_id", user.Id, "username", user.UserName, "error", err)
		return false, err
	}

	if isValid {
		s.logger.Debug("Recovery code verification successful", "user_id", user.Id, "username", user.UserName)
	} else {
		s.logger.Warn("Recovery code verification failed", "user_id", user.Id, "username", user.UserName)
	}

	return isValid, nil
}

// RecoverMek recovers the user's MEK using recovery code and re-encrypts with new password.
func (s *DefaultSecurityService) RecoverMek(user User, recoveryCode string, newPassword string) (newMek string, newPdkSalt string, err error) {
	s.logger.Debug("Recovering MEK with recovery code", "user_id", user.Id, "username", user.UserName)

	// First verify the recovery code
	isValid, err := s.VerifyRecoveryCode(user, recoveryCode)
	if err != nil {
		s.logger.Error("Failed to verify recovery code during MEK recovery", "user_id", user.Id, "username", user.UserName, "error", err)
		return "", "", err
	}
	if !isValid {
		s.logger.Warn("Invalid recovery code provided for MEK recovery", "user_id", user.Id, "username", user.UserName)
		return "", "", ccc.NewInvalidInputError("recovery code", "invalid recovery code")
	}

	// Decrypt the original MEK using the recovery code
	// Generate a key from the recovery code using PBKDF2 (matching the encryption process)
	recoveryKey, err := s.encryptionService.GenerateKeyFromPassword(recoveryCode, user.RecoveryCodeSalt)
	if err != nil {
		s.logger.Error("Failed to generate key from recovery code during MEK recovery", "user_id", user.Id, "username", user.UserName, "error", err)
		return "", "", ccc.NewInternalError("generate key from recovery code", err)
	}

	originalMek, err := s.encryptionService.Decrypt(user.RecoveryMek, recoveryKey)
	if err != nil {
		s.logger.Error("Failed to decrypt original MEK with recovery code", "user_id", user.Id, "username", user.UserName, "error", err)
		return "", "", ccc.NewInternalError("decrypt MEK with recovery code", err)
	}

	// Re-encrypt the original MEK with the new password
	newMek, newPdkSalt, err = s.EncryptMek(originalMek, newPassword)
	if err != nil {
		s.logger.Error("Failed to encrypt MEK with new password during recovery", "user_id", user.Id, "username", user.UserName, "error", err)
		return "", "", err
	}

	s.logger.Debug("MEK recovery completed successfully", "user_id", user.Id, "username", user.UserName)
	return newMek, newPdkSalt, nil
}

// EncryptMekWithRecoveryCode encrypts the MEK with recovery code for recovery purposes.
func (s *DefaultSecurityService) EncryptMekWithRecoveryCode(plainMek string, recoveryCode string, salt string) (encryptedMek string, err error) {
	s.logger.Debug("Encrypting MEK with recovery code")

	// Generate a key from the recovery code using PBKDF2 (similar to password-based encryption)
	recoveryKey, err := s.encryptionService.GenerateKeyFromPassword(recoveryCode, salt)
	if err != nil {
		s.logger.Error("Failed to generate key from recovery code", "error", err)
		return "", err
	}

	// Encrypt the MEK using the recovery-derived key
	encryptedMek, err = s.encryptionService.Encrypt(plainMek, recoveryKey)
	if err != nil {
		s.logger.Error("Failed to encrypt MEK with recovery code", "error", err)
		return "", err
	}

	s.logger.Debug("MEK encrypted with recovery code successfully")
	return encryptedMek, nil
}
