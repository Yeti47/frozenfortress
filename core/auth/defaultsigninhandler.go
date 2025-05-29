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
}

// NewDefaultSignInHandler creates a new DefaultSignInHandler with all dependencies injected
func NewDefaultSignInHandler(
	userRepo UserRepository,
	signInHistoryRepo SignInHistoryItemRepository,
	securityService SecurityService,
	config ccc.AppConfig) *DefaultSignInHandler {

	return &DefaultSignInHandler{
		userRepository:          userRepo,
		signInHistoryRepository: signInHistoryRepo,
		securityService:         securityService,
		config:                  config,
	}
}

// HandleSignIn performs the core sign-in logic including validation, authentication,
// failed attempt tracking, and sign-in history recording
func (h *DefaultSignInHandler) HandleSignIn(request SignInRequest, context SignInContext) (SignInResult, error) {
	// Validate input
	if request.UserName == "" {
		return SignInResult{
			Success:      false,
			ErrorMessage: "Invalid credentials",
		}, ccc.NewInvalidInputError("username", "cannot be empty")
	}
	if request.Password == "" {
		return SignInResult{
			Success:      false,
			ErrorMessage: "Invalid credentials",
		}, ccc.NewInvalidInputError("password", "cannot be empty")
	}

	user, err := h.userRepository.FindByUserName(request.UserName)
	if err != nil {
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
		// Even for non-existent users, we still log the attempt
		historyItem.DenialReason = "Invalid credentials"
		_ = h.signInHistoryRepository.Add(historyItem)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Invalid credentials",
		}, ccc.NewUnauthorizedError("invalid credentials")
	}

	// Set user ID now that we have it
	historyItem.UserId = user.Id

	// Check if account is locked
	if user.IsLocked {
		historyItem.DenialReason = "Account is locked"
		_ = h.signInHistoryRepository.Add(historyItem)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Account is locked",
		}, ccc.NewForbiddenError("account is locked")
	}

	// Check if account is inactive
	if !user.IsActive {
		historyItem.DenialReason = "Account is inactive"
		_ = h.signInHistoryRepository.Add(historyItem)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Account is inactive",
		}, ccc.NewForbiddenError("account is inactive")
	}

	// Verify password using SecurityService
	valid, err := h.securityService.VerifyUserPassword(*user, request.Password)
	if err != nil {
		historyItem.DenialReason = "Password verification failed"
		_ = h.signInHistoryRepository.Add(historyItem)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Invalid credentials",
		}, ccc.NewUnauthorizedError("invalid credentials")
	}

	// If password verification failed
	if !valid {
		// Log the failed attempt
		historyItem.DenialReason = "Invalid credentials"
		_ = h.signInHistoryRepository.Add(historyItem)

		// Check for too many failed attempts, but only if the account is active
		if user.IsActive {
			failedAttempts, err := h.signInHistoryRepository.GetRecentFailedSignInsByUserName(
				user.UserName, h.config.SignInAttemptWindow)

			if err == nil && len(failedAttempts) >= h.config.MaxSignInAttempts {
				// Lock the account using the SecurityService
				_, _ = h.securityService.LockUser(*user)
				return SignInResult{
					Success:      false,
					ErrorMessage: "Account has been locked due to too many failed attempts",
				}, ccc.NewForbiddenError("account locked due to too many failed attempts")
			}
		}
		return SignInResult{
			Success:      false,
			ErrorMessage: "Invalid credentials",
		}, ccc.NewUnauthorizedError("invalid credentials")
	}

	// Authentication succeeded - get MEK
	mek, err := h.securityService.UncoverMek(*user, request.Password)
	if err != nil {
		_ = h.signInHistoryRepository.Add(historyItem)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Internal error",
		}, ccc.NewInternalError("failed to uncover MEK", err)
	}
	if mek == "" {
		_ = h.signInHistoryRepository.Add(historyItem)
		return SignInResult{
			Success:      false,
			ErrorMessage: "Internal error",
		}, ccc.NewInternalError("MEK uncovering returned empty result", nil)
	}

	// Update history item to reflect successful login
	historyItem.Successful = true
	_ = h.signInHistoryRepository.Add(historyItem)

	return SignInResult{
		Success: true,
		User:    user,
		Mek:     mek,
	}, nil
}
