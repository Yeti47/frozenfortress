package auth

import (
	"errors"
	"regexp"
	"time"

	"fmt"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
)

type DefaultUserManager struct {
	userRepository    UserRepository
	userIdGenerator   UserIdGenerator
	encryptionService encryption.EncryptionService
	securityService   SecurityService
}

func NewDefaultUserManager(userRepository UserRepository, userIdGenerator UserIdGenerator, encryptionService encryption.EncryptionService, securityService SecurityService) *DefaultUserManager {
	return &DefaultUserManager{
		userRepository:    userRepository,
		userIdGenerator:   userIdGenerator,
		encryptionService: encryptionService,
		securityService:   securityService,
	}
}

func (manager *DefaultUserManager) CreateUser(request CreateUserRequest) (CreateUserResponse, error) {

	// Validate the request
	if request.UserName == "" {
		return CreateUserResponse{}, errors.New("username cannot be empty")
	}
	if request.Password == "" {
		return CreateUserResponse{}, errors.New("password cannot be empty")
	}

	if !manager.IsValidUsername(request.UserName) {
		return CreateUserResponse{}, errors.New("invalid username")
	}

	isValidPw, err := manager.IsValidPassword(request.Password)
	if err != nil {
		return CreateUserResponse{}, err
	}
	if !isValidPw {
		return CreateUserResponse{}, errors.New("invalid password")
	}

	userId := manager.userIdGenerator.GenerateUserId()

	pwHash, pwSalt, err := manager.encryptionService.Hash(request.Password)
	if err != nil {
		return CreateUserResponse{}, err
	}

	encryptedEncryptionKey, encryptionSalt, err := manager.securityService.GenerateEncryptedMek(request.Password)
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
		Id:         user.Id,
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
		Id:         user.Id,
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
			Id:         user.Id,
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

	locked, err := manager.securityService.LockUser(*user)

	return locked, err
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

	unlocked, err := manager.securityService.UnlockUser(*user)

	return unlocked, err
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
	isValid, err := manager.securityService.VerifyUserPassword(*user, request.OldPassword)
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
	plainEncryptionKey, err := manager.securityService.UncoverMek(*user, request.OldPassword)
	if err != nil {
		return false, err
	}

	// Re-encrypt the encryption key with the new password-derived key
	encryptedEncryptionKey, encryptionSalt, err := manager.securityService.EncryptMek(plainEncryptionKey, request.NewPassword)
	if err != nil {
		return false, err
	}

	user.EncryptionKey = encryptedEncryptionKey
	user.EncryptionSalt = encryptionSalt

	success, err := manager.userRepository.Update(user)
	return success, err
}

// IsValidUsername checks if the username is valid
func (manager *DefaultUserManager) IsValidUsername(userName string) bool {

	// Check if the username is empty
	if userName == "" {
		return false
	}

	// Check if the username matches the required pattern
	re := regexp.MustCompile(`^[a-zA-Z0-9_]{3,20}$`)
	return re.MatchString(userName)
}

// IsValidPassword checks if the password is valid
func (manager *DefaultUserManager) IsValidPassword(password string) (bool, error) {

	// Check if the password is empty
	if password == "" {
		return false, errors.New("password cannot be empty")
	}

	const minPasswordLength = 16

	if len(password) < minPasswordLength {
		return false, errors.New("password must be at least " + fmt.Sprint(minPasswordLength) + " characters long")
	}

	// Check if the password matches the required pattern
	re := regexp.MustCompile(`^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[@$!%*?&])[A-Za-z\d@$!%*?&]{` + fmt.Sprint(minPasswordLength) + `,}$`)

	if !re.MatchString(password) {
		return false, errors.New("password must contain at least one uppercase letter, one lowercase letter, one number, and one special character")
	}

	return true, nil
}
