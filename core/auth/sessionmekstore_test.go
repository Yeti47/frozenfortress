package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
	"github.com/gorilla/sessions"
)

// memorySessionStore is a minimal server-side sessions.Store standing in for the Redis
// store: the cookie only carries the session ID, and the values stay in data, where the
// tests can inspect exactly what the server side holds.
type memorySessionStore struct {
	data   map[string]map[interface{}]interface{}
	nextId int
}

func newMemorySessionStore() *memorySessionStore {
	return &memorySessionStore{data: make(map[string]map[interface{}]interface{})}
}

func (s *memorySessionStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

func (s *memorySessionStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(s, name)
	session.Options = &sessions.Options{Path: "/", MaxAge: 3600}
	session.IsNew = true

	cookie, err := r.Cookie(name)
	if err != nil {
		return session, nil
	}

	values, ok := s.data[cookie.Value]
	if !ok {
		return session, nil
	}

	session.ID = cookie.Value
	session.IsNew = false
	for k, v := range values {
		session.Values[k] = v
	}
	return session, nil
}

func (s *memorySessionStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	if session.ID == "" {
		s.nextId++
		session.ID = fmt.Sprintf("session-%d", s.nextId)
	}

	values := make(map[interface{}]interface{}, len(session.Values))
	for k, v := range session.Values {
		values[k] = v
	}
	s.data[session.ID] = values

	http.SetCookie(w, sessions.NewCookie(session.Name(), session.ID, session.Options))
	return nil
}

type mekStoreFixture struct {
	sessionStore     *memorySessionStore
	wrappingKeyStore *CookieWrappingKeyStore
	mekStore         *SessionMekStore
	enc              encryption.EncryptionService
}

func newMekStoreFixture() *mekStoreFixture {
	enc := encryption.NewDefaultEncryptionService()
	sessionStore := newMemorySessionStore()
	wrappingKeyStore := NewCookieWrappingKeyStore(enc, 3600, ccc.NopLogger)

	return &mekStoreFixture{
		sessionStore:     sessionStore,
		wrappingKeyStore: wrappingKeyStore,
		mekStore:         NewSessionMekStore(sessionStore, wrappingKeyStore, enc, ccc.NopLogger),
		enc:              enc,
	}
}

func (f *mekStoreFixture) generateKey(t *testing.T) string {
	t.Helper()

	key, err := f.enc.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	return key
}

// storeMek stores mek as a sign-in would and returns the cookies the browser would
// receive.
func (f *mekStoreFixture) storeMek(t *testing.T, mek string) []*http.Cookie {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	if err := f.mekStore.Store(rec, req, mek); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}
	return rec.Result().Cookies()
}

func requestWithCookies(cookies ...*http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	return req
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func TestSessionMekStore_RoundTrip(t *testing.T) {
	f := newMekStoreFixture()
	mek := f.generateKey(t)

	cookies := f.storeMek(t, mek)

	got, err := f.mekStore.Retrieve(requestWithCookies(cookies...))
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if got != mek {
		t.Fatalf("expected the stored MEK back, got %q", got)
	}
}

func TestSessionMekStore_SessionStoreDoesNotHoldPlaintextMek(t *testing.T) {
	f := newMekStoreFixture()
	mek := f.generateKey(t)

	cookies := f.storeMek(t, mek)

	if len(f.sessionStore.data) != 1 {
		t.Fatalf("expected exactly one stored session, got %d", len(f.sessionStore.data))
	}
	for _, values := range f.sessionStore.data {
		for k, v := range values {
			if strings.Contains(fmt.Sprint(v), mek) {
				t.Fatalf("session value %v contains the plaintext MEK", k)
			}
		}
	}

	// Without the wrapping key cookie, the session store's contents are useless
	sessionCookie := findCookie(cookies, sessionName)
	if sessionCookie == nil {
		t.Fatal("expected a session cookie")
	}
	got, err := f.mekStore.Retrieve(requestWithCookies(sessionCookie))
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if got != "" {
		t.Fatal("expected no MEK without the wrapping key cookie")
	}
}

func TestSessionMekStore_RetrieveWithWrongWrappingKeyReturnsNoMek(t *testing.T) {
	f := newMekStoreFixture()
	mek := f.generateKey(t)

	cookies := f.storeMek(t, mek)
	wrappingKeyCookie := findCookie(cookies, wrappingKeyCookieName)
	if wrappingKeyCookie == nil {
		t.Fatal("expected a wrapping key cookie")
	}
	wrappingKeyCookie.Value = f.generateKey(t)

	got, err := f.mekStore.Retrieve(requestWithCookies(cookies...))
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if got != "" {
		t.Fatal("expected no MEK with a wrong wrapping key")
	}
}

func TestSessionMekStore_WrappingKeyFromOtherSessionReturnsNoMek(t *testing.T) {
	f := newMekStoreFixture()

	cookiesA := f.storeMek(t, f.generateKey(t))
	cookiesB := f.storeMek(t, f.generateKey(t))

	got, err := f.mekStore.Retrieve(requestWithCookies(
		findCookie(cookiesA, sessionName),
		findCookie(cookiesB, wrappingKeyCookieName),
	))
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if got != "" {
		t.Fatal("expected no MEK when combining one session with another session's wrapping key")
	}
}

func TestSessionMekStore_LegacyPlaintextMekIsIgnored(t *testing.T) {
	f := newMekStoreFixture()
	mek := f.generateKey(t)

	f.sessionStore.data["legacy"] = map[interface{}]interface{}{
		"userId":            "user-1",
		legacyMekSessionKey: mek,
	}

	got, err := f.mekStore.Retrieve(requestWithCookies(
		&http.Cookie{Name: sessionName, Value: "legacy"},
		&http.Cookie{Name: wrappingKeyCookieName, Value: f.generateKey(t)},
	))
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if got != "" {
		t.Fatal("expected a legacy plaintext MEK to be ignored")
	}
}

func TestSessionMekStore_StoreRemovesLegacyPlaintextMek(t *testing.T) {
	f := newMekStoreFixture()

	f.sessionStore.data["legacy"] = map[interface{}]interface{}{
		legacyMekSessionKey: f.generateKey(t),
	}

	rec := httptest.NewRecorder()
	req := requestWithCookies(&http.Cookie{Name: sessionName, Value: "legacy"})
	if err := f.mekStore.Store(rec, req, f.generateKey(t)); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}

	if _, ok := f.sessionStore.data["legacy"][legacyMekSessionKey]; ok {
		t.Fatal("expected Store to remove the legacy plaintext MEK")
	}
}

func TestSessionMekStore_DeleteRemovesMekAndExpiresWrappingKey(t *testing.T) {
	f := newMekStoreFixture()
	cookies := f.storeMek(t, f.generateKey(t))

	rec := httptest.NewRecorder()
	if err := f.mekStore.Delete(rec, requestWithCookies(cookies...)); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	sessionId := findCookie(cookies, sessionName).Value
	if _, ok := f.sessionStore.data[sessionId][mekSessionKey]; ok {
		t.Fatal("expected Delete to remove the wrapped MEK from the session")
	}

	expired := findCookie(rec.Result().Cookies(), wrappingKeyCookieName)
	if expired == nil || expired.MaxAge >= 0 {
		t.Fatalf("expected Delete to expire the wrapping key cookie, got %+v", expired)
	}
}

func TestCookieWrappingKeyStore_CookieAttributes(t *testing.T) {
	enc := encryption.NewDefaultEncryptionService()
	store := NewCookieWrappingKeyStore(enc, 1234, ccc.NopLogger)

	tests := []struct {
		name       string
		forwarded  string
		wantSecure bool
	}{
		{name: "behind TLS-terminating proxy", forwarded: "https", wantSecure: true},
		{name: "plain HTTP", forwarded: "", wantSecure: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/login", nil)
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwarded)
			}
			rec := httptest.NewRecorder()

			key, err := store.Issue(rec, req)
			if err != nil {
				t.Fatalf("Issue returned error: %v", err)
			}

			cookie := findCookie(rec.Result().Cookies(), wrappingKeyCookieName)
			if cookie == nil {
				t.Fatal("expected a wrapping key cookie")
			}
			if cookie.Value != key {
				t.Fatal("expected the cookie to carry the issued key")
			}
			if !cookie.HttpOnly {
				t.Fatal("expected the cookie to be HttpOnly")
			}
			if cookie.SameSite != http.SameSiteStrictMode {
				t.Fatalf("expected SameSite=Strict, got %v", cookie.SameSite)
			}
			if cookie.Path != "/" {
				t.Fatalf("expected Path=/, got %q", cookie.Path)
			}
			if cookie.MaxAge != 1234 {
				t.Fatalf("expected MaxAge 1234, got %d", cookie.MaxAge)
			}
			if cookie.Secure != tt.wantSecure {
				t.Fatalf("expected Secure=%v, got %v", tt.wantSecure, cookie.Secure)
			}
		})
	}
}

func TestCookieWrappingKeyStore_IssueGeneratesFreshKeys(t *testing.T) {
	store := NewCookieWrappingKeyStore(encryption.NewDefaultEncryptionService(), 3600, ccc.NopLogger)
	req := httptest.NewRequest(http.MethodPost, "/login", nil)

	first, err := store.Issue(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	second, err := store.Issue(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	if first == second {
		t.Fatal("expected each Issue to generate a new key")
	}
}

func TestSessionSignInManager_GetCurrentUserRequiresUsableMek(t *testing.T) {
	f := newMekStoreFixture()
	userRepo := NewInMemoryUserRepository()
	if _, err := userRepo.Add(&User{Id: "user-1", UserName: "alice", IsActive: true}); err != nil {
		t.Fatalf("failed to add user: %v", err)
	}
	manager := NewSessionSignInManager(userRepo, nil, f.sessionStore, f.mekStore, ccc.NopLogger)

	// Simulate a sign-in: user ID and wrapped MEK in the session
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	session, _ := f.sessionStore.Get(req, sessionName)
	session.Values["userId"] = "user-1"
	if err := session.Save(req, rec); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}
	if err := f.mekStore.Store(rec, req, f.generateKey(t)); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}
	cookies := rec.Result().Cookies()

	user, err := manager.GetCurrentUser(requestWithCookies(cookies...))
	if err != nil {
		t.Fatalf("GetCurrentUser returned error: %v", err)
	}
	if user.Id != "user-1" {
		t.Fatalf("expected user-1 to be signed in, got %q", user.Id)
	}

	user, err = manager.GetCurrentUser(requestWithCookies(findCookie(cookies, sessionName)))
	if err != nil {
		t.Fatalf("GetCurrentUser returned error: %v", err)
	}
	if user.Id != "" {
		t.Fatal("expected a session without its wrapping key to count as signed out")
	}
}
