package auth

import (
	"regexp"
	"time"

	"fmt"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
)

const (
	// specialCharsRegexFragment defines the regex pattern fragment for allowed special characters.
	// Characters: @ $ ! % * ? & # _ - . , ; : + § / [ ] ( ) { } =
	specialCharsRegexFragment = `@$!%*?&#_.,;:+§/\\[\\](){}=-`

	// displaySpecialChars is a user-friendly string of allowed special characters for error messages.
	displaySpecialChars = "@ $ ! % * ? & # _ - . , ; : + § / [ ] ( ) { } ="
)

var (
	// Pre-compiled regexps for password validation efficiency.
	hasLowerRegexp        = regexp.MustCompile(`[a-z]`)
	hasUpperRegexp        = regexp.MustCompile(`[A-Z]`)
	hasDigitRegexp        = regexp.MustCompile(`\\d`)
	hasSpecialRegexp      = regexp.MustCompile(`[` + specialCharsRegexFragment + `]`)
	allAllowedCharsRegexp = regexp.MustCompile(`^[A-Za-z0-9` + specialCharsRegexFragment + `]+$`)
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
		return CreateUserResponse{}, ccc.NewInvalidInputError("username", "cannot be empty")
	}
	if request.Password == "" {
		return CreateUserResponse{}, ccc.NewInvalidInputError("password", "cannot be empty")
	}

	if !manager.IsValidUsername(request.UserName) {
		return CreateUserResponse{}, ccc.NewInvalidInputError("username", "invalid username")
	}

	isValidPw, err := manager.IsValidPassword(request.Password)
	if err != nil {
		return CreateUserResponse{}, err
	}
	if !isValidPw {
		return CreateUserResponse{}, ccc.NewInvalidInputError("password", "invalid password")
	}

	userId := manager.userIdGenerator.GenerateUserId()

	pwHash, pwSalt, err := manager.encryptionService.Hash(request.Password)
	if err != nil {
		return CreateUserResponse{}, ccc.NewInternalError("hash password", err)
	}

	mek, pdkSalt, err := manager.securityService.GenerateEncryptedMek(request.Password)
	if err != nil {
		return CreateUserResponse{}, ccc.NewInternalError("generate MEK", err)
	}

	user := &User{
		Id:           userId,
		UserName:     request.UserName,
		PasswordHash: pwHash,
		PasswordSalt: pwSalt,
		Mek:          mek,
		PdkSalt:      pdkSalt,
		IsActive:     false, // Set to false for new users, can be activated by admin
		IsLocked:     false,
		CreatedAt:    time.Now(),
		ModifiedAt:   time.Now(),
	}

	success, err := manager.userRepository.Add(user)
	if err != nil || !success {
		return CreateUserResponse{}, ccc.NewDatabaseError("add user", err)
	}

	return CreateUserResponse{UserId: userId}, nil
}

func (manager *DefaultUserManager) GetUserById(userId string) (UserDto, error) {
	if userId == "" {
		return UserDto{}, ccc.NewInvalidInputError("user ID", "cannot be empty")
	}

	user, err := manager.userRepository.FindById(userId)
	if err != nil {
		return UserDto{}, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		return UserDto{}, ccc.NewResourceNotFoundError(userId, "User")
	}

	userDto := UserDto{
		Id:         user.Id,
		UserName:   user.UserName,
		IsActive:   user.IsActive,
		IsLocked:   user.IsLocked,
		CreatedAt:  user.CreatedAt.Format(time.RFC3339),
		ModifiedAt: user.ModifiedAt.Format(time.RFC3339),
	}

	return userDto, nil
}

func (manager *DefaultUserManager) GetUserByUserName(userName string) (UserDto, error) {
	if userName == "" {
		return UserDto{}, ccc.NewInvalidInputError("username", "cannot be empty")
	}

	user, err := manager.userRepository.FindByUserName(userName)
	if err != nil {
		return UserDto{}, ccc.NewDatabaseError("find user by username", err)
	}

	if user == nil {
		return UserDto{}, ccc.NewResourceNotFoundError(userName, "User")
	}

	userDto := UserDto{
		Id:         user.Id,
		UserName:   user.UserName,
		IsActive:   user.IsActive,
		IsLocked:   user.IsLocked,
		CreatedAt:  user.CreatedAt.Format(time.RFC3339),
		ModifiedAt: user.ModifiedAt.Format(time.RFC3339),
	}

	return userDto, nil
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
	if id == "" {
		return false, ccc.NewInvalidInputError("user ID", "cannot be empty")
	}

	user, err := manager.userRepository.FindById(id)
	if err != nil {
		return false, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		return false, ccc.NewResourceNotFoundError(id, "User")
	}

	user.IsActive = true
	success, err := manager.userRepository.Update(user)
	if err != nil {
		return false, ccc.NewDatabaseError("update user", err)
	}

	return success, nil
}

// DeactivateUser deactivates a user by their ID
func (manager *DefaultUserManager) DeactivateUser(id string) (bool, error) {
	if id == "" {
		return false, ccc.NewInvalidInputError("user ID", "cannot be empty")
	}

	user, err := manager.userRepository.FindById(id)
	if err != nil {
		return false, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		return false, ccc.NewResourceNotFoundError(id, "User")
	}

	user.IsActive = false
	success, err := manager.userRepository.Update(user)
	if err != nil {
		return false, ccc.NewDatabaseError("update user", err)
	}

	return success, nil
}

// LockUser locks a user by their ID
func (manager *DefaultUserManager) LockUser(id string) (bool, error) {
	if id == "" {
		return false, ccc.NewInvalidInputError("user ID", "cannot be empty")
	}

	user, err := manager.userRepository.FindById(id)
	if err != nil {
		return false, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		return false, ccc.NewResourceNotFoundError(id, "User")
	}

	locked, err := manager.securityService.LockUser(*user)
	if err != nil {
		return false, ccc.NewDatabaseError("lock user", err)
	}

	return locked, nil
}

// UnlockUser unlocks a user by their ID
func (manager *DefaultUserManager) UnlockUser(id string) (bool, error) {
	if id == "" {
		return false, ccc.NewInvalidInputError("user ID", "cannot be empty")
	}

	user, err := manager.userRepository.FindById(id)
	if err != nil {
		return false, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		return false, ccc.NewResourceNotFoundError(id, "User")
	}

	unlocked, err := manager.securityService.UnlockUser(*user)
	if err != nil {
		return false, ccc.NewDatabaseError("unlock user", err)
	}

	return unlocked, nil
}

// ChangePassword changes the password for a user
func (manager *DefaultUserManager) ChangePassword(request ChangePasswordRequest) (bool, error) {
	user, err := manager.userRepository.FindById(request.UserId)
	if err != nil {
		return false, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		return false, ccc.NewResourceNotFoundError(request.UserId, "User")
	}

	// Verify the old password
	isValid, err := manager.securityService.VerifyUserPassword(*user, request.OldPassword)
	if err != nil {
		return false, ccc.NewInternalError("verify user password", err)
	}
	if !isValid {
		return false, ccc.NewUnauthorizedError("operation not authorized")
	}

	pwHash, pwSalt, err := manager.encryptionService.Hash(request.NewPassword)
	if err != nil {
		return false, ccc.NewInternalError("hash password", err)
	}

	user.PasswordHash = pwHash
	user.PasswordSalt = pwSalt
	user.ModifiedAt = time.Now()

	// Decrypt the current encryption key using the old password
	plainMek, err := manager.securityService.UncoverMek(*user, request.OldPassword)
	if err != nil {
		return false, ccc.NewInternalError("uncover MEK", err)
	}

	// Re-encrypt the encryption key with the new password-derived key
	mek, pdkSalt, err := manager.securityService.EncryptMek(plainMek, request.NewPassword)
	if err != nil {
		return false, ccc.NewInternalError("encrypt MEK", err)
	}

	user.Mek = mek
	user.PdkSalt = pdkSalt

	success, err := manager.userRepository.Update(user)
	return success, ccc.NewDatabaseError("update user", err)
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
		return false, ccc.NewInvalidInputError("password", "cannot be empty")
	}

	const minPasswordLength = 16

	// 1. Check length
	if len(password) < minPasswordLength {
		return false, ccc.NewInvalidInputError("password", "must be at least "+fmt.Sprint(minPasswordLength)+" characters long")
	}

	// 2. Check inclusion of lower case letters
	if !hasLowerRegexp.MatchString(password) {
		return false, ccc.NewInvalidInputError("password", "must contain at least one lowercase letter")
	}

	// 3. Check inclusion of upper case letters
	if !hasUpperRegexp.MatchString(password) {
		return false, ccc.NewInvalidInputError("password", "must contain at least one uppercase letter")
	}

	// 4. Check inclusion of numbers
	if !hasDigitRegexp.MatchString(password) {
		return false, ccc.NewInvalidInputError("password", "must contain at least one number")
	}

	// 5. Check inclusion of allowed special characters
	if !hasSpecialRegexp.MatchString(password) {
		return false, ccc.NewInvalidInputError("password", "must contain at least one special character. Allowed special characters are: "+displaySpecialChars)
	}

	// 6. Check that all characters in the password are from the allowed set
	// (alphanumeric or one of the defined special characters)
	if !allAllowedCharsRegexp.MatchString(password) {
		return false, ccc.NewInvalidInputError("password", "contains invalid characters. Only letters, numbers, and the following special characters are allowed: "+displaySpecialChars)
	}

	return true, nil
}

// DeleteUser deletes a user by their ID
func (manager *DefaultUserManager) DeleteUser(id string) (bool, error) {

	if id == "" {
		return false, ccc.NewInvalidInputError("user ID", "cannot be empty")
	}

	success, err := manager.userRepository.Remove(id)
	if err != nil {
		return false, ccc.NewDatabaseError("remove user", err)
	}

	// Idempotency: if the user was not found, we still return success; no error is raised
	return success, nil
}

// VerifyPassword checks if the provided password matches the user's password
func (manager *DefaultUserManager) VerifyPassword(userId string, password string) (bool, error) {
	user, err := manager.userRepository.FindById(userId)
	if err != nil {
		return false, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		return false, ccc.NewResourceNotFoundError(userId, "User")
	}

	success, err := manager.securityService.VerifyUserPassword(*user, password)
	if err != nil {
		return false, ccc.NewInternalError("verify user password", err)
	}

	return success, nil
}
