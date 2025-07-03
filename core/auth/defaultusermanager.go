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
	specialCharsRegexFragment = `@$!%*?&#_.,;:+§/\[\(){}=-`

	// displaySpecialChars is a user-friendly string of allowed special characters for error messages.
	displaySpecialChars = "@ $ ! % * ? & # _ - . , ; : + § / [ ] ( ) { } ="
)

var (
	// Pre-compiled regexps for password validation efficiency.
	hasLowerRegexp        = regexp.MustCompile(`[a-z]`)
	hasUpperRegexp        = regexp.MustCompile(`[A-Z]`)
	hasDigitRegexp        = regexp.MustCompile(`[0-9]`)
	hasSpecialRegexp      = regexp.MustCompile(`[` + specialCharsRegexFragment + `]`)
	allAllowedCharsRegexp = regexp.MustCompile(`^[A-Za-z0-9` + specialCharsRegexFragment + `]+$`)
)

type DefaultUserManager struct {
	userRepository    UserRepository
	userIdGenerator   UserIdGenerator
	encryptionService encryption.EncryptionService
	securityService   SecurityService
	logger            ccc.Logger
}

func NewDefaultUserManager(userRepository UserRepository, userIdGenerator UserIdGenerator, encryptionService encryption.EncryptionService, securityService SecurityService, logger ccc.Logger) *DefaultUserManager {
	if logger == nil {
		logger = ccc.NopLogger
	}

	return &DefaultUserManager{
		userRepository:    userRepository,
		userIdGenerator:   userIdGenerator,
		encryptionService: encryptionService,
		securityService:   securityService,
		logger:            logger,
	}
}

func (manager *DefaultUserManager) CreateUser(request CreateUserRequest) (CreateUserResponse, error) {
	manager.logger.Info("Creating new user", "username", request.UserName)

	// Validate the request
	if request.UserName == "" {
		manager.logger.Warn("User creation failed: empty username")
		return CreateUserResponse{}, ccc.NewInvalidInputErrorWithMessage(
			"username",
			"cannot be empty",
			"Username cannot be empty. Please provide a valid username.",
		)
	}
	if request.Password == "" {
		manager.logger.Warn("User creation failed: empty password", "username", request.UserName)
		return CreateUserResponse{}, ccc.NewInvalidInputErrorWithMessage(
			"password",
			"cannot be empty",
			"Password cannot be empty. Please provide a valid password.",
		)
	}

	if !manager.IsValidUsername(request.UserName) {
		manager.logger.Warn("User creation failed: invalid username", "username", request.UserName)
		return CreateUserResponse{}, ccc.NewInvalidInputErrorWithMessage(
			"username",
			"username invalid",
			"Username must be 3-20 characters long and can only contain letters, numbers, and underscores.",
		)
	}

	// Check if username already exists
	existingUser, err := manager.userRepository.FindByUserName(request.UserName)
	if err != nil {
		manager.logger.Error("Failed to check if username exists", "username", request.UserName, "error", err)
		return CreateUserResponse{}, ccc.NewDatabaseError("check username existence", err)
	}
	if existingUser != nil {
		manager.logger.Warn("User creation failed: username already exists", "username", request.UserName)
		return CreateUserResponse{}, ccc.NewInvalidInputErrorWithMessage(
			"username",
			"username invalid",
			"Username must be 3-20 characters long and can only contain letters, numbers, and underscores.",
		)
	}

	isValidPw, err := manager.IsValidPassword(request.Password)
	if err != nil {
		manager.logger.Error("Password validation failed", "username", request.UserName, "error", err)
		return CreateUserResponse{}, err
	}
	if !isValidPw {
		manager.logger.Warn("User creation failed: invalid password", "username", request.UserName)
		return CreateUserResponse{}, ccc.NewInvalidInputError("password", "invalid password")
	}

	userId := manager.userIdGenerator.GenerateId()
	manager.logger.Debug("Generated user ID", "user_id", userId, "username", request.UserName)

	pwHash, pwSalt, err := manager.encryptionService.Hash(request.Password)
	if err != nil {
		manager.logger.Error("Failed to hash password", "user_id", userId, "username", request.UserName, "error", err)
		return CreateUserResponse{}, ccc.NewInternalError("hash password", err)
	}

	mek, pdkSalt, err := manager.securityService.GenerateEncryptedMek(request.Password)
	if err != nil {
		manager.logger.Error("Failed to generate MEK", "user_id", userId, "username", request.UserName, "error", err)
		return CreateUserResponse{}, ccc.NewInternalError("generate MEK", err)
	}

	manager.logger.Debug("User credentials and encryption keys generated successfully", "user_id", userId, "username", request.UserName)

	// Generate recovery code
	recoveryCode, recoveryHash, recoverySalt, err := manager.securityService.GenerateRecoveryCode()
	if err != nil {
		manager.logger.Error("Failed to generate recovery code", "user_id", userId, "username", request.UserName, "error", err)
		return CreateUserResponse{}, ccc.NewInternalError("generate recovery code", err)
	}

	user := &User{
		Id:                userId,
		UserName:          request.UserName,
		PasswordHash:      pwHash,
		PasswordSalt:      pwSalt,
		Mek:               mek,
		PdkSalt:           pdkSalt,
		IsActive:          false, // Set to false for new users, can be activated by admin
		IsLocked:          false,
		RecoveryCodeHash:  recoveryHash,
		RecoveryCodeSalt:  recoverySalt,
		RecoveryGenerated: time.Now(),
		CreatedAt:         time.Now(),
		ModifiedAt:        time.Now(),
	}

	// Get the plain MEK to encrypt with recovery code
	plainMek, err := manager.securityService.UncoverMek(*user, request.Password)
	if err != nil {
		manager.logger.Error("Failed to uncover MEK for recovery encryption", "user_id", userId, "username", request.UserName, "error", err)
		return CreateUserResponse{}, ccc.NewInternalError("uncover MEK for recovery", err)
	}

	// Encrypt MEK with recovery code
	recoveryMek, err := manager.securityService.EncryptMekWithRecoveryCode(plainMek, recoveryCode, recoverySalt)
	if err != nil {
		manager.logger.Error("Failed to encrypt MEK with recovery code", "user_id", userId, "username", request.UserName, "error", err)
		return CreateUserResponse{}, ccc.NewInternalError("encrypt MEK with recovery code", err)
	}

	manager.logger.Debug("Recovery code and MEK encryption completed successfully", "user_id", userId, "username", request.UserName)

	user.RecoveryMek = recoveryMek

	success, err := manager.userRepository.Add(user)
	if err != nil || !success {
		manager.logger.Error("Failed to add user to repository", "user_id", userId, "username", request.UserName, "success", success, "error", err)
		return CreateUserResponse{}, ccc.NewDatabaseError("add user", err)
	}

	manager.logger.Info("User created successfully", "user_id", userId, "username", request.UserName)
	return CreateUserResponse{
		UserId:       userId,
		RecoveryCode: recoveryCode,
	}, nil
}

func (manager *DefaultUserManager) GetUserById(userId string) (UserDto, error) {
	manager.logger.Debug("Retrieving user by ID", "user_id", userId)

	if userId == "" {
		manager.logger.Warn("Get user by ID failed: empty user ID")
		return UserDto{}, ccc.NewInvalidInputError("user ID", "cannot be empty")
	}

	user, err := manager.userRepository.FindById(userId)
	if err != nil {
		manager.logger.Error("Failed to find user by ID", "user_id", userId, "error", err)
		return UserDto{}, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		manager.logger.Debug("User not found by ID", "user_id", userId)
		return UserDto{}, ccc.NewResourceNotFoundError(userId, "User")
	}

	manager.logger.Debug("User retrieved successfully by ID", "user_id", userId, "username", user.UserName)

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
	manager.logger.Debug("Retrieving user by username", "username", userName)

	if userName == "" {
		manager.logger.Warn("Get user by username failed: empty username")
		return UserDto{}, ccc.NewInvalidInputError("username", "cannot be empty")
	}

	user, err := manager.userRepository.FindByUserName(userName)
	if err != nil {
		manager.logger.Error("Failed to find user by username", "username", userName, "error", err)
		return UserDto{}, ccc.NewDatabaseError("find user by username", err)
	}

	if user == nil {
		manager.logger.Debug("User not found by username", "username", userName)
		return UserDto{}, ccc.NewResourceNotFoundError(userName, "User")
	}

	manager.logger.Debug("User retrieved successfully by username", "user_id", user.Id, "username", userName)

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
	manager.logger.Debug("Retrieving all users")

	users := manager.userRepository.GetAll()

	if users == nil {
		manager.logger.Debug("No users found in repository")
		return nil, nil
	}

	manager.logger.Debug("Retrieved users from repository", "user_count", len(users))

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
	manager.logger.Info("Activating user", "user_id", id)

	if id == "" {
		manager.logger.Warn("User activation failed: empty user ID")
		return false, ccc.NewInvalidInputError("user ID", "cannot be empty")
	}

	user, err := manager.userRepository.FindById(id)
	if err != nil {
		manager.logger.Error("Failed to find user for activation", "user_id", id, "error", err)
		return false, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		manager.logger.Warn("User not found for activation", "user_id", id)
		return false, ccc.NewResourceNotFoundError(id, "User")
	}

	user.IsActive = true
	success, err := manager.userRepository.Update(user)
	if err != nil {
		manager.logger.Error("Failed to activate user", "user_id", id, "username", user.UserName, "error", err)
		return false, ccc.NewDatabaseError("update user", err)
	}

	if success {
		manager.logger.Info("User activated successfully", "user_id", id, "username", user.UserName)
	} else {
		manager.logger.Warn("User activation operation returned false", "user_id", id, "username", user.UserName)
	}

	return success, nil
}

// DeactivateUser deactivates a user by their ID
func (manager *DefaultUserManager) DeactivateUser(id string) (bool, error) {
	manager.logger.Info("Deactivating user", "user_id", id)

	if id == "" {
		manager.logger.Warn("User deactivation failed: empty user ID")
		return false, ccc.NewInvalidInputError("user ID", "cannot be empty")
	}

	user, err := manager.userRepository.FindById(id)
	if err != nil {
		manager.logger.Error("Failed to find user for deactivation", "user_id", id, "error", err)
		return false, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		manager.logger.Warn("User not found for deactivation", "user_id", id)
		return false, ccc.NewResourceNotFoundError(id, "User")
	}

	user.IsActive = false
	success, err := manager.userRepository.Update(user)
	if err != nil {
		manager.logger.Error("Failed to deactivate user", "user_id", id, "username", user.UserName, "error", err)
		return false, ccc.NewDatabaseError("update user", err)
	}

	if success {
		manager.logger.Info("User deactivated successfully", "user_id", id, "username", user.UserName)
	} else {
		manager.logger.Warn("User deactivation operation returned false", "user_id", id, "username", user.UserName)
	}

	return success, nil
}

// LockUser locks a user by their ID
func (manager *DefaultUserManager) LockUser(id string) (bool, error) {
	manager.logger.Info("Locking user via user manager", "user_id", id)

	if id == "" {
		manager.logger.Warn("User lock failed: empty user ID")
		return false, ccc.NewInvalidInputError("user ID", "cannot be empty")
	}

	user, err := manager.userRepository.FindById(id)
	if err != nil {
		manager.logger.Error("Failed to find user for locking", "user_id", id, "error", err)
		return false, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		manager.logger.Warn("User not found for locking", "user_id", id)
		return false, ccc.NewResourceNotFoundError(id, "User")
	}

	locked, err := manager.securityService.LockUser(*user)
	if err != nil {
		manager.logger.Error("Security service failed to lock user", "user_id", id, "username", user.UserName, "error", err)
		return false, ccc.NewDatabaseError("lock user", err)
	}

	if locked {
		manager.logger.Info("User locked successfully via user manager", "user_id", id, "username", user.UserName)
	} else {
		manager.logger.Warn("User lock operation returned false", "user_id", id, "username", user.UserName)
	}

	return locked, nil
}

// UnlockUser unlocks a user by their ID
func (manager *DefaultUserManager) UnlockUser(id string) (bool, error) {
	manager.logger.Info("Unlocking user via user manager", "user_id", id)

	if id == "" {
		manager.logger.Warn("User unlock failed: empty user ID")
		return false, ccc.NewInvalidInputError("user ID", "cannot be empty")
	}

	user, err := manager.userRepository.FindById(id)
	if err != nil {
		manager.logger.Error("Failed to find user for unlocking", "user_id", id, "error", err)
		return false, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		manager.logger.Warn("User not found for unlocking", "user_id", id)
		return false, ccc.NewResourceNotFoundError(id, "User")
	}

	unlocked, err := manager.securityService.UnlockUser(*user)
	if err != nil {
		manager.logger.Error("Security service failed to unlock user", "user_id", id, "username", user.UserName, "error", err)
		return false, ccc.NewDatabaseError("unlock user", err)
	}

	if unlocked {
		manager.logger.Info("User unlocked successfully via user manager", "user_id", id, "username", user.UserName)
	} else {
		manager.logger.Warn("User unlock operation returned false", "user_id", id, "username", user.UserName)
	}

	return unlocked, nil
}

// ChangePassword changes the password for a user
func (manager *DefaultUserManager) ChangePassword(request ChangePasswordRequest) (bool, error) {
	manager.logger.Info("Changing user password", "user_id", request.UserId)

	user, err := manager.userRepository.FindById(request.UserId)
	if err != nil {
		manager.logger.Error("Failed to find user for password change", "user_id", request.UserId, "error", err)
		return false, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		manager.logger.Warn("User not found for password change", "user_id", request.UserId)
		return false, ccc.NewResourceNotFoundError(request.UserId, "User")
	}

	// Verify the old password
	isValid, err := manager.securityService.VerifyUserPassword(*user, request.OldPassword)
	if err != nil {
		manager.logger.Error("Failed to verify old password during password change", "user_id", request.UserId, "username", user.UserName, "error", err)
		return false, ccc.NewInternalError("verify user password", err)
	}
	if !isValid {
		manager.logger.Warn("Invalid old password provided for password change", "user_id", request.UserId, "username", user.UserName)
		return false, ccc.NewUnauthorizedError("operation not authorized")
	}

	// Validate the new password meets requirements
	isValidNewPassword, err := manager.IsValidPassword(request.NewPassword)
	if err != nil {
		manager.logger.Error("Failed to validate new password during password change", "user_id", request.UserId, "username", user.UserName, "error", err)
		return false, err
	}
	if !isValidNewPassword {
		manager.logger.Warn("Invalid new password provided for password change", "user_id", request.UserId, "username", user.UserName)
		return false, ccc.NewInvalidInputError("new password", "does not meet password requirements")
	}

	pwHash, pwSalt, err := manager.encryptionService.Hash(request.NewPassword)
	if err != nil {
		manager.logger.Error("Failed to hash new password", "user_id", request.UserId, "username", user.UserName, "error", err)
		return false, ccc.NewInternalError("hash password", err)
	}

	user.PasswordHash = pwHash
	user.PasswordSalt = pwSalt
	user.ModifiedAt = time.Now()

	// Decrypt the current encryption key using the old password
	plainMek, err := manager.securityService.UncoverMek(*user, request.OldPassword)
	if err != nil {
		manager.logger.Error("Failed to uncover MEK during password change", "user_id", request.UserId, "username", user.UserName, "error", err)
		return false, ccc.NewInternalError("uncover MEK", err)
	}

	// Re-encrypt the encryption key with the new password-derived key
	mek, pdkSalt, err := manager.securityService.EncryptMek(plainMek, request.NewPassword)
	if err != nil {
		manager.logger.Error("Failed to encrypt MEK with new password", "user_id", request.UserId, "username", user.UserName, "error", err)
		return false, ccc.NewInternalError("encrypt MEK", err)
	}

	user.Mek = mek
	user.PdkSalt = pdkSalt

	success, err := manager.userRepository.Update(user)
	if err != nil {
		manager.logger.Error("Failed to update user with new password", "user_id", request.UserId, "username", user.UserName, "error", err)
		return false, ccc.NewDatabaseError("update user", err)
	}

	if success {
		manager.logger.Info("User password changed successfully", "user_id", request.UserId, "username", user.UserName)
	} else {
		manager.logger.Warn("Password change operation returned false", "user_id", request.UserId, "username", user.UserName)
	}

	return success, nil
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
		return false, ccc.NewInvalidInputErrorWithMessage("password", "cannot be empty", "Password cannot be empty")
	}

	const minPasswordLength = 16

	// 1. Check length
	if len(password) < minPasswordLength {
		return false, ccc.NewInvalidInputErrorWithMessage(
			"password",
			"must be at least "+fmt.Sprint(minPasswordLength)+" characters long",
			fmt.Sprintf("Password must be at least %d characters long", minPasswordLength),
		)
	}

	// 2. Check inclusion of lower case letters
	if !hasLowerRegexp.MatchString(password) {
		return false, ccc.NewInvalidInputErrorWithMessage(
			"password",
			"must contain at least one lowercase letter",
			"Password must contain at least one lowercase letter (a-z)",
		)
	}

	// 3. Check inclusion of upper case letters
	if !hasUpperRegexp.MatchString(password) {
		return false, ccc.NewInvalidInputErrorWithMessage(
			"password",
			"must contain at least one uppercase letter",
			"Password must contain at least one uppercase letter (A-Z)",
		)
	}

	// 4. Check inclusion of numbers
	if !hasDigitRegexp.MatchString(password) {
		return false, ccc.NewInvalidInputErrorWithMessage(
			"password",
			"must contain at least one number",
			"Password must contain at least one number (0-9)",
		)
	}

	// 5. Check inclusion of allowed special characters
	if !hasSpecialRegexp.MatchString(password) {
		return false, ccc.NewInvalidInputErrorWithMessage(
			"password",
			"must contain at least one special character. Allowed special characters are: "+displaySpecialChars,
			"Password must contain at least one special character. Allowed special characters are: "+displaySpecialChars,
		)
	}

	// 6. Check that all characters in the password are from the allowed set
	// (alphanumeric or one of the defined special characters)
	if !allAllowedCharsRegexp.MatchString(password) {
		return false, ccc.NewInvalidInputErrorWithMessage(
			"password",
			"contains invalid characters. Only letters, numbers, and the following special characters are allowed: "+displaySpecialChars,
			"Password contains invalid characters. Only letters, numbers, and the following special characters are allowed: "+displaySpecialChars,
		)
	}

	return true, nil
}

// DeleteUser deletes a user by their ID
func (manager *DefaultUserManager) DeleteUser(id string) (bool, error) {
	manager.logger.Info("Deleting user", "user_id", id)

	if id == "" {
		manager.logger.Warn("User deletion failed: empty user ID")
		return false, ccc.NewInvalidInputError("user ID", "cannot be empty")
	}

	success, err := manager.userRepository.Remove(id)
	if err != nil {
		manager.logger.Error("Failed to delete user from repository", "user_id", id, "error", err)
		return false, ccc.NewDatabaseError("remove user", err)
	}

	if success {
		manager.logger.Info("User deleted successfully", "user_id", id)
	} else {
		manager.logger.Debug("User deletion returned false (user may not have existed)", "user_id", id)
	}

	// Idempotency: if the user was not found, we still return success; no error is raised
	return success, nil
}

// VerifyPassword checks if the provided password matches the user's password
func (manager *DefaultUserManager) VerifyPassword(userId string, password string) (bool, error) {
	manager.logger.Debug("Verifying user password", "user_id", userId)

	user, err := manager.userRepository.FindById(userId)
	if err != nil {
		manager.logger.Error("Failed to find user for password verification", "user_id", userId, "error", err)
		return false, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		manager.logger.Warn("User not found for password verification", "user_id", userId)
		return false, ccc.NewResourceNotFoundError(userId, "User")
	}

	success, err := manager.securityService.VerifyUserPassword(*user, password)
	if err != nil {
		manager.logger.Error("Security service failed to verify password", "user_id", userId, "username", user.UserName, "error", err)
		return false, ccc.NewInternalError("verify user password", err)
	}

	if success {
		manager.logger.Debug("Password verification successful", "user_id", userId, "username", user.UserName)
	} else {
		manager.logger.Warn("Password verification failed", "user_id", userId, "username", user.UserName)
	}

	return success, nil
}

// GenerateRecoveryCode generates a new recovery code for a user.
func (manager *DefaultUserManager) GenerateRecoveryCode(request GenerateRecoveryCodeRequest) (GenerateRecoveryCodeResponse, error) {
	manager.logger.Info("Generating recovery code for user", "user_id", request.UserId)

	if request.UserId == "" {
		manager.logger.Warn("Recovery code generation failed: empty user ID")
		return GenerateRecoveryCodeResponse{}, ccc.NewInvalidInputError("user ID", "cannot be empty")
	}

	if request.Password == "" {
		manager.logger.Warn("Recovery code generation failed: empty password")
		return GenerateRecoveryCodeResponse{}, ccc.NewInvalidInputError("password", "cannot be empty")
	}

	// Find the user
	user, err := manager.userRepository.FindById(request.UserId)
	if err != nil {
		manager.logger.Error("Failed to find user for recovery code generation", "user_id", request.UserId, "error", err)
		return GenerateRecoveryCodeResponse{}, ccc.NewDatabaseError("find user by ID", err)
	}

	if user == nil {
		manager.logger.Warn("User not found for recovery code generation", "user_id", request.UserId)
		return GenerateRecoveryCodeResponse{}, ccc.NewResourceNotFoundError(request.UserId, "User")
	}

	// Verify the password before generating recovery code
	manager.logger.Debug("Verifying password for recovery code generation", "user_id", request.UserId, "username", user.UserName)
	passwordValid, err := manager.securityService.VerifyUserPassword(*user, request.Password)
	if err != nil {
		manager.logger.Error("Password verification failed during recovery code generation", "user_id", request.UserId, "username", user.UserName, "error", err)
		return GenerateRecoveryCodeResponse{}, err
	}

	if !passwordValid {
		manager.logger.Warn("Invalid password provided for recovery code generation", "user_id", request.UserId, "username", user.UserName)
		return GenerateRecoveryCodeResponse{}, ccc.NewUnauthorizedError("Invalid password")
	}

	// Uncover the MEK using the current password
	manager.logger.Debug("Uncovering MEK for recovery code generation", "user_id", request.UserId, "username", user.UserName)
	plainMek, err := manager.securityService.UncoverMek(*user, request.Password)
	if err != nil {
		manager.logger.Error("Failed to uncover MEK for recovery code generation", "user_id", request.UserId, "username", user.UserName, "error", err)
		return GenerateRecoveryCodeResponse{}, ccc.NewInternalError("uncover MEK", err)
	}

	if plainMek == "" {
		manager.logger.Error("MEK uncovering returned empty result for recovery code generation", "user_id", request.UserId, "username", user.UserName)
		return GenerateRecoveryCodeResponse{}, ccc.NewInternalError("MEK uncovering returned empty result", nil)
	}

	// Generate the new recovery code
	recoveryCode, hash, salt, err := manager.securityService.GenerateRecoveryCode()
	if err != nil {
		manager.logger.Error("Security service failed to generate recovery code", "user_id", request.UserId, "username", user.UserName, "error", err)
		return GenerateRecoveryCodeResponse{}, ccc.NewInternalError("generate recovery code", err)
	}

	// Encrypt the MEK with the new recovery code
	manager.logger.Debug("Encrypting MEK with new recovery code", "user_id", request.UserId, "username", user.UserName)
	encryptedMek, err := manager.securityService.EncryptMekWithRecoveryCode(plainMek, recoveryCode, salt)
	if err != nil {
		manager.logger.Error("Failed to encrypt MEK with new recovery code", "user_id", request.UserId, "username", user.UserName, "error", err)
		return GenerateRecoveryCodeResponse{}, ccc.NewInternalError("encrypt MEK with recovery code", err)
	}

	// Update the user with the new recovery code and encrypted MEK
	user.RecoveryCodeHash = hash
	user.RecoveryCodeSalt = salt
	user.RecoveryMek = encryptedMek
	user.RecoveryGenerated = time.Now()
	user.ModifiedAt = time.Now()

	success, err := manager.userRepository.Update(user)
	if err != nil {
		manager.logger.Error("Failed to save recovery code for user", "user_id", request.UserId, "username", user.UserName, "error", err)
		return GenerateRecoveryCodeResponse{}, ccc.NewDatabaseError("update user", err)
	}

	if !success {
		manager.logger.Warn("Recovery code save operation returned false", "user_id", request.UserId, "username", user.UserName)
		return GenerateRecoveryCodeResponse{}, ccc.NewOperationFailedError("save recovery code", "update operation returned false")
	}

	manager.logger.Info("Recovery code generated successfully", "user_id", request.UserId, "username", user.UserName)
	return GenerateRecoveryCodeResponse{
		RecoveryCode: recoveryCode,
		Generated:    user.RecoveryGenerated.Format(time.RFC3339),
	}, nil
}
