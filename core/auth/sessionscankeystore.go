package auth

import (
	"net/http"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/gorilla/sessions"
)

// scanKeyTTL is how long a stored scan handoff key stays valid, deliberately much
// tighter than the session's own 30-day MaxAge (see CreateRedisStore) - these keys are
// only ever needed for a few minutes during a single scan handoff.
const scanKeyTTL = 5 * time.Minute

func scanKeySessionKey(token string) string {
	return "ffscankey:" + token
}

func scanKeyTimestampSessionKey(token string) string {
	return "ffscankeyts:" + token
}

// SessionScanKeyStore stores the transient scan handoff encryption key in the same
// Redis-backed session that already holds the MEK (see SessionMekStore). Reusing that
// session store doesn't introduce a new class of exposure: anyone with enough Redis
// access to read arbitrary session values already holds the MEK and can decrypt any
// document, so a temporary scan key sitting alongside it under a much shorter expiry
// is consistent with the app's existing trust boundary. It also makes the key
// retrievable from any browser tab of the same session, which matters because the
// companion app's "return to Frozen Fortress" step may reopen the page in a new tab
// rather than resuming the original one.
type SessionScanKeyStore struct {
	sessionStore sessions.Store
	logger       ccc.Logger
}

// NewSessionScanKeyStore creates a new SessionScanKeyStore.
func NewSessionScanKeyStore(sessionStore sessions.Store, logger ccc.Logger) *SessionScanKeyStore {
	if logger == nil {
		logger = ccc.NopLogger
	}

	return &SessionScanKeyStore{
		sessionStore: sessionStore,
		logger:       logger,
	}
}

// Store saves key for token in the session, timestamped so it can be expired after scanKeyTTL.
func (s *SessionScanKeyStore) Store(w http.ResponseWriter, r *http.Request, token, key string) error {
	s.logger.Debug("Storing scan key in session store", "token", token)

	session, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		s.logger.Error("Failed to get session for scan key storage", "token", token, "error", err)
		return err
	}

	session.Values[scanKeySessionKey(token)] = key
	session.Values[scanKeyTimestampSessionKey(token)] = time.Now().UTC().Format(time.RFC3339)

	if err := s.sessionStore.Save(r, w, session); err != nil {
		s.logger.Error("Failed to save session with scan key", "token", token, "error", err)
		return err
	}

	s.logger.Debug("Scan key stored in session successfully", "token", token)
	return nil
}

// Retrieve reads the scan key for token from the session. It returns an empty string
// (no error) if no key is stored for token, or if the stored entry is older than
// scanKeyTTL - in the latter case the stale entry is opportunistically cleared.
func (s *SessionScanKeyStore) Retrieve(w http.ResponseWriter, r *http.Request, token string) (string, error) {
	s.logger.Debug("Retrieving scan key from session store", "token", token)

	session, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		s.logger.Error("Failed to get session for scan key retrieval", "token", token, "error", err)
		return "", err
	}

	keyRaw, ok := session.Values[scanKeySessionKey(token)]
	if !ok || keyRaw == nil {
		s.logger.Debug("No scan key found in session", "token", token)
		return "", nil
	}

	if s.isExpired(session, token) {
		s.logger.Debug("Scan key expired; clearing stale entry", "token", token)
		s.clearEntry(session, token)
		if err := s.sessionStore.Save(r, w, session); err != nil {
			// The key is treated as absent either way; a cleanup failure just means the
			// stale entry lingers until the next Retrieve or Delete call for this token.
			s.logger.Error("Failed to save session after clearing expired scan key", "token", token, "error", err)
		}
		return "", nil
	}

	key, ok := keyRaw.(string)
	if !ok {
		s.logger.Warn("Scan key in session has unexpected type; treating as absent", "token", token)
		return "", nil
	}

	s.logger.Debug("Scan key retrieved from session successfully", "token", token)
	return key, nil
}

// Delete removes the scan key entry for token from the session.
func (s *SessionScanKeyStore) Delete(w http.ResponseWriter, r *http.Request, token string) error {
	s.logger.Debug("Deleting scan key from session store", "token", token)

	session, err := s.sessionStore.Get(r, sessionName)
	if err != nil {
		s.logger.Error("Failed to get session for scan key deletion", "token", token, "error", err)
		return err
	}

	s.clearEntry(session, token)

	if err := s.sessionStore.Save(r, w, session); err != nil {
		s.logger.Error("Failed to save session after scan key deletion", "token", token, "error", err)
		return err
	}

	s.logger.Debug("Scan key deleted from session successfully", "token", token)
	return nil
}

// isExpired reports whether the stored entry for token is missing a valid timestamp or
// is older than scanKeyTTL.
func (s *SessionScanKeyStore) isExpired(session *sessions.Session, token string) bool {
	storedAtRaw, ok := session.Values[scanKeyTimestampSessionKey(token)]
	if !ok || storedAtRaw == nil {
		return true
	}

	storedAtStr, ok := storedAtRaw.(string)
	if !ok {
		return true
	}

	storedAt, err := time.Parse(time.RFC3339, storedAtStr)
	if err != nil {
		return true
	}

	return time.Since(storedAt) > scanKeyTTL
}

// clearEntry removes both the key and its timestamp for token from session.Values.
func (s *SessionScanKeyStore) clearEntry(session *sessions.Session, token string) {
	delete(session.Values, scanKeySessionKey(token))
	delete(session.Values, scanKeyTimestampSessionKey(token))
}
