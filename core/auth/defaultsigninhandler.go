package auth

import (
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// DefaultSignInHandler implements SignInHandler and contains all the core sign-in logic
// that is independent of session management
type DefaultSignInHandler struct {
	userRepository          UserRepository
	signInHistoryRepository SignInHistoryItemRepository
	securityService         SecurityService
	config                  ccc.AppConfig
	logger                  ccc.Logger
}

// NewDefaultSignInHandler creates a new DefaultSignInHandler with all dependencies injected
func NewDefaultSignInHandler(
	userRepo UserRepository,
	signInHistoryRepo SignInHistoryItemRepository,
	securityService SecurityService,
	config ccc.AppConfig,
	logger ccc.Logger) *DefaultSignInHandler {

	if logger == nil {
		logger = ccc.NopLogger
	}

	return &DefaultSignInHandler{
		userRepository:          userRepo,
		signInHistoryRepository: signInHistoryRepo,
		securityService:         securityService,
		config:                  config,
		logger:                  logger,
	}
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
		}, ccc.NewInvalidInputError("username", "cannot be empty")
	}
	if request.Password == "" {
		h.logger.Warn("Sign-in failed: empty password", "username", request.UserName, "ip_address", context.IPAddress)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Invalid credentials",
		}, ccc.NewInvalidInputError("password", "cannot be empty")
	}

	user, err := h.userRepository.FindByUserName(request.UserName)
	if err != nil {
		h.logger.Error("Failed to find user during sign-in", "username", request.UserName, "ip_address", context.IPAddress, "error", err)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Internal error",
		}, ccc.NewDatabaseError("find user by username", err)
	}

	// Prepare sign-in history object
	historyItem := &SignInHistoryItem{
		UserName:   request.UserName,
		Timestamp:  time.Now(),
		IPAddress:  context.IPAddress,
		UserAgent:  context.UserAgent,
		ClientType: string(context.ClientType),
		Successful: false, // Default to failed, will update if successful
	}

	// Check if user exists
	if user == nil || user.Id == "" {
		h.logger.Warn("Sign-in failed: user not found", "username", request.UserName, "ip_address", context.IPAddress)
		// Even for non-existent users, we still log the attempt
		historyItem.DenialReason = "Invalid credentials"
		_ = h.signInHistoryRepository.Add(historyItem)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Invalid credentials",
		}, ccc.NewUnauthorizedError("invalid credentials")
	}

	h.logger.Debug("User found for sign-in", "username", request.UserName, "user_id", user.Id, "ip_address", context.IPAddress)

	// Set user ID now that we have it
	historyItem.UserId = user.Id

	// Check if account is locked
	if user.IsLocked {
		h.logger.Warn("Sign-in failed: account is locked", "username", request.UserName, "user_id", user.Id, "ip_address", context.IPAddress)
		historyItem.DenialReason = "Account is locked"
		_ = h.signInHistoryRepository.Add(historyItem)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Account is locked",
		}, ccc.NewForbiddenError("account is locked")
	}

	// Check if account is inactive
	if !user.IsActive {
		h.logger.Warn("Sign-in failed: account is inactive", "username", request.UserName, "user_id", user.Id, "ip_address", context.IPAddress)
		historyItem.DenialReason = "Account is inactive"
		_ = h.signInHistoryRepository.Add(historyItem)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Account is inactive",
		}, ccc.NewForbiddenError("account is inactive")
	}

	// Verify password using SecurityService
	h.logger.Debug("Verifying password for sign-in", "username", request.UserName, "user_id", user.Id)
	valid, err := h.securityService.VerifyUserPassword(*user, request.Password)
	if err != nil {
		h.logger.Error("Password verification failed during sign-in", "username", request.UserName, "user_id", user.Id, "ip_address", context.IPAddress, "error", err)
		historyItem.DenialReason = "Password verification failed"
		_ = h.signInHistoryRepository.Add(historyItem)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Invalid credentials",
		}, ccc.NewUnauthorizedError("invalid credentials")
	}

	// If password verification failed
	if !valid {
		h.logger.Warn("Sign-in failed: invalid password", "username", request.UserName, "user_id", user.Id, "ip_address", context.IPAddress)

		// Log the failed attempt
		historyItem.DenialReason = "Invalid credentials"
		_ = h.signInHistoryRepository.Add(historyItem)

		// Check for too many failed attempts, but only if the account is active
		if user.IsActive {
			failedAttempts, err := h.signInHistoryRepository.GetRecentFailedSignInsByUserName(
				user.UserName, h.config.SignInAttemptWindow)

			if err != nil {
				h.logger.Error("Failed to check recent failed sign-in attempts", "username", request.UserName, "user_id", user.Id, "error", err)
			} else {
				h.logger.Debug("Checked recent failed sign-in attempts", "username", request.UserName, "user_id", user.Id, "failed_count", len(failedAttempts), "max_attempts", h.config.MaxSignInAttempts)

				if len(failedAttempts) >= h.config.MaxSignInAttempts {
					h.logger.Warn("Locking account due to too many failed sign-in attempts",
						"username", request.UserName,
						"user_id", user.Id,
						"failed_attempts", len(failedAttempts),
						"max_attempts", h.config.MaxSignInAttempts,
						"ip_address", context.IPAddress)

					// Lock the account using the SecurityService
					_, lockErr := h.securityService.LockUser(*user)
					if lockErr != nil {
						h.logger.Error("Failed to lock user account after failed attempts", "username", request.UserName, "user_id", user.Id, "error", lockErr)
					} else {
						h.logger.Info("Account locked successfully due to failed attempts", "username", request.UserName, "user_id", user.Id)
					}

					return SignInResult{
						Success:      false,
						ErrorMessage: "Account has been locked due to too many failed attempts",
					}, ccc.NewForbiddenError("account locked due to too many failed attempts")
				}
			}
		}
		return SignInResult{
			Success:      false,
			ErrorMessage: "Invalid credentials",
		}, ccc.NewUnauthorizedError("invalid credentials")
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

	// Update history item to reflect successful login
	historyItem.Successful = true
	_ = h.signInHistoryRepository.Add(historyItem)

	h.logger.Info("Sign-in successful", "username", request.UserName, "user_id", user.Id, "ip_address", context.IPAddress, "client_type", context.ClientType)

	return SignInResult{
		Success: true,
		User:    user,
		Mek:     mek,
	}, nil
}
