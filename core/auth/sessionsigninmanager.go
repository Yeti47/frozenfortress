package auth

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/sessions"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
)

// signInConfig holds configuration values for the sign-in process
type signInConfig struct {
	MaxFailedAttempts    int    // Maximum number of failed login attempts before locking account
	FailedAttemptsWindow int    // Time window in minutes for counting failed attempts
	SessionName          string // Name of the session/cookie used for authentication
}

// Default configuration values if environment variables aren't set
var defaultConfig = signInConfig{
	MaxFailedAttempts:    3,
	FailedAttemptsWindow: 30,
	SessionName:          "FROZENFORTRESS_SESSION",
}

// LoadConfigFromEnvironment loads configuration values from environment variables
// with FROZEN_FORTRESS prefix. If variables are missing or invalid, default values are used.
func LoadConfigFromEnvironment() signInConfig {
	config := defaultConfig

	// Try to load MaxFailedAttempts
	if envValue := os.Getenv("FROZEN_FORTRESS_MAX_FAILED_ATTEMPTS"); envValue != "" {
		if value, err := strconv.Atoi(envValue); err == nil && value > 0 {
			config.MaxFailedAttempts = value
		}
	}

	// Try to load FailedAttemptsWindow
	if envValue := os.Getenv("FROZEN_FORTRESS_FAILED_ATTEMPTS_WINDOW"); envValue != "" {
		if value, err := strconv.Atoi(envValue); err == nil && value > 0 {
			config.FailedAttemptsWindow = value
		}
	}

	// Try to load SessionName
	if envValue := os.Getenv("FROZEN_FORTRESS_SESSION_NAME"); envValue != "" {
		config.SessionName = envValue
	}

	return config
}

// SessionSignInManager implements SignInManager using gorilla sessions
// and verifies user credentials using a UserManager.
type SessionSignInManager struct {
	userManager             UserManager
	signInHistoryRepository SignInHistoryItemRepository
	sessionStore            sessions.Store
	config                  signInConfig
}

// NewSessionSignInManager creates a new SessionSignInManager with all dependencies injected
func NewSessionSignInManager(
	userManager UserManager,
	signInHistoryRepo SignInHistoryItemRepository,
	store sessions.Store,
	config signInConfig) *SessionSignInManager {

	return &SessionSignInManager{
		userManager:             userManager,
		signInHistoryRepository: signInHistoryRepo,
		sessionStore:            store,
		config:                  config,
	}
}

// SignIn verifies the user's credentials and creates a session if successful.
// Also tracks login attempts and locks accounts after too many failed attempts.
func (m *SessionSignInManager) SignIn(w http.ResponseWriter, r *http.Request, request SignInRequest) (SignInResponse, error) {
	// Find the user using UserManager instead of direct UserRepository
	userDto, err := m.userManager.GetUserByUserName(request.UserName)
	if err != nil {
		return SignInResponse{Success: false, Error: "User lookup failed"}, err
	}

	// Prepare sign-in history object
	historyItem := &SignInHistoryItem{
		UserName:   request.UserName,
		Timestamp:  time.Now(),
		Successful: false, // Default to failed, will update if successful
		// ID will be assigned by repository on insert
	}

	// Get IP and user agent if available
	if r != nil {
		historyItem.IPAddress = r.RemoteAddr
		historyItem.UserAgent = r.UserAgent()
	}

	// Check if user exists
	if userDto.UserId == "" {
		// Even for non-existent users, we still log the attempt
		_ = m.signInHistoryRepository.Add(historyItem)
		return SignInResponse{Success: false, Error: "Invalid credentials"}, nil
	}

	// Set user ID now that we have it
	historyItem.UserId = userDto.UserId

	// Check if account is locked
	if userDto.IsLocked {
		_ = m.signInHistoryRepository.Add(historyItem)
		return SignInResponse{Success: false, Error: "Account is locked"}, nil
	}

	// Verify password using UserManager
	valid, err := m.userManager.VerifyUserPassword(userDto.UserId, request.Password)
	if err != nil {
		_ = m.signInHistoryRepository.Add(historyItem)
		return SignInResponse{Success: false, Error: "Password verification failed"}, err
	}

	// If password verification failed
	if !valid {
		// Log the failed attempt
		_ = m.signInHistoryRepository.Add(historyItem)

		// Check for too many failed attempts
		failedAttempts, err := m.signInHistoryRepository.GetRecentFailedSignInsByUserName(
			userDto.UserName, m.config.FailedAttemptsWindow)

		if err == nil && len(failedAttempts) >= m.config.MaxFailedAttempts {
			// Lock the account using the UserManager
			_, _ = m.userManager.LockUser(userDto.UserId)
			return SignInResponse{Success: false, Error: "Account has been locked due to too many failed attempts"}, nil
		}

		return SignInResponse{Success: false, Error: "Invalid credentials"}, nil
	}

	// Authentication succeeded - create session
	session, err := m.sessionStore.Get(r, m.config.SessionName)
	if err != nil {
		_ = m.signInHistoryRepository.Add(historyItem)
		return SignInResponse{Success: false, Error: "Session error"}, err
	}

	session.Values["userId"] = userDto.UserId
	err = session.Save(r, w)
	if err != nil {
		_ = m.signInHistoryRepository.Add(historyItem)
		return SignInResponse{Success: false, Error: "Session save error"}, err
	}

	// Update history item to reflect successful login
	historyItem.Successful = true
	_ = m.signInHistoryRepository.Add(historyItem)

	// Already have the user DTO from earlier - userDto is already a pointer so we don't need &
	return SignInResponse{
		Success: true,
		User:    userDto,
	}, nil
}

// SignOut clears the session for the user.
func (m *SessionSignInManager) SignOut(w http.ResponseWriter, r *http.Request, request SignOutRequest) error {
	session, err := m.sessionStore.Get(r, m.config.SessionName)
	if err != nil {
		return err
	}
	session.Options.MaxAge = -1 // Mark session for deletion
	return session.Save(r, w)
}

// GetCurrentUser retrieves the currently signed in user, if any
func (m *SessionSignInManager) GetCurrentUser(r *http.Request) (*UserDto, error) {
	session, err := m.sessionStore.Get(r, m.config.SessionName)
	if err != nil {
		return nil, err
	}

	userId, ok := session.Values["userId"]
	if !ok || userId == nil {
		return nil, nil // No user is signed in
	}

	// Use UserManager instead of direct repository access
	userDto, err := m.userManager.GetUserById(userId.(string))
	if err != nil {
		return nil, err
	}

	// If user is no longer valid
	if userDto == nil || userDto.UserId == "" {
		return nil, nil
	}

	return userDto, nil
}

// IsSignedIn checks if a user is currently signed in
func (m *SessionSignInManager) IsSignedIn(r *http.Request) (bool, error) {
	user, err := m.GetCurrentUser(r)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}

// CreateSecureCookieStore creates a new cookie store for session management with secure keys
// If environment variable FROZEN_FORTRESS_SIGNING_KEY is not set, it generates a secure key
// and sets the environment variable. Similarly for FROZEN_FORTRESS_ENCRYPTION_KEY.
func CreateSecureCookieStore(encryptionService encryption.EncryptionService) (sessions.Store, error) {
	// Get or create signing key from environment
	signingKey, err := getOrCreateKey(encryptionService, "FROZEN_FORTRESS_SIGNING_KEY")
	if err != nil {
		return nil, err
	}

	// Get or create encryption key from environment
	encryptionKey, err := getOrCreateKey(encryptionService, "FROZEN_FORTRESS_ENCRYPTION_KEY")
	if err != nil {
		return nil, err
	}

	// Create cookie store with both keys - first key is authentication, second is encryption
	return sessions.NewCookieStore([]byte(signingKey), []byte(encryptionKey)), nil
}

// getOrCreateKey reads a key from an environment variable or creates a new one if it doesn't exist
func getOrCreateKey(encryptionService encryption.EncryptionService, envVarName string) (string, error) {
	// Check if the environment variable exists
	key := os.Getenv(envVarName)
	if key != "" {
		return key, nil
	}

	// Generate a new secure key
	key, err := encryptionService.GenerateKey()
	if err != nil {
		return "", err
	}

	// Set the environment variable for future use
	err = os.Setenv(envVarName, key)
	if err != nil {
		return "", err
	}

	return key, nil
}
