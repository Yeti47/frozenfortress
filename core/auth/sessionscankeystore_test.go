package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/sessions"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// carryCookies builds a fresh request carrying the cookies set on a previous response,
// simulating a browser round-trip across separate HTTP requests against the same session.
func carryCookies(rec *httptest.ResponseRecorder) *http.Request {
	req := httptest.NewRequest("GET", "/", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

func newTestSessionStore() sessions.Store {
	return sessions.NewCookieStore([]byte("test-signing-key-32-bytes-long!"))
}

func TestSessionScanKeyStore_StoreThenRetrieve(t *testing.T) {
	sessionStore := newTestSessionStore()
	store := NewSessionScanKeyStore(sessionStore, ccc.NopLogger)

	w1 := httptest.NewRecorder()
	if err := store.Store(w1, httptest.NewRequest("GET", "/", nil), "token-1", "the-key"); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}

	w2 := httptest.NewRecorder()
	key, err := store.Retrieve(w2, carryCookies(w1), "token-1")
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if key != "the-key" {
		t.Fatalf("expected %q, got %q", "the-key", key)
	}
}

func TestSessionScanKeyStore_RetrieveReturnsEmptyForUnknownToken(t *testing.T) {
	sessionStore := newTestSessionStore()
	store := NewSessionScanKeyStore(sessionStore, ccc.NopLogger)

	w := httptest.NewRecorder()
	key, err := store.Retrieve(w, httptest.NewRequest("GET", "/", nil), "does-not-exist")
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if key != "" {
		t.Fatalf("expected an empty key for an unknown token, got %q", key)
	}
}

func TestSessionScanKeyStore_EntriesAreKeyedIndependentlyPerToken(t *testing.T) {
	sessionStore := newTestSessionStore()
	store := NewSessionScanKeyStore(sessionStore, ccc.NopLogger)

	w1 := httptest.NewRecorder()
	if err := store.Store(w1, httptest.NewRequest("GET", "/", nil), "token-a", "key-a"); err != nil {
		t.Fatalf("Store (a) returned error: %v", err)
	}

	w2 := httptest.NewRecorder()
	if err := store.Store(w2, carryCookies(w1), "token-b", "key-b"); err != nil {
		t.Fatalf("Store (b) returned error: %v", err)
	}

	w3 := httptest.NewRecorder()
	keyA, err := store.Retrieve(w3, carryCookies(w2), "token-a")
	if err != nil {
		t.Fatalf("Retrieve (a) returned error: %v", err)
	}
	if keyA != "key-a" {
		t.Fatalf("expected %q, got %q", "key-a", keyA)
	}

	w4 := httptest.NewRecorder()
	keyB, err := store.Retrieve(w4, carryCookies(w2), "token-b")
	if err != nil {
		t.Fatalf("Retrieve (b) returned error: %v", err)
	}
	if keyB != "key-b" {
		t.Fatalf("expected %q, got %q", "key-b", keyB)
	}
}

func TestSessionScanKeyStore_DeleteRemovesEntry(t *testing.T) {
	sessionStore := newTestSessionStore()
	store := NewSessionScanKeyStore(sessionStore, ccc.NopLogger)

	w1 := httptest.NewRecorder()
	if err := store.Store(w1, httptest.NewRequest("GET", "/", nil), "token-1", "the-key"); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}

	w2 := httptest.NewRecorder()
	if err := store.Delete(w2, carryCookies(w1), "token-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	w3 := httptest.NewRecorder()
	key, err := store.Retrieve(w3, carryCookies(w2), "token-1")
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if key != "" {
		t.Fatalf("expected an empty key after deletion, got %q", key)
	}
}

func TestSessionScanKeyStore_RetrieveTreatsExpiredEntryAsAbsent(t *testing.T) {
	sessionStore := newTestSessionStore()
	store := NewSessionScanKeyStore(sessionStore, ccc.NopLogger)
	token := "expiring-token"

	w1 := httptest.NewRecorder()
	if err := store.Store(w1, httptest.NewRequest("GET", "/", nil), token, "the-key"); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}

	// Backdate the stored timestamp directly (white-box, same package) to simulate an
	// entry older than scanKeyTTL without waiting for real time to pass. The surrounding
	// session's own MaxAge is untouched and far from expiring - this proves the tighter,
	// independent expiry on this specific entry is what's being enforced.
	r2 := carryCookies(w1)
	session, err := sessionStore.Get(r2, sessionName)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	session.Values[scanKeyTimestampSessionKey(token)] = time.Now().Add(-scanKeyTTL - time.Minute).UTC().Format(time.RFC3339)
	w2 := httptest.NewRecorder()
	if err := sessionStore.Save(r2, w2, session); err != nil {
		t.Fatalf("failed to save backdated session: %v", err)
	}

	w3 := httptest.NewRecorder()
	key, err := store.Retrieve(w3, carryCookies(w2), token)
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if key != "" {
		t.Fatalf("expected the expired key to be treated as absent, got %q", key)
	}
}
