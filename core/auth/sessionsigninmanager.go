package auth

import (
	"net/http"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
	"github.com/boj/redistore"
	"github.com/gorilla/sessions"
)

const sessionName = "frozenfortress_session"

// mekSessionKey holds the MEK wrapped with the session wrapping key (see WrappingKeyStore).
const mekSessionKey = "ffmekw"

// legacyMekSessionKey held the plaintext MEK in older versions. It is never read, so
// sessions from before the update count as signed out.
const legacyMekSessionKey = "ffmek"

// SessionSignInManager implements SignInManager using gorilla sessions
// and delegates core sign-in logic to a SignInHandler.
type SessionSignInManager struct {
	userRepository UserRepository
	signInHandler  SignInHandler
	sessionStore   sessions.Store
	mekStore       MekStore
	logger         ccc.Logger
}

// NewSessionSignInManager creates a new SessionSignInManager with all dependencies injected
func NewSessionSignInManager(
	userRepo UserRepository,
	signInHandler SignInHandler,
	store sessions.Store,
	mekStore MekStore,
	logger ccc.Logger) *SessionSignInManager {

	if logger == nil {
		logger = ccc.NopLogger
	}

	return &SessionSignInManager{
		userRepository: userRepo,
		signInHandler:  signInHandler,
		sessionStore:   store,
		mekStore:       mekStore,
		logger:         logger,
	}
}

// SignIn verifies the user's credentials using SignInHandler and creates a session if successful.
func (m *SessionSignInManager) SignIn(w http.ResponseWriter, r *http.Request, request SignInRequest) (SignInResponse, error) {
	m.logger.Info("Processing web sign-in request", "username", request.UserName)

	// Create SignInContext with web client information
	context := SignInContext{
		ClientType: ClientTypeWeb,
	}

	// Extract IP address and user agent from request if available
	if r != nil {
		context.IPAddress = r.RemoteAddr
		context.UserAgent = r.UserAgent()
		m.logger.Debug("Extracted web context for sign-in", "ip_address", context.IPAddress, "user_agent", context.UserAgent)
	}

	// Use SignInHandler to perform core authentication logic
	m.logger.Debug("Delegating to sign-in handler for authentication", "username", request.UserName)
	result, err := m.signInHandler.HandleSignIn(request, context)
	if err != nil {
		// SignInHandler only returns errors for genuine internal/system issues
		m.logger.Error("Sign-in handler returned internal error", "username", request.UserName, "error", err)
		return SignInResponse{Success: false, Error: "Internal error"}, err
	}

	// If authentication failed, return the result without an error
	// Authentication failures (invalid credentials, locked accounts, etc.) are not errors
	// but simply unsuccessful responses
	if !result.Success {
		m.logger.Debug("Authentication failed via sign-in handler", "username", request.UserName, "error", result.ErrorMessage)
		return SignInResponse{Success: false, Error: result.ErrorMessage}, nil
	}

	m.logger.Info("Authentication successful, creating web session", "username", request.UserName, "user_id", result.User.Id)

	// Authentication succeeded - create session
	session, err := m.sessionStore.Get(r, sessionName)
	if err != nil {
		m.logger.Error("Failed to get session from store", "username", request.UserName, "user_id", result.User.Id, "error", err)
		return SignInResponse{Success: false, Error: "Internal error"}, ccc.NewInternalError("failed to get session", err)
	}

	session.Values["userId"] = result.User.Id
	err = session.Save(r, w)
	if err != nil {
		m.logger.Error("Failed to save session", "username", request.UserName, "user_id", result.User.Id, "error", err)
		return SignInResponse{Success: false, Error: "Internal error"}, ccc.NewInternalError("failed to save session", err)
	}

	m.logger.Debug("Session created and saved successfully", "username", request.UserName, "user_id", result.User.Id)

	// Store the MEK in the session
	err = m.mekStore.Store(w, r, result.Mek)
	if err != nil {
		m.logger.Error("Failed to store MEK in session", "username", request.UserName, "user_id", result.User.Id, "error", err)
		return SignInResponse{Success: false, Error: "Internal error"}, ccc.NewInternalError("failed to store MEK", err)
	}

	m.logger.Info("Web sign-in completed successfully", "username", request.UserName, "user_id", result.User.Id)

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
	m.logger.Info("Processing sign-out request")

	session, err := m.sessionStore.Get(r, sessionName)
	if err != nil {
		m.logger.Error("Failed to get session for sign-out", "error", err)
		return ccc.NewInternalError("failed to get session for sign out", err)
	}

	// Get user ID for logging before clearing session
	var userId string
	if userIdValue, ok := session.Values["userId"]; ok && userIdValue != nil {
		userId = userIdValue.(string)
		m.logger.Debug("Found user ID in session for sign-out", "user_id", userId)
	}

	// Make sure the MEK is deleted from the session (ignore error)
	mekErr := m.mekStore.Delete(w, r)
	if mekErr != nil {
		m.logger.Warn("Failed to delete MEK from session during sign-out", "user_id", userId, "error", mekErr)
	} else {
		m.logger.Debug("MEK deleted from session successfully", "user_id", userId)
	}

	session.Options.MaxAge = -1 // Mark session for deletion
	err = session.Save(r, w)
	if err != nil {
		m.logger.Error("Failed to save session for sign-out", "user_id", userId, "error", err)
		return ccc.NewInternalError("failed to save session for sign out", err)
	}

	m.logger.Info("Sign-out completed successfully", "user_id", userId)
	return nil
}

// GetCurrentUser retrieves the currently signed in user, if any
func (m *SessionSignInManager) GetCurrentUser(r *http.Request) (UserDto, error) {
	m.logger.Debug("Retrieving current user from session")

	session, err := m.sessionStore.Get(r, sessionName)
	if err != nil {
		m.logger.Error("Failed to get session for current user lookup", "error", err)
		return UserDto{}, ccc.NewInternalError("failed to get session", err)
	}

	userId, ok := session.Values["userId"]
	if !ok || userId == nil {
		m.logger.Debug("No user ID found in session")
		return UserDto{}, nil // No user is signed in
	}

	userIdStr := userId.(string)

	// A session without a usable MEK can't decrypt anything, e.g. because the wrapping
	// key cookie is gone or the session predates MEK wrapping. Treat it as signed out.
	mek, err := m.mekStore.Retrieve(r)
	if err != nil {
		m.logger.Error("Failed to retrieve MEK for current user lookup", "user_id", userIdStr, "error", err)
		return UserDto{}, ccc.NewInternalError("failed to retrieve MEK", err)
	}
	if mek == "" {
		m.logger.Debug("Session has no usable MEK, treating as signed out", "user_id", userIdStr)
		return UserDto{}, nil
	}

	m.logger.Debug("Found user ID in session, looking up user", "user_id", userIdStr)

	user, err := m.userRepository.FindById(userIdStr)
	if err != nil {
		m.logger.Error("Failed to find user by ID from session", "user_id", userIdStr, "error", err)
		return UserDto{}, ccc.NewDatabaseError("find user by ID", err)
	}

	// If user is no longer valid
	if user == nil || user.Id == "" {
		m.logger.Warn("User ID from session not found in database", "user_id", userIdStr)
		return UserDto{}, nil
	}

	m.logger.Debug("Successfully retrieved current user", "user_id", user.Id, "username", user.UserName)

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
	m.logger.Debug("Checking if user is signed in")

	user, err := m.GetCurrentUser(r)
	if err != nil {
		m.logger.Error("Failed to check sign-in status", "error", err)
		return false, err
	}

	isSignedIn := user.Id != ""
	m.logger.Debug("Sign-in status checked", "is_signed_in", isSignedIn, "user_id", user.Id)

	return isSignedIn, nil
}

// RecoverySignIn verifies the user's recovery code and new password using SignInHandler and creates a session if successful.
func (m *SessionSignInManager) RecoverySignIn(w http.ResponseWriter, r *http.Request, request RecoverySignInRequest) (RecoverySignInResponse, error) {
	m.logger.Info("Processing web recovery sign-in request", "username", request.UserName)

	// Create SignInContext with web client information
	context := SignInContext{
		ClientType: ClientTypeWeb,
	}

	// Extract IP address and user agent from request if available
	if r != nil {
		context.IPAddress = r.RemoteAddr
		context.UserAgent = r.UserAgent()
		m.logger.Debug("Extracted web context for recovery sign-in", "ip_address", context.IPAddress, "user_agent", context.UserAgent)
	}

	// Use SignInHandler to perform core recovery authentication logic
	m.logger.Debug("Delegating to sign-in handler for recovery authentication", "username", request.UserName)
	result, err := m.signInHandler.HandleRecoverySignIn(request, context)
	if err != nil {
		// SignInHandler only returns errors for genuine internal/system issues
		m.logger.Error("Recovery sign-in handler returned internal error", "username", request.UserName, "error", err)
		return RecoverySignInResponse{Success: false, NewRecoveryCode: "", Error: "Internal error"}, err
	}

	// If recovery authentication failed, return the result without an error
	if !result.Success {
		m.logger.Debug("Recovery authentication failed via sign-in handler", "username", request.UserName, "error", result.ErrorMessage)
		return RecoverySignInResponse{Success: false, NewRecoveryCode: "", Error: result.ErrorMessage}, nil
	}

	m.logger.Info("Recovery authentication successful, creating web session", "username", request.UserName, "user_id", result.User.Id)

	// Recovery authentication succeeded - create session
	session, err := m.sessionStore.Get(r, sessionName)
	if err != nil {
		m.logger.Error("Failed to get session from store", "username", request.UserName, "user_id", result.User.Id, "error", err)
		return RecoverySignInResponse{Success: false, NewRecoveryCode: "", Error: "Internal error"}, ccc.NewInternalError("failed to get session", err)
	}

	session.Values["userId"] = result.User.Id
	err = session.Save(r, w)
	if err != nil {
		m.logger.Error("Failed to save session", "username", request.UserName, "user_id", result.User.Id, "error", err)
		return RecoverySignInResponse{Success: false, NewRecoveryCode: "", Error: "Internal error"}, ccc.NewInternalError("failed to save session", err)
	}

	m.logger.Debug("Session created and saved successfully", "username", request.UserName, "user_id", result.User.Id)

	// Store the MEK in the session
	err = m.mekStore.Store(w, r, result.Mek)
	if err != nil {
		m.logger.Error("Failed to store MEK in session", "username", request.UserName, "user_id", result.User.Id, "error", err)
		return RecoverySignInResponse{Success: false, NewRecoveryCode: "", Error: "Internal error"}, ccc.NewInternalError("failed to store MEK", err)
	}

	m.logger.Info("Web recovery sign-in completed successfully", "username", request.UserName, "user_id", result.User.Id)

	return RecoverySignInResponse{
		Success: true,
		User: UserDto{
			Id:         result.User.Id,
			UserName:   result.User.UserName,
			IsActive:   result.User.IsActive,
			IsLocked:   result.User.IsLocked,
			CreatedAt:  result.User.CreatedAt.Format(time.RFC3339),
			ModifiedAt: result.User.ModifiedAt.Format(time.RFC3339),
		},
		NewRecoveryCode: result.NewRecoveryCode,
	}, nil
}

// CreateRedisStore creates a new Redis store for session management.
// It uses the provided AppConfig for Redis connection details and the
// SessionKeyProvider to obtain signing and encryption keys.
//
// Configuration fields from AppConfig used:
// - RedisAddress: Address of the Redis server (e.g., "localhost:6379")
// - RedisUser: Username for the Redis server (empty if none)
// - RedisPassword: Password for the Redis server (empty if none)
// - RedisSize: Maximum number of idle connections in the pool (e.g., 10)
// - RedisNetwork: Network type, "tcp" or "unix" (e.g., "tcp")
// - SessionMaxAgeDays: How long a session lasts (e.g., 30)
//
// The SessionKeyProvider is responsible for the logic of obtaining,
// generating, and persisting the session keys.
func CreateRedisStore(config ccc.AppConfig, keyProvider SessionKeyProvider, logger ccc.Logger) (sessions.Store, error) {
	if logger == nil {
		logger = ccc.NopLogger
	}

	logger.Info("Creating Redis session store", "redis_address", config.RedisAddress, "redis_network", config.RedisNetwork, "pool_size", config.RedisSize)

	signingKeyBytes, err := keyProvider.GetSigningKey()
	if err != nil {
		logger.Error("Failed to get signing key for Redis store", "error", err)
		return nil, err
	}

	encryptionKeyBytes, err := keyProvider.GetEncryptionKey()
	if err != nil {
		logger.Error("Failed to get encryption key for Redis store", "error", err)
		return nil, err
	}

	logger.Debug("Retrieved session keys successfully")

	store, err := redistore.NewRediStore(config.RedisSize, config.RedisNetwork, config.RedisAddress, config.RedisUser, config.RedisPassword, signingKeyBytes, encryptionKeyBytes)
	if err != nil {
		logger.Error("Failed to create Redis store", "error", err, "redis_address", config.RedisAddress)
		return nil, err
	}

	store.SetMaxAge(SessionMaxAgeSeconds(config))
	store.SetMaxLength(4096) // 4KB

	logger.Info("Redis session store created successfully", "max_age_days", config.SessionMaxAgeDays, "max_length_bytes", 4096)

	return store, nil
}

// SessionMaxAgeSeconds returns the configured session lifetime in seconds. Session and
// wrapping key cookies share it.
func SessionMaxAgeSeconds(config ccc.AppConfig) int {
	return config.SessionMaxAgeDays * 24 * 60 * 60
}

type SessionMekStore struct {
	sessionStore      sessions.Store
	wrappingKeyStore  WrappingKeyStore
	encryptionService encryption.EncryptionService
	logger            ccc.Logger
}

// NewSessionMekStore creates a SessionMekStore that keeps the MEK in the session store
// only in wrapped (encrypted) form. The key that unwraps it lives in wrappingKeyStore.
func NewSessionMekStore(sessionStore sessions.Store, wrappingKeyStore WrappingKeyStore, encryptionService encryption.EncryptionService, logger ccc.Logger) *SessionMekStore {
	if logger == nil {
		logger = ccc.NopLogger
	}

	return &SessionMekStore{
		sessionStore:      sessionStore,
		wrappingKeyStore:  wrappingKeyStore,
		encryptionService: encryptionService,
		logger:            logger,
	}
}

// Retrieve reads the wrapped MEK (Master Encryption Key) from the session store and
// unwraps it with the request's wrapping key. It returns "" if either part is missing
// or the MEK can't be unwrapped, which callers treat as "not signed in".
func (s *SessionMekStore) Retrieve(r *http.Request) (string, error) {
	s.logger.Debug("Retrieving MEK from session store")

	session, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		s.logger.Error("Failed to get session for MEK retrieval", "error", err)
		return "", err
	}

	wrappedMek, ok := session.Values[mekSessionKey].(string)
	if !ok || wrappedMek == "" {
		s.logger.Debug("No wrapped MEK found in session")
		return "", nil
	}

	wrappingKey, err := s.wrappingKeyStore.Retrieve(r)
	if err != nil {
		s.logger.Error("Failed to get session wrapping key for MEK retrieval", "error", err)
		return "", err
	}
	if wrappingKey == "" {
		s.logger.Debug("No session wrapping key found for MEK")
		return "", nil
	}

	mek, err := s.encryptionService.Decrypt(wrappedMek, wrappingKey)
	if err != nil {
		s.logger.Warn("Failed to unwrap MEK with session wrapping key", "error", err)
		return "", nil
	}

	s.logger.Debug("MEK retrieved from session successfully")
	return mek, nil
}

// Store wraps the MEK (Master Encryption Key) with a newly issued wrapping key and
// saves only the wrapped MEK in the session store
func (s *SessionMekStore) Store(w http.ResponseWriter, r *http.Request, mek string) error {
	s.logger.Debug("Storing MEK in session store")

	session, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		s.logger.Error("Failed to get session for MEK storage", "error", err)
		return err
	}

	wrappingKey, err := s.wrappingKeyStore.Issue(w, r)
	if err != nil {
		s.logger.Error("Failed to issue session wrapping key for MEK storage", "error", err)
		return err
	}

	wrappedMek, err := s.encryptionService.Encrypt(mek, wrappingKey)
	if err != nil {
		s.logger.Error("Failed to wrap MEK with session wrapping key", "error", err)
		return err
	}

	// Drop a plaintext MEK left behind by an older version
	delete(session.Values, legacyMekSessionKey)
	session.Values[mekSessionKey] = wrappedMek
	err = s.sessionStore.Save(r, w, session)
	if err != nil {
		s.logger.Error("Failed to save session with MEK", "error", err)
		return err
	}

	s.logger.Debug("MEK stored in session successfully")
	return nil
}

// Delete removes the MEK (Master Encryption Key) from the session store and discards
// the wrapping key
func (s *SessionMekStore) Delete(w http.ResponseWriter, r *http.Request) error {
	s.logger.Debug("Deleting MEK from session store")

	if err := s.wrappingKeyStore.Delete(w, r); err != nil {
		s.logger.Error("Failed to delete session wrapping key", "error", err)
		return err
	}

	session, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		s.logger.Error("Failed to get session for MEK deletion", "error", err)
		return err
	}

	delete(session.Values, mekSessionKey)
	delete(session.Values, legacyMekSessionKey)
	err = s.sessionStore.Save(r, w, session)
	if err != nil {
		s.logger.Error("Failed to save session after MEK deletion", "error", err)
		return err
	}

	s.logger.Debug("MEK deleted from session successfully")
	return nil
}
