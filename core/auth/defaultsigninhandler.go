package auth

import (
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
)

// DefaultSignInHandler implements SignInHandler and contains all the core sign-in logic
// that is independent of session management
type DefaultSignInHandler struct {
	userRepository          UserRepository
	signInHistoryRepository SignInHistoryItemRepository
	securityService         SecurityService
	encryptionService       encryption.EncryptionService
	config                  ccc.AppConfig
	logger                  ccc.Logger
}

// NewDefaultSignInHandler creates a new DefaultSignInHandler with all dependencies injected
func NewDefaultSignInHandler(
	userRepo UserRepository,
	signInHistoryRepo SignInHistoryItemRepository,
	securityService SecurityService,
	encryptionService encryption.EncryptionService,
	config ccc.AppConfig,
	logger ccc.Logger) *DefaultSignInHandler {

	if logger == nil {
		logger = ccc.NopLogger
	}

	return &DefaultSignInHandler{
		userRepository:          userRepo,
		signInHistoryRepository: signInHistoryRepo,
		securityService:         securityService,
		encryptionService:       encryptionService,
		config:                  config,
		logger:                  logger,
	}
}

// Helper function to create sign-in history item
func (h *DefaultSignInHandler) createHistoryItem(userName, userId string, context SignInContext) *SignInHistoryItem {
	return &SignInHistoryItem{
		UserId:       userId,
		UserName:     userName,
		IPAddress:    context.IPAddress,
		UserAgent:    context.UserAgent,
		ClientType:   string(context.ClientType),
		Successful:   false,
		Timestamp:    time.Now(),
		DenialReason: "",
	}
}

// Helper function to find and validate user existence for both sign-in types
func (h *DefaultSignInHandler) findAndValidateUser(userName string, context SignInContext, invalidCredentialsMessage string) (*User, *SignInHistoryItem, error) {
	historyItem := h.createHistoryItem(userName, "", context)

	user, err := h.userRepository.FindByUserName(userName)
	if err != nil {
		h.logger.Error("Failed to find user", "username", userName, "ip_address", context.IPAddress, "error", err)
		return nil, historyItem, ccc.NewDatabaseError("find user by username", err)
	}

	// Check if user exists
	if user == nil || user.Id == "" {
		h.logger.Warn("Sign-in failed: user not found", "username", userName, "ip_address", context.IPAddress)
		historyItem.DenialReason = "User not found"
		_ = h.signInHistoryRepository.Add(historyItem)
		return nil, historyItem, nil // Return nil user to indicate not found
	}

	h.logger.Debug("User found", "username", userName, "user_id", user.Id, "ip_address", context.IPAddress)
	historyItem.UserId = user.Id
	return user, historyItem, nil
}

// Helper function to validate user status (active, not locked)
func (h *DefaultSignInHandler) validateUserStatus(user *User, historyItem *SignInHistoryItem, context SignInContext) (bool, string) {
	if user.IsLocked {
		h.logger.Warn("Sign-in failed: account is locked", "username", user.UserName, "user_id", user.Id, "ip_address", context.IPAddress)
		historyItem.DenialReason = "Account is locked"
		_ = h.signInHistoryRepository.Add(historyItem)
		return false, "Account is locked"
	}

	if !user.IsActive {
		h.logger.Warn("Sign-in failed: account is inactive", "username", user.UserName, "user_id", user.Id, "ip_address", context.IPAddress)
		historyItem.DenialReason = "Account is inactive"
		_ = h.signInHistoryRepository.Add(historyItem)
		return false, "Account is inactive"
	}

	return true, ""
}

// Helper function to handle failed attempts and rate limiting
func (h *DefaultSignInHandler) handleFailedAttempt(user *User, historyItem *SignInHistoryItem, denialReason string, context SignInContext, attemptType string) (bool, string) {
	historyItem.DenialReason = denialReason
	_ = h.signInHistoryRepository.Add(historyItem)

	// Check for too many failed attempts, but only if the account is active
	if user.IsActive {
		failedAttempts, err := h.signInHistoryRepository.GetRecentFailedSignInsByUserName(
			user.UserName, h.config.SignInAttemptWindow)

		if err != nil {
			h.logger.Error("Failed to check recent failed attempts", "username", user.UserName, "user_id", user.Id, "attempt_type", attemptType, "error", err)
			return false, "Internal error"
		}

		h.logger.Debug("Checked recent failed attempts", "username", user.UserName, "user_id", user.Id, "attempt_type", attemptType, "failed_count", len(failedAttempts), "max_attempts", h.config.MaxSignInAttempts)

		if len(failedAttempts) >= h.config.MaxSignInAttempts {
			h.logger.Warn("Locking account due to too many failed attempts",
				"username", user.UserName,
				"user_id", user.Id,
				"attempt_type", attemptType,
				"failed_attempts", len(failedAttempts),
				"max_attempts", h.config.MaxSignInAttempts,
				"ip_address", context.IPAddress)

			// Lock the account using the SecurityService
			_, lockErr := h.securityService.LockUser(*user)
			if lockErr != nil {
				h.logger.Error("Failed to lock user account after failed attempts", "username", user.UserName, "user_id", user.Id, "attempt_type", attemptType, "error", lockErr)
			} else {
				h.logger.Info("Account locked successfully due to failed attempts", "username", user.UserName, "user_id", user.Id, "attempt_type", attemptType)
			}

			return true, "Account has been locked due to too many failed attempts"
		}
	}

	return false, denialReason
}

// Helper function to log successful attempt
func (h *DefaultSignInHandler) logSuccessfulAttempt(historyItem *SignInHistoryItem, successMessage string) {
	historyItem.Successful = true
	historyItem.DenialReason = successMessage
	_ = h.signInHistoryRepository.Add(historyItem)
}

// HandleSignIn performs the core sign-in logic including validation, authentication,
// failed attempt tracking, and sign-in history recording
func (h *DefaultSignInHandler) HandleSignIn(request SignInRequest, context SignInContext) (SignInResult, error) {
	h.logger.Info("Processing sign-in attempt", "username", request.UserName, "ip_address", context.IPAddress, "client_type", context.ClientType)

	// Validate input
	if request.UserName == "" {
		h.logger.Warn("Sign-in failed: empty username", "ip_address", context.IPAddress)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Invalid credentials",
		}, nil
	}
	if request.Password == "" {
		h.logger.Warn("Sign-in failed: empty password", "username", request.UserName, "ip_address", context.IPAddress)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Invalid credentials",
		}, nil
	}

	// Find and validate user
	user, historyItem, err := h.findAndValidateUser(request.UserName, context, "Invalid credentials")
	if err != nil {
		return SignInResult{
			Success:      false,
			ErrorMessage: "Internal error",
		}, err
	}
	if user == nil {
		return SignInResult{
			Success:      false,
			ErrorMessage: "Invalid credentials",
		}, nil
	}

	// Validate user status
	valid, errorMessage := h.validateUserStatus(user, historyItem, context)
	if !valid {
		return SignInResult{
			Success:      false,
			ErrorMessage: errorMessage,
		}, nil
	}

	// Verify password using SecurityService
	h.logger.Debug("Verifying password for sign-in", "username", request.UserName, "user_id", user.Id)
	passwordValid, err := h.securityService.VerifyUserPassword(*user, request.Password)
	if err != nil {
		h.logger.Error("Password verification failed during sign-in", "username", request.UserName, "user_id", user.Id, "ip_address", context.IPAddress, "error", err)
		historyItem.DenialReason = "Password verification failed"
		_ = h.signInHistoryRepository.Add(historyItem)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Internal error",
		}, err
	}

	// If password verification failed
	if !passwordValid {
		h.logger.Warn("Sign-in failed: invalid password", "username", request.UserName, "user_id", user.Id, "ip_address", context.IPAddress)

		accountLocked, errorMsg := h.handleFailedAttempt(user, historyItem, "Invalid credentials", context, "password")
		if accountLocked {
			return SignInResult{
				Success:      false,
				ErrorMessage: errorMsg,
			}, nil
		}
		return SignInResult{
			Success:      false,
			ErrorMessage: "Invalid credentials",
		}, nil
	}

	// Authentication succeeded - get MEK
	h.logger.Debug("Password verification successful, uncovering MEK", "username", request.UserName, "user_id", user.Id)
	mek, err := h.securityService.UncoverMek(*user, request.Password)
	if err != nil {
		h.logger.Error("Failed to uncover MEK after successful authentication", "username", request.UserName, "user_id", user.Id, "error", err)
		_ = h.signInHistoryRepository.Add(historyItem)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Internal error",
		}, ccc.NewInternalError("failed to uncover MEK", err)
	}
	if mek == "" {
		h.logger.Error("MEK uncovering returned empty result", "username", request.UserName, "user_id", user.Id)
		_ = h.signInHistoryRepository.Add(historyItem)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Internal error",
		}, ccc.NewInternalError("MEK uncovering returned empty result", nil)
	}

	h.logger.Debug("MEK uncovered successfully", "username", request.UserName, "user_id", user.Id)

	// Log successful sign-in
	h.logSuccessfulAttempt(historyItem, "Sign-in successful")

	h.logger.Info("Sign-in successful", "username", request.UserName, "user_id", user.Id, "ip_address", context.IPAddress, "client_type", context.ClientType)

	return SignInResult{
		Success: true,
		User:    user,
		Mek:     mek,
	}, nil
}

// HandleRecoverySignIn performs recovery sign-in using recovery code and new password
func (h *DefaultSignInHandler) HandleRecoverySignIn(request RecoverySignInRequest, context SignInContext) (RecoverySignInResult, error) {
	h.logger.Info("Processing recovery sign-in attempt", "username", request.UserName, "ip_address", context.IPAddress, "client_type", context.ClientType)

	// Validate input
	if request.UserName == "" {
		h.logger.Warn("Recovery sign-in failed: empty username", "ip_address", context.IPAddress)
		return RecoverySignInResult{
			Success:      false,
			ErrorMessage: "Invalid username or recovery code",
		}, nil
	}

	if request.RecoveryCode == "" {
		h.logger.Warn("Recovery sign-in failed: empty recovery code", "username", request.UserName, "ip_address", context.IPAddress)
		return RecoverySignInResult{
			Success:      false,
			ErrorMessage: "Invalid username or recovery code",
		}, nil
	}

	if request.NewPassword == "" {
		h.logger.Warn("Recovery sign-in failed: empty new password", "username", request.UserName, "ip_address", context.IPAddress)
		return RecoverySignInResult{
			Success:      false,
			ErrorMessage: "New password cannot be empty",
		}, nil
	}

	// Find and validate user
	user, historyItem, err := h.findAndValidateUser(request.UserName, context, "Invalid username or recovery code")
	if err != nil {
		return RecoverySignInResult{
			Success:      false,
			ErrorMessage: "Internal error",
		}, err
	}
	if user == nil {
		return RecoverySignInResult{
			Success:      false,
			ErrorMessage: "Invalid username or recovery code",
		}, nil
	}

	// Validate user status
	valid, errorMessage := h.validateUserStatus(user, historyItem, context)
	if !valid {
		return RecoverySignInResult{
			Success:      false,
			ErrorMessage: errorMessage,
		}, nil
	}

	// Recover MEK using recovery code and new password
	newMek, newPdkSalt, err := h.securityService.RecoverMek(*user, request.RecoveryCode, request.NewPassword)
	if err != nil {
		h.logger.Warn("Recovery sign-in failed: invalid recovery code", "username", request.UserName, "user_id", user.Id, "ip_address", context.IPAddress, "error", err)

		accountLocked, errorMsg := h.handleFailedAttempt(user, historyItem, "Invalid recovery code", context, "recovery")
		if accountLocked {
			return RecoverySignInResult{
				Success:      false,
				ErrorMessage: errorMsg,
			}, nil
		}

		return RecoverySignInResult{
			Success:      false,
			ErrorMessage: "Invalid username or recovery code",
		}, nil
	}

	// Hash the new password
	newPasswordHash, newPasswordSalt, err := h.encryptionService.Hash(request.NewPassword)
	if err != nil {
		h.logger.Error("Failed to hash new password during recovery", "username", request.UserName, "user_id", user.Id, "error", err)
		historyItem.DenialReason = "Internal error"
		_ = h.signInHistoryRepository.Add(historyItem)
		return RecoverySignInResult{
			Success:      false,
			ErrorMessage: "Internal error",
		}, ccc.NewInternalError("failed to hash new password", err)
	}

	// Update user with new password and MEK, mark recovery code as used
	user.PasswordHash = newPasswordHash
	user.PasswordSalt = newPasswordSalt
	user.Mek = newMek
	user.PdkSalt = newPdkSalt
	now := time.Now()
	user.RecoveryUsed = &now
	user.ModifiedAt = now

	// Save updated user
	success, err := h.userRepository.Update(user)
	if err != nil || !success {
		h.logger.Error("Failed to update user during recovery", "username", request.UserName, "user_id", user.Id, "error", err)
		historyItem.DenialReason = "Internal error"
		_ = h.signInHistoryRepository.Add(historyItem)
		return RecoverySignInResult{
			Success:      false,
			ErrorMessage: "Internal error",
		}, ccc.NewDatabaseError("update user", err)
	}

	// Generate new MEK for the result (it should be the plaintext MEK, not encrypted)
	plainMek, err := h.securityService.UncoverMek(*user, request.NewPassword)
	if err != nil {
		h.logger.Error("Failed to uncover MEK after recovery", "username", request.UserName, "user_id", user.Id, "error", err)
		historyItem.DenialReason = "Internal error"
		_ = h.signInHistoryRepository.Add(historyItem)
		return RecoverySignInResult{
			Success:      false,
			ErrorMessage: "Internal error",
		}, ccc.NewInternalError("failed to uncover MEK after recovery", err)
	}

	// Log successful recovery
	h.logSuccessfulAttempt(historyItem, "Recovery successful")

	h.logger.Info("Recovery sign-in successful", "username", request.UserName, "user_id", user.Id, "ip_address", context.IPAddress, "client_type", context.ClientType)

	return RecoverySignInResult{
		Success: true,
		User:    user,
		Mek:     plainMek,
	}, nil
}
