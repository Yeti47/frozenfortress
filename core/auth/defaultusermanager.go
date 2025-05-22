package auth

import (
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
)

type DefaultUserManager struct {
	userRepository    UserRepository
	userIdGenerator   UserIdGenerator
	encryptionService encryption.EncryptionService
}

func NewDefaultUserManager(userRepository UserRepository, userIdGenerator UserIdGenerator, encryptionService encryption.EncryptionService) *DefaultUserManager {
	return &DefaultUserManager{
		userRepository:    userRepository,
		userIdGenerator:   userIdGenerator,
		encryptionService: encryptionService,
	}
}

// generateAndEncryptUserKey generates a new encryption key and encrypts it with a key derived from the given password.
// Returns the encrypted key, the plain key, the encryption salt, and any error encountered.
func (manager *DefaultUserManager) generateAndEncryptUserKey(password string) (encryptedEncryptionKey string, plainEncryptionKey string, encryptionSalt string, err error) {

	plainEncryptionKey, err = manager.encryptionService.GenerateKey()
	if err != nil {
		return
	}

	var passwordDerivedKey string
	passwordDerivedKey, encryptionSalt, err = manager.encryptionService.GenerateKeyFromPassword(password)
	if err != nil {
		return
	}

	encryptedEncryptionKey, err = manager.encryptionService.Encrypt(plainEncryptionKey, passwordDerivedKey)
	if err != nil {
		return
	}

	return
}

// encryptUserKeyWithPassword encrypts the given plain encryption key with a key derived from the given password.
// Returns the encrypted key, the encryption salt, and any error encountered.
func (manager *DefaultUserManager) encryptUserKeyWithPassword(plainEncryptionKey string, password string) (encryptedEncryptionKey string, encryptionSalt string, err error) {

	var passwordDerivedKey string
	passwordDerivedKey, encryptionSalt, err = manager.encryptionService.GenerateKeyFromPassword(password)
	if err != nil {
		return
	}

	encryptedEncryptionKey, err = manager.encryptionService.Encrypt(plainEncryptionKey, passwordDerivedKey)
	if err != nil {
		return
	}

	return
}

func (manager *DefaultUserManager) CreateUser(request CreateUserRequest) (CreateUserResponse, error) {
	userId := manager.userIdGenerator.GenerateUserId()

	pwHash, pwSalt, err := manager.encryptionService.Hash(request.Password)
	if err != nil {
		return CreateUserResponse{}, err
	}

	encryptedEncryptionKey, _, encryptionSalt, err := manager.generateAndEncryptUserKey(request.Password)
	if err != nil {
		return CreateUserResponse{}, err
	}

	user := &User{
		Id:             userId,
		UserName:       request.UserName,
		PasswordHash:   pwHash,
		PasswordSalt:   pwSalt,
		EncryptionKey:  encryptedEncryptionKey,
		EncryptionSalt: encryptionSalt,
		IsActive:       false, // Set to false for new users, can be activated by admin
		IsLocked:       false,
		CreatedAt:      time.Now(),
		ModifiedAt:     time.Now(),
	}

	success, err := manager.userRepository.Add(user)
	if err != nil || !success {
		return CreateUserResponse{}, err
	}

	return CreateUserResponse{UserId: userId}, nil
}

func (manager *DefaultUserManager) GetUserById(userId string) (UserDto, error) {

	user, err := manager.userRepository.FindById(userId)

	if err != nil {
		return UserDto{}, err
	}

	if user == nil {
		return UserDto{}, nil
	}

	return UserDto{
		UserId:     user.Id,
		UserName:   user.UserName,
		IsActive:   user.IsActive,
		IsLocked:   user.IsLocked,
		CreatedAt:  user.CreatedAt.Format(time.RFC3339),
		ModifiedAt: user.ModifiedAt.Format(time.RFC3339),
	}, nil
}

func (manager *DefaultUserManager) GetUserByUserName(userName string) (UserDto, error) {

	user, err := manager.userRepository.FindByUserName(userName)

	if err != nil {
		return UserDto{}, err
	}

	if user == nil {
		return UserDto{}, nil
	}

	return UserDto{
		UserId:     user.Id,
		UserName:   user.UserName,
		IsActive:   user.IsActive,
		IsLocked:   user.IsLocked,
		CreatedAt:  user.CreatedAt.Format(time.RFC3339),
		ModifiedAt: user.ModifiedAt.Format(time.RFC3339),
	}, nil
}

func (manager *DefaultUserManager) GetAllUsers() ([]UserDto, error) {

	users := manager.userRepository.GetAll()

	if users == nil {
		return nil, nil
	}

	userDtos := make([]UserDto, len(users))

	for i, user := range users {
		userDtos[i] = UserDto{
			UserId:     user.Id,
			UserName:   user.UserName,
			IsActive:   user.IsActive,
			IsLocked:   user.IsLocked,
			CreatedAt:  user.CreatedAt.Format(time.RFC3339),
			ModifiedAt: user.ModifiedAt.Format(time.RFC3339),
		}
	}

	return userDtos, nil
}

// ActivateUser activates a user by their ID
func (manager *DefaultUserManager) ActivateUser(id string) (bool, error) {
	user, err := manager.userRepository.FindById(id)

	if err != nil {
		return false, err
	}

	if user == nil {
		return false, nil
	}

	user.IsActive = true
	success, err := manager.userRepository.Update(user)

	return success, err
}

// DeactivateUser deactivates a user by their ID
func (manager *DefaultUserManager) DeactivateUser(id string) (bool, error) {

	user, err := manager.userRepository.FindById(id)

	if err != nil {
		return false, err
	}

	if user == nil {
		return false, nil
	}

	user.IsActive = false
	success, err := manager.userRepository.Update(user)

	return success, err
}

// LockUser locks a user by their ID
func (manager *DefaultUserManager) LockUser(id string) (bool, error) {
	user, err := manager.userRepository.FindById(id)

	if err != nil {
		return false, err
	}

	if user == nil {
		return false, nil
	}

	user.IsLocked = true
	success, err := manager.userRepository.Update(user)

	return success, err
}

// UnlockUser unlocks a user by their ID
func (manager *DefaultUserManager) UnlockUser(id string) (bool, error) {
	user, err := manager.userRepository.FindById(id)

	if err != nil {
		return false, err
	}

	if user == nil {
		return false, nil
	}

	user.IsLocked = false
	success, err := manager.userRepository.Update(user)

	return success, err
}

// ChangePassword changes the password for a user
func (manager *DefaultUserManager) ChangePassword(request ChangePasswordRequest) (bool, error) {
	user, err := manager.userRepository.FindById(request.UserId)
	if err != nil {
		return false, err
	}

	if user == nil {
		return false, nil
	}

	// Verify the old password
	isValid, err := manager.encryptionService.VerifyHash(request.OldPassword, user.PasswordHash, user.PasswordSalt)
	if err != nil {
		return false, err
	}
	if !isValid {
		return false, nil // Old password is incorrect. Don't specify the reason to avoid abuse.
	}

	pwHash, pwSalt, err := manager.encryptionService.Hash(request.NewPassword)
	if err != nil {
		return false, err
	}

	user.PasswordHash = pwHash
	user.PasswordSalt = pwSalt
	user.ModifiedAt = time.Now()

	// Decrypt the current encryption key using the old password
	oldPasswordDerivedKey, _, err := manager.encryptionService.GenerateKeyFromPassword(request.OldPassword)
	if err != nil {
		return false, err
	}
	plainEncryptionKey, err := manager.encryptionService.Decrypt(user.EncryptionKey, oldPasswordDerivedKey)
	if err != nil {
		return false, err
	}

	// Re-encrypt the encryption key with the new password-derived key
	encryptedEncryptionKey, encryptionSalt, err := manager.encryptUserKeyWithPassword(plainEncryptionKey, request.NewPassword)
	if err != nil {
		return false, err
	}

	user.EncryptionKey = encryptedEncryptionKey
	user.EncryptionSalt = encryptionSalt

	success, err := manager.userRepository.Update(user)
	return success, err
}

// VerifyUserPassword checks if the given password is valid for the user with the given ID.
func (manager *DefaultUserManager) VerifyUserPassword(userId string, password string) (bool, error) {
	user, err := manager.userRepository.FindById(userId)
	if err != nil {
		return false, err
	}

	if user == nil {
		return false, nil // User not found
	}

	return manager.encryptionService.VerifyHash(password, user.PasswordHash, user.PasswordSalt)
}
