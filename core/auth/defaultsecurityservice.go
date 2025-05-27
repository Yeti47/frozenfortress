package auth

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
)

type DefaultSecurityService struct {
	userRepository    UserRepository
	encryptionService encryption.EncryptionService
}

func NewDefaultSecurityService(userRepository UserRepository, encryptionService encryption.EncryptionService) *DefaultSecurityService {
	return &DefaultSecurityService{
		userRepository:    userRepository,
		encryptionService: encryptionService,
	}
}

// LockUser locks the user account.
func (s *DefaultSecurityService) LockUser(user User) (bool, error) {

	user.IsLocked = true

	// TODO: Add logic to log the lock action

	// Update the user in the repository
	updated, err := s.userRepository.Update(&user)
	if err != nil {
		return false, err
	}

	return updated, nil
}

// UnlockUser unlocks the user account.
func (s *DefaultSecurityService) UnlockUser(user User) (bool, error) {

	user.IsLocked = false

	// TODO: Add logic to log the unlock action

	// Update the user in the repository
	updated, err := s.userRepository.Update(&user)
	if err != nil {
		return false, err
	}

	return updated, nil
}

// VerifyUserPassword verifies the user's password.
func (s *DefaultSecurityService) VerifyUserPassword(user User, password string) (bool, error) {

	return s.encryptionService.VerifyHash(password, user.PasswordHash, user.PasswordSalt)
}

// UncoverMek reads the user's MEK (Master Encryption Key) from the database.
func (s *DefaultSecurityService) UncoverMek(user User, password string) (string, error) {

	// Verify the user's password
	isValid, err := s.VerifyUserPassword(user, password)
	if err != nil {
		return "", err
	}
	if !isValid {
		return "", nil
	}

	// Restore the PDK (Password-Derived Key) via the given password and stored salt
	pdk, err := s.encryptionService.GenerateKeyFromPassword(password, user.PdkSalt)
	if err != nil {
		return "", err
	}

	// Decrypt the MEK (Master Encryption Key) using the PDK
	mek, err := s.encryptionService.Decrypt(user.Mek, pdk)
	if err != nil {
		return "", err
	}

	return mek, nil
}

// EncryptMek encrypts the user's MEK (Master Encryption Key) using the provided password.
func (s *DefaultSecurityService) EncryptMek(plainMek string, password string) (ecnryptedMek string, salt string, err error) {

	// Generate a random salt
	_, salt, err = s.encryptionService.GenerateSalt()
	if err != nil {
		return "", "", err
	}

	// Generate the PDK (Password-Derived Key) using the password and salt
	pdk, err := s.encryptionService.GenerateKeyFromPassword(password, salt)
	if err != nil {
		return "", "", err
	}

	// Encrypt the MEK using the PDK
	ecnryptedMek, err = s.encryptionService.Encrypt(plainMek, pdk)
	if err != nil {
		return "", "", err
	}

	return ecnryptedMek, salt, nil
}

// GenerateEncryptedMek generates an encrypted MEK using the user's password.
func (s *DefaultSecurityService) GenerateEncryptedMek(password string) (encryptedMek string, salt string, err error) {

	// Generate a random salt
	_, salt, err = s.encryptionService.GenerateSalt()
	if err != nil {
		return "", "", err
	}

	// Generate the PDK (Password-Derived Key) using the password and salt
	pdk, err := s.encryptionService.GenerateKeyFromPassword(password, salt)
	if err != nil {
		return "", "", err
	}

	// Generate a random MEK (Master Encryption Key)
	mek, err := s.encryptionService.GenerateKey()
	if err != nil {
		return "", "", err
	}

	// Encrypt the MEK using the PDK
	encryptedMek, err = s.encryptionService.Encrypt(mek, pdk)
	if err != nil {
		return "", "", err
	}

	return encryptedMek, salt, nil
}
