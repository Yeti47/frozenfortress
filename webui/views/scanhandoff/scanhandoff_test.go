package scanhandoff

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/scanhandoff"
)

// fakeSignInManager is a minimal auth.SignInManager stub for handler tests.
type fakeSignInManager struct {
	user auth.UserDto
	err  error
}

func (f *fakeSignInManager) SignIn(w http.ResponseWriter, r *http.Request, req auth.SignInRequest) (auth.SignInResponse, error) {
	return auth.SignInResponse{}, nil
}
func (f *fakeSignInManager) RecoverySignIn(w http.ResponseWriter, r *http.Request, req auth.RecoverySignInRequest) (auth.RecoverySignInResponse, error) {
	return auth.RecoverySignInResponse{}, nil
}
func (f *fakeSignInManager) SignOut(w http.ResponseWriter, r *http.Request) error { return nil }
func (f *fakeSignInManager) GetCurrentUser(r *http.Request) (auth.UserDto, error) {
	return f.user, f.err
}
func (f *fakeSignInManager) IsSignedIn(r *http.Request) (bool, error) { return f.err == nil, nil }

func newAuthedSignInManager(userId string) auth.SignInManager {
	return &fakeSignInManager{user: auth.UserDto{Id: userId, IsActive: true}}
}

func newUnauthenticatedSignInManager() auth.SignInManager {
	return &fakeSignInManager{err: ccc.NewUnauthorizedError("not signed in")}
}

// in-memory ScanHandoffStore/ScanKeyStore fakes, so these handler tests don't depend on
// a running Redis instance - the Redis-backed implementations have their own tests in
// core/scanhandoff.
type inMemoryScanHandoffStore struct {
	mu      sync.Mutex
	records map[string]*scanhandoff.StagedScan
}

func newInMemoryScanHandoffStore() *inMemoryScanHandoffStore {
	return &inMemoryScanHandoffStore{records: make(map[string]*scanhandoff.StagedScan)}
}

func (s *inMemoryScanHandoffStore) Save(ctx context.Context, record *scanhandoff.StagedScan, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	clone := *record
	s.records[record.Token] = &clone
	return nil
}

func (s *inMemoryScanHandoffStore) Get(ctx context.Context, token string) (*scanhandoff.StagedScan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.records[token]
	if !ok {
		return nil, nil
	}

	clone := *record
	return &clone, nil
}

func (s *inMemoryScanHandoffStore) Delete(ctx context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.records, token)
	return nil
}

type inMemoryScanKeyStore struct {
	mu   sync.Mutex
	keys map[string]string
}

func newInMemoryScanKeyStore() *inMemoryScanKeyStore {
	return &inMemoryScanKeyStore{keys: make(map[string]string)}
}

func (s *inMemoryScanKeyStore) Store(ctx context.Context, token, key string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.keys[token] = key
	return nil
}

func (s *inMemoryScanKeyStore) Retrieve(ctx context.Context, token string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.keys[token], nil
}

func (s *inMemoryScanKeyStore) Refresh(ctx context.Context, token string, ttl time.Duration) error {
	return nil
}

func (s *inMemoryScanKeyStore) Delete(ctx context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.keys, token)
	return nil
}

func newTestHandoffService() scanhandoff.ScanHandoffService {
	return scanhandoff.NewDefaultScanHandoffService(newInMemoryScanHandoffStore(), newInMemoryScanKeyStore(), encryption.NewDefaultEncryptionService(), ccc.NopLogger)
}

func newTestRouter(signInManager auth.SignInManager, handoffService scanhandoff.ScanHandoffService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router, signInManager, handoffService, ccc.NopLogger)
	return router
}

func multipartFileBody(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write form file content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	return body, writer.FormDataContentType()
}

func TestScanHandoffRoundTrip_StartUploadStatusFile(t *testing.T) {
	signInManager := newAuthedSignInManager("user-1")
	router := newTestRouter(signInManager, newTestHandoffService())
	enc := encryption.NewDefaultEncryptionService()

	key, err := enc.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	// 1. Start
	startBody, _ := json.Marshal(map[string]string{"key": key})
	startReq := httptest.NewRequest(http.MethodPost, "/api/scan-handoff/start", bytes.NewReader(startBody))
	startReq.Header.Set("Content-Type", "application/json")
	startRec := httptest.NewRecorder()
	router.ServeHTTP(startRec, startReq)

	if startRec.Code != 200 {
		t.Fatalf("start: expected 200, got %d (%s)", startRec.Code, startRec.Body.String())
	}

	var startResp struct {
		Success          bool   `json:"success"`
		Token            string `json:"token"`
		ExpiresInSeconds int    `json:"expiresInSeconds"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &startResp); err != nil {
		t.Fatalf("failed to parse start response: %v", err)
	}
	if !startResp.Success || startResp.Token == "" {
		t.Fatalf("expected a successful start response with a token, got %+v", startResp)
	}
	if startResp.ExpiresInSeconds != int(scanhandoff.HandoffTTL.Seconds()) {
		t.Fatalf("expected expiresInSeconds %d, got %d", int(scanhandoff.HandoffTTL.Seconds()), startResp.ExpiresInSeconds)
	}

	// 2. Upload (as the companion app would, no session cookie at all)
	plainData := []byte("scanned document bytes")
	cipherBlob, err := enc.EncryptBytes(plainData, key)
	if err != nil {
		t.Fatalf("failed to encrypt test payload: %v", err)
	}

	uploadBody, contentType := multipartFileBody(t, "scan.jpg", cipherBlob)
	uploadReq := httptest.NewRequest(http.MethodPost, "/api/scan-handoff/"+startResp.Token+"/upload", uploadBody)
	uploadReq.Header.Set("Content-Type", contentType)
	uploadRec := httptest.NewRecorder()
	router.ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != 200 {
		t.Fatalf("upload: expected 200, got %d (%s)", uploadRec.Code, uploadRec.Body.String())
	}

	// 3. Status
	statusReq := httptest.NewRequest(http.MethodGet, "/api/scan-handoff/"+startResp.Token+"/status", nil)
	statusRec := httptest.NewRecorder()
	router.ServeHTTP(statusRec, statusReq)

	if statusRec.Code != 200 {
		t.Fatalf("status: expected 200, got %d (%s)", statusRec.Code, statusRec.Body.String())
	}

	var statusResp struct {
		Success bool   `json:"success"`
		State   string `json:"state"`
	}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("failed to parse status response: %v", err)
	}
	if statusResp.State != string(scanhandoff.ScanHandoffStateReady) {
		t.Fatalf("expected state %q, got %q", scanhandoff.ScanHandoffStateReady, statusResp.State)
	}

	// 4. Fetch the decrypted file
	fileReq := httptest.NewRequest(http.MethodGet, "/api/scan-handoff/"+startResp.Token+"/file", nil)
	fileRec := httptest.NewRecorder()
	router.ServeHTTP(fileRec, fileReq)

	if fileRec.Code != 200 {
		t.Fatalf("file: expected 200, got %d (%s)", fileRec.Code, fileRec.Body.String())
	}
	if fileRec.Header().Get("X-Scan-Filename") != "scan.jpg" {
		t.Fatalf("expected X-Scan-Filename %q, got %q", "scan.jpg", fileRec.Header().Get("X-Scan-Filename"))
	}
	if fileRec.Body.String() != string(plainData) {
		t.Fatalf("expected decrypted body %q, got %q", plainData, fileRec.Body.String())
	}

	// Fetch-and-burn: fetching again must fail now that the handoff is consumed.
	secondFileReq := httptest.NewRequest(http.MethodGet, "/api/scan-handoff/"+startResp.Token+"/file", nil)
	secondFileRec := httptest.NewRecorder()
	router.ServeHTTP(secondFileRec, secondFileReq)
	if secondFileRec.Code == 200 {
		t.Fatalf("expected the second fetch of a consumed token to fail, got 200")
	}
}

func TestHandleStartHandoff_RejectsMissingKey(t *testing.T) {
	router := newTestRouter(newAuthedSignInManager("user-1"), newTestHandoffService())

	req := httptest.NewRequest(http.MethodPost, "/api/scan-handoff/start", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("expected 400 for a missing key, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleUploadScan_RejectsMissingFile(t *testing.T) {
	// Deliberately unauthenticated: the upload endpoint has no session and must not
	// require one - it should fail only because the file is missing.
	router := newTestRouter(newUnauthenticatedSignInManager(), newTestHandoffService())

	req := httptest.NewRequest(http.MethodPost, "/api/scan-handoff/some-token/upload", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("expected 400 for a missing file, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleHandoffStatus_ReturnsNotFoundForUnknownToken(t *testing.T) {
	router := newTestRouter(newAuthedSignInManager("user-1"), newTestHandoffService())

	req := httptest.NewRequest(http.MethodGet, "/api/scan-handoff/does-not-exist/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Fatalf("expected 404 for an unknown token, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestScanHandoffAuthenticatedRoutes_RedirectWhenNotSignedIn(t *testing.T) {
	router := newTestRouter(newUnauthenticatedSignInManager(), newTestHandoffService())

	req := httptest.NewRequest(http.MethodGet, "/api/scan-handoff/some-token/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected a %d redirect to login for an unauthenticated request, got %d", http.StatusSeeOther, rec.Code)
	}
	if rec.Header().Get("Location") != "/login" {
		t.Fatalf("expected redirect to /login, got %q", rec.Header().Get("Location"))
	}
}
