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
