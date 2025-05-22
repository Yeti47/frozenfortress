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

func (manager *DefaultUserManager) CreateUser(request CreateUserRequest) (CreateUserResponse, error) {

	userId := manager.userIdGenerator.GenerateUserId()

	pwHash, pwSalt, err := manager.encryptionService.Hash(request.Password)

	if err != nil {
		return CreateUserResponse{}, err
	}

	plainEncryptionKey, err := manager.encryptionService.GenerateKey()

	if err != nil {
		return CreateUserResponse{}, err
	}

	// Encrypt the plain encryption key with the user's password
	// This is the key that will be used to encrypt/decrypt the user's secrets
	encryptedEncryptionKey, err := manager.encryptionService.Encrypt(plainEncryptionKey, request.Password)

	if err != nil {
		return CreateUserResponse{}, err
	}

	user := &User{
		Id:            userId,
		UserName:      request.UserName,
		PasswordHash:  pwHash,
		PasswordSalt:  pwSalt,
		EncryptionKey: encryptedEncryptionKey,
		IsActive:      false, // Set to false for new users, can be activated by admin
		IsLocked:      false,
		CreatedAt:     time.Now(),
		ModifiedAt:    time.Now(),
	}

	success, err := manager.userRepository.Add(user)

	if err != nil || !success {
		return CreateUserResponse{}, err
	}

	return CreateUserResponse{UserId: userId}, nil
}
