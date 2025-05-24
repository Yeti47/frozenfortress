package auth

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
	"github.com/boj/redistore"
	"github.com/gorilla/sessions"
)

// signInConfig holds configuration values for the sign-in process
type signInConfig struct {
	MaxFailedAttempts    int // Maximum number of failed login attempts before locking account
	FailedAttemptsWindow int // Time window in minutes for counting failed attempts
}

// Default configuration values if environment variables aren't set
var defaultConfig = signInConfig{
	MaxFailedAttempts:    3,
	FailedAttemptsWindow: 30,
}

const sessionName = "frozenfortress_session"
const mekSessionKey = "ffmek"

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

	return config
}

// SessionSignInManager implements SignInManager using gorilla sessions
// and verifies user credentials using a UserManager.
type SessionSignInManager struct {
	userRepository          UserRepository
	signInHistoryRepository SignInHistoryItemRepository
	sessionStore            sessions.Store
	securityService         SecurityService
	mekStore                MekStore
	config                  signInConfig
}

// NewSessionSignInManager creates a new SessionSignInManager with all dependencies injected
func NewSessionSignInManager(
	userRepo UserRepository,
	signInHistoryRepo SignInHistoryItemRepository,
	store sessions.Store,
	securityService SecurityService,
	mekStore MekStore,
	config signInConfig) *SessionSignInManager {

	return &SessionSignInManager{
		userRepository:          userRepo,
		signInHistoryRepository: signInHistoryRepo,
		sessionStore:            store,
		securityService:         securityService,
		mekStore:                mekStore,
		config:                  config,
	}
}

// SignIn verifies the user's credentials and creates a session if successful.
// Also tracks login attempts and locks accounts after too many failed attempts.
func (m *SessionSignInManager) SignIn(w http.ResponseWriter, r *http.Request, request SignInRequest) (SignInResponse, error) {

	user, err := m.userRepository.FindByUserName(request.UserName)
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
	if user == nil || user.Id == "" {
		// Even for non-existent users, we still log the attempt
		historyItem.DenialReason = "Invalid credentials"
		_ = m.signInHistoryRepository.Add(historyItem)
		return SignInResponse{Success: false, Error: "Invalid credentials"}, nil
	}

	// Set user ID now that we have it
	historyItem.UserId = user.Id

	// Check if account is locked
	if user.IsLocked {
		historyItem.DenialReason = "Account is locked"
		_ = m.signInHistoryRepository.Add(historyItem)
		return SignInResponse{Success: false, Error: "Account is locked"}, nil
	}

	// Check if account is inactive
	if !user.IsActive {
		historyItem.DenialReason = "Account is inactive"
		_ = m.signInHistoryRepository.Add(historyItem)
		return SignInResponse{Success: false, Error: "Account is inactive"}, nil
	}

	// Verify password using SecurityService
	valid, err := m.securityService.VerifyUserPassword(*user, request.Password)
	if err != nil {
		historyItem.DenialReason = "Password verification failed"
		_ = m.signInHistoryRepository.Add(historyItem)
		return SignInResponse{Success: false, Error: "Password verification failed"}, err
	}

	// If password verification failed
	if !valid {
		// Log the failed attempt
		historyItem.DenialReason = "Invalid credentials"
		_ = m.signInHistoryRepository.Add(historyItem)

		// Check for too many failed attempts, but only if the account is active
		if user.IsActive {
			failedAttempts, err := m.signInHistoryRepository.GetRecentFailedSignInsByUserName(
				user.UserName, m.config.FailedAttemptsWindow)

			if err == nil && len(failedAttempts) >= m.config.MaxFailedAttempts {
				// Lock the account using the SecurityService
				_, _ = m.securityService.LockUser(*user)
				return SignInResponse{Success: false, Error: "Account has been locked due to too many failed attempts"}, nil
			}
		}
		return SignInResponse{Success: false, Error: "Invalid credentials"}, nil
	}

	// Authentication succeeded - create session
	session, err := m.sessionStore.Get(r, sessionName)
	if err != nil {
		_ = m.signInHistoryRepository.Add(historyItem)
		return SignInResponse{Success: false, Error: "Session error"}, err
	}

	mek, err := m.securityService.UncoverMek(*user, request.Password)
	if err != nil {
		_ = m.signInHistoryRepository.Add(historyItem)
		return SignInResponse{Success: false, Error: "MEK uncovering error"}, err
	}
	if mek == "" {
		_ = m.signInHistoryRepository.Add(historyItem)
		return SignInResponse{Success: false, Error: "MEK uncovering failed"}, nil
	}

	session.Values["userId"] = user.Id
	err = session.Save(r, w)
	if err != nil {
		_ = m.signInHistoryRepository.Add(historyItem)
		return SignInResponse{Success: false, Error: "Session save error"}, err
	}

	// Store the MEK in the session
	err = m.mekStore.Store(w, r, mek)
	if err != nil {
		_ = m.signInHistoryRepository.Add(historyItem)
		return SignInResponse{Success: false, Error: "MEK store error"}, err
	}

	// Update history item to reflect successful login
	historyItem.Successful = true
	_ = m.signInHistoryRepository.Add(historyItem)

	return SignInResponse{
		Success: true,
		User: UserDto{
			Id:         user.Id,
			UserName:   user.UserName,
			IsActive:   user.IsActive,
			IsLocked:   user.IsLocked,
			CreatedAt:  user.CreatedAt.Format(time.RFC3339),
			ModifiedAt: user.ModifiedAt.Format(time.RFC3339),
		},
	}, nil
}

// SignOut clears the session for the user.
func (m *SessionSignInManager) SignOut(w http.ResponseWriter, r *http.Request) error {
	session, err := m.sessionStore.Get(r, sessionName)
	if err != nil {
		return err
	}

	// Make sure the MEK is deleted from the session (ignore error)
	_ = m.mekStore.Delete(w, r)

	session.Options.MaxAge = -1 // Mark session for deletion
	return session.Save(r, w)
}

// GetCurrentUser retrieves the currently signed in user, if any
func (m *SessionSignInManager) GetCurrentUser(r *http.Request) (UserDto, error) {
	session, err := m.sessionStore.Get(r, sessionName)
	if err != nil {
		return UserDto{}, err
	}

	userId, ok := session.Values["userId"]
	if !ok || userId == nil {
		return UserDto{}, nil // No user is signed in
	}

	user, err := m.userRepository.FindById(userId.(string))
	if err != nil {
		return UserDto{}, err
	}

	// If user is no longer valid
	if user == nil || user.Id == "" {
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

// IsSignedIn checks if a user is currently signed in
func (m *SessionSignInManager) IsSignedIn(r *http.Request) (bool, error) {
	user, err := m.GetCurrentUser(r)
	if err != nil {
		return false, err
	}
	return user.Id != "", nil
}

// CreateRedisStore creates a new Redis store for session management.
// It reads Redis connection details and signing/encryption keys from environment variables.
// Environment variables for Redis:
// - FROZEN_FORTRESS_REDIS_ADDRESS: Address of the Redis server (e.g., "localhost:6379")
// - FROZEN_FORTESS_RESIS_USER: Username for the Redis server (empty if none)
// - FROZEN_FORTRESS_REDIS_PASSWORD: Password for the Redis server (empty if none)
// - FROZEN_FORTRESS_REDIS_REDIS_SIZE: Maximum number of idle connections in the pool (e.g., 10)
// - FROZEN_FORTRESS_REDIS_NETWORK: Network type, "tcp" or "unix" (e.g., "tcp")
// Environment variables for keys (reused from cookie store):
// - FROZEN_FORTRESS_SIGNING_KEY: Key for session authentication
// - FROZEN_FORTRESS_ENCRYPTION_KEY: Key for session encryption
func CreateRedisStore(encryptionService encryption.EncryptionService) (sessions.Store, error) {
	// Redis connection details from environment variables
	redisAddress := os.Getenv("FROZEN_FORTRESS_REDIS_ADDRESS")
	if redisAddress == "" {
		redisAddress = "localhost:6379" // Default address
	}

	redisUser := os.Getenv("FROZEN_FORTRESS_REDIS_USER")

	redisPassword := os.Getenv("FROZEN_FORTRESS_REDIS_PASSWORD") // Default is empty string

	redisSizeString := os.Getenv("FROZEN_FORTRESS_REDIS_SIZE")
	redisSize, err := strconv.Atoi(redisSizeString)
	if err != nil || redisSize <= 0 {
		redisSize = 10 // Default size
	}

	redisNetwork := os.Getenv("FROZEN_FORTRESS_REDIS_NETWORK")
	if redisNetwork == "" {
		redisNetwork = "tcp" // Default network type
	}

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

	signingKeyBytes, err := encryptionService.ConvertStringToKey(signingKey)
	if err != nil {
		return nil, err
	}

	encryptionKeyBytes, err := encryptionService.ConvertStringToKey(encryptionKey)
	if err != nil {
		return nil, err
	}

	// Create Redis store
	store, err := redistore.NewRediStore(redisSize, redisNetwork, redisAddress, redisUser, redisPassword, signingKeyBytes, encryptionKeyBytes)
	if err != nil {
		return nil, err
	}
	return store, nil
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

type SessionMekStore struct {
	sessionStore sessions.Store
}

func NewSessionMekStore(sessionStore sessions.Store) *SessionMekStore {
	return &SessionMekStore{
		sessionStore: sessionStore,
	}
}

// Retrieve reads the MEK (Master Encryption Key) from the session store
func (s *SessionMekStore) Retrieve(r *http.Request) (string, error) {

	session, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		return "", err
	}

	mek, ok := session.Values[mekSessionKey]
	if !ok || mek == nil {
		return "", nil // No MEK found in session
	}

	return mek.(string), nil
}

// Store saves the MEK (Master Encryption Key) in the session store
func (s *SessionMekStore) Store(w http.ResponseWriter, r *http.Request, mek string) error {

	session, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		return err
	}

	session.Values[mekSessionKey] = mek
	return s.sessionStore.Save(r, w, session)
}

// Delete removes the MEK (Master Encryption Key) from the session store
func (s *SessionMekStore) Delete(w http.ResponseWriter, r *http.Request) error {

	session, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		return err
	}

	delete(session.Values, mekSessionKey)
	return s.sessionStore.Save(r, w, session)
}
