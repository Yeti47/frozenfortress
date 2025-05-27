package auth

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
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
// and delegates core sign-in logic to a SignInHandler.
type SessionSignInManager struct {
	userRepository UserRepository
	signInHandler  SignInHandler
	sessionStore   sessions.Store
	mekStore       MekStore
}

// NewSessionSignInManager creates a new SessionSignInManager with all dependencies injected
func NewSessionSignInManager(
	userRepo UserRepository,
	signInHandler SignInHandler,
	store sessions.Store,
	mekStore MekStore) *SessionSignInManager {

	return &SessionSignInManager{
		userRepository: userRepo,
		signInHandler:  signInHandler,
		sessionStore:   store,
		mekStore:       mekStore,
	}
}

// SignIn verifies the user's credentials using SignInHandler and creates a session if successful.
func (m *SessionSignInManager) SignIn(w http.ResponseWriter, r *http.Request, request SignInRequest) (SignInResponse, error) {
	// Create SignInContext with web client information
	context := SignInContext{
		ClientType: ClientTypeWeb,
	}

	// Extract IP address and user agent from request if available
	if r != nil {
		context.IPAddress = r.RemoteAddr
		context.UserAgent = r.UserAgent()
	}

	// Use SignInHandler to perform core authentication logic
	result, err := m.signInHandler.HandleSignIn(request, context)
	if err != nil {
		return SignInResponse{Success: false, Error: result.ErrorMessage}, err
	}

	// If authentication failed, return the result
	if !result.Success {
		return SignInResponse{Success: false, Error: result.ErrorMessage}, result.ErrorCode
	}

	// Authentication succeeded - create session
	session, err := m.sessionStore.Get(r, sessionName)
	if err != nil {
		return SignInResponse{Success: false, Error: "Internal error"}, ccc.NewInternalError("failed to get session", err)
	}

	session.Values["userId"] = result.User.Id
	err = session.Save(r, w)
	if err != nil {
		return SignInResponse{Success: false, Error: "Internal error"}, ccc.NewInternalError("failed to save session", err)
	}

	// Store the MEK in the session
	err = m.mekStore.Store(w, r, result.Mek)
	if err != nil {
		return SignInResponse{Success: false, Error: "Internal error"}, ccc.NewInternalError("failed to store MEK", err)
	}

	return SignInResponse{
		Success: true,
		User: UserDto{
			Id:         result.User.Id,
			UserName:   result.User.UserName,
			IsActive:   result.User.IsActive,
			IsLocked:   result.User.IsLocked,
			CreatedAt:  result.User.CreatedAt.Format(time.RFC3339),
			ModifiedAt: result.User.ModifiedAt.Format(time.RFC3339),
		},
	}, nil
}

// SignOut clears the session for the user.
func (m *SessionSignInManager) SignOut(w http.ResponseWriter, r *http.Request) error {
	session, err := m.sessionStore.Get(r, sessionName)
	if err != nil {
		return ccc.NewInternalError("failed to get session for sign out", err)
	}

	// Make sure the MEK is deleted from the session (ignore error)
	_ = m.mekStore.Delete(w, r)

	session.Options.MaxAge = -1 // Mark session for deletion
	err = session.Save(r, w)
	if err != nil {
		return ccc.NewInternalError("failed to save session for sign out", err)
	}
	return nil
}

// GetCurrentUser retrieves the currently signed in user, if any
func (m *SessionSignInManager) GetCurrentUser(r *http.Request) (UserDto, error) {
	session, err := m.sessionStore.Get(r, sessionName)
	if err != nil {
		return UserDto{}, ccc.NewInternalError("failed to get session", err)
	}

	userId, ok := session.Values["userId"]
	if !ok || userId == nil {
		return UserDto{}, nil // No user is signed in
	}

	user, err := m.userRepository.FindById(userId.(string))
	if err != nil {
		return UserDto{}, ccc.NewDatabaseError("find user by ID", err)
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
