package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
)

const wrappingKeyCookieName = "frozenfortress_swk"

// CookieWrappingKeyStore implements WrappingKeyStore by keeping the session wrapping key
// in its own HttpOnly cookie. The key never reaches the server-side session store, so
// neither the session store (Redis) nor the cookie alone can unwrap the MEK.
type CookieWrappingKeyStore struct {
	encryptionService encryption.EncryptionService
	maxAge            int
	logger            ccc.Logger
}

// NewCookieWrappingKeyStore creates a new CookieWrappingKeyStore whose cookies expire
// after maxAge seconds. maxAge should match the session's max age.
func NewCookieWrappingKeyStore(encryptionService encryption.EncryptionService, maxAge int, logger ccc.Logger) *CookieWrappingKeyStore {
	if logger == nil {
		logger = ccc.NopLogger
	}

	return &CookieWrappingKeyStore{
		encryptionService: encryptionService,
		maxAge:            maxAge,
		logger:            logger,
	}
}

// Issue generates a new wrapping key and sets it as a cookie on w.
func (s *CookieWrappingKeyStore) Issue(w http.ResponseWriter, r *http.Request) (string, error) {
	key, err := s.encryptionService.GenerateKey()
	if err != nil {
		s.logger.Error("Failed to generate session wrapping key", "error", err)
		return "", ccc.NewInternalError("failed to generate session wrapping key", err)
	}

	http.SetCookie(w, s.newCookie(r, key, s.maxAge))

	s.logger.Debug("Session wrapping key issued")
	return key, nil
}

// Retrieve returns the wrapping key cookie sent with r, or "" if there is none.
func (s *CookieWrappingKeyStore) Retrieve(r *http.Request) (string, error) {
	cookie, err := r.Cookie(wrappingKeyCookieName)
	if errors.Is(err, http.ErrNoCookie) {
		s.logger.Debug("No session wrapping key cookie in request")
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return cookie.Value, nil
}

// Delete expires the wrapping key cookie on the client.
func (s *CookieWrappingKeyStore) Delete(w http.ResponseWriter, r *http.Request) error {
	http.SetCookie(w, s.newCookie(r, "", -1))

	s.logger.Debug("Session wrapping key cookie deleted")
	return nil
}

func (s *CookieWrappingKeyStore) newCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     wrappingKeyCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteStrictMode,
	}
}

// isSecureRequest reports whether the client reached us over HTTPS, either directly or
// through a TLS-terminating reverse proxy (the Docker setup's nginx). The web UI itself
// serves plain HTTP, so a Secure cookie can't be set unconditionally without breaking
// binary setups that are accessed over plain HTTP.
func isSecureRequest(r *http.Request) bool {
	if r == nil {
		return false
	}

	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
