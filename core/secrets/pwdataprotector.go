package secrets

import (
	"errors"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
)

// PasswordDataProtector is a struct that provides methods to encrypt and decrypt data using the a user's password.

type PasswordDataProtector struct {
	encryptionService encryption.EncryptionService
	securityService   auth.SecurityService
	userRepo          auth.UserRepository
	userId            string // User ID for which the password is used
	password          string
	user              *auth.User // Cached user to avoid multiple lookups
}

// CreatePasswordDataProtector creates a new PasswordDataProtector instance.
// It is intended to be used for encrypting and decrypting data with a user's password.
func CreatePasswordDataProtector(encryptionService encryption.EncryptionService, securityService auth.SecurityService, userRepo auth.UserRepository, userId, password string) *PasswordDataProtector {
	return &PasswordDataProtector{
		encryptionService: encryptionService,
		securityService:   securityService,
		userRepo:          userRepo,
		userId:            userId,
		password:          password,
	}
}

// Protect encrypts the given piece of data using the user's password.
func (p *PasswordDataProtector) Protect(data string) (protectedData string, err error) {

	user, err := p.getUser()
	if err != nil {
		return "", err
	}

	plainMek, err := p.securityService.UncoverMek(*user, p.password)
	if err != nil {
		return "", errors.New(("MEK not available: " + err.Error()))
	}

	protectedData, err = p.encryptionService.Encrypt(data, plainMek)
	if err != nil {
		return "", errors.New(("Encryption failed: " + err.Error()))
	}

	return protectedData, nil
}

// Unprotect decrypts the given piece of data using the user's password.
func (p *PasswordDataProtector) Unprotect(protectedData string) (data string, err error) {

	user, err := p.getUser()
	if err != nil {
		return "", err
	}

	plainMek, err := p.securityService.UncoverMek(*user, p.password)
	if err != nil {
		return "", errors.New(("MEK not available: " + err.Error()))
	}

	data, err = p.encryptionService.Decrypt(protectedData, plainMek)
	if err != nil {
		return "", errors.New(("Decryption failed: " + err.Error()))
	}

	return data, nil
}

// getUser returns the user associated with the PasswordDataProtector and caches it for future use.
func (p *PasswordDataProtector) getUser() (*auth.User, error) {
	if p.user == nil {
		user, err := p.userRepo.FindById(p.userId)
		if err != nil {
			return nil, errors.New("user not found: " + err.Error())
		}
		p.user = user
	}
	return p.user, nil
}
