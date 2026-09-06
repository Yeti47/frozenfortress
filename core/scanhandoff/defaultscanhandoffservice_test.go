package scanhandoff

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
)

// inMemoryScanHandoffStore is a minimal fake ScanHandoffStore for service-level tests,
// so these tests don't depend on a running Redis instance.
type inMemoryScanHandoffStore struct {
	mu      sync.Mutex
	records map[string]*StagedScan
}

func newInMemoryScanHandoffStore() *inMemoryScanHandoffStore {
	return &inMemoryScanHandoffStore{records: make(map[string]*StagedScan)}
}

func (s *inMemoryScanHandoffStore) Save(ctx context.Context, record *StagedScan, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	clone := *record
	s.records[record.Token] = &clone
	return nil
}

func (s *inMemoryScanHandoffStore) Get(ctx context.Context, token string) (*StagedScan, error) {
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

func newTestService(store ScanHandoffStore) *DefaultScanHandoffService {
	return NewDefaultScanHandoffService(store, encryption.NewDefaultEncryptionService(), ccc.NopLogger)
}

func TestStartHandoff_CreatesRetrievablePendingRecord(t *testing.T) {
	svc := newTestService(newInMemoryScanHandoffStore())

	token, err := svc.StartHandoff(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("StartHandoff returned error: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty token")
	}

	state, err := svc.GetStatus(context.Background(), token, "user-1")
	if err != nil {
		t.Fatalf("GetStatus returned error: %v", err)
	}
	if state != ScanHandoffStatePending {
		t.Fatalf("expected pending state, got %q", state)
	}
}

func TestUploadScan_TransitionsToReady(t *testing.T) {
	svc := newTestService(newInMemoryScanHandoffStore())
	enc := encryption.NewDefaultEncryptionService()

	token, err := svc.StartHandoff(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("StartHandoff returned error: %v", err)
	}

	key, _ := enc.GenerateKey()
	cipherBlob, err := enc.EncryptBytes([]byte("scan bytes"), key)
	if err != nil {
		t.Fatalf("failed to encrypt test payload: %v", err)
	}

	if err := svc.UploadScan(context.Background(), token, "scan.jpg", cipherBlob); err != nil {
		t.Fatalf("UploadScan returned error: %v", err)
	}

	state, err := svc.GetStatus(context.Background(), token, "user-1")
	if err != nil {
		t.Fatalf("GetStatus returned error: %v", err)
	}
	if state != ScanHandoffStateReady {
		t.Fatalf("expected ready state, got %q", state)
	}
}

func TestUploadScan_RejectsUnknownToken(t *testing.T) {
	svc := newTestService(newInMemoryScanHandoffStore())

	err := svc.UploadScan(context.Background(), "does-not-exist", "scan.jpg", []byte("cipher"))
	if err == nil {
		t.Fatal("expected an error for an unknown token")
	}
	if !ccc.IsNotFound(err) {
		t.Fatalf("expected a not-found error, got %v", err)
	}
}

func TestUploadScan_RejectsAlreadyReadyToken(t *testing.T) {
	svc := newTestService(newInMemoryScanHandoffStore())
	enc := encryption.NewDefaultEncryptionService()

	token, err := svc.StartHandoff(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("StartHandoff returned error: %v", err)
	}

	key, _ := enc.GenerateKey()
	cipherBlob, _ := enc.EncryptBytes([]byte("scan bytes"), key)

	if err := svc.UploadScan(context.Background(), token, "scan.jpg", cipherBlob); err != nil {
		t.Fatalf("first upload failed unexpectedly: %v", err)
	}

	// Proves single-use: a second upload to the same (now-ready) token must be rejected.
	if err := svc.UploadScan(context.Background(), token, "scan.jpg", cipherBlob); err == nil {
		t.Fatal("expected an error re-uploading to an already-ready token")
	}
}

func TestGetStatus_ReturnsNotFoundForMismatchedOwner(t *testing.T) {
	svc := newTestService(newInMemoryScanHandoffStore())

	token, err := svc.StartHandoff(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("StartHandoff returned error: %v", err)
	}

	_, err = svc.GetStatus(context.Background(), token, "user-2")
	if err == nil {
		t.Fatal("expected an error for a mismatched owner")
	}
	if !ccc.IsNotFound(err) {
		t.Fatalf("expected a not-found error, got %v", err)
	}
}

func TestFetchAndConsume_DecryptsAndDeletesRecord(t *testing.T) {
	svc := newTestService(newInMemoryScanHandoffStore())
	enc := encryption.NewDefaultEncryptionService()

	token, err := svc.StartHandoff(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("StartHandoff returned error: %v", err)
	}

	key, _ := enc.GenerateKey()
	plainData := []byte("scanned document bytes")
	cipherBlob, err := enc.EncryptBytes(plainData, key)
	if err != nil {
		t.Fatalf("failed to encrypt test payload: %v", err)
	}

	if err := svc.UploadScan(context.Background(), token, "scan.jpg", cipherBlob); err != nil {
		t.Fatalf("UploadScan returned error: %v", err)
	}

	fileName, gotData, err := svc.FetchAndConsume(context.Background(), token, "user-1", key)
	if err != nil {
		t.Fatalf("FetchAndConsume returned error: %v", err)
	}
	if fileName != "scan.jpg" {
		t.Fatalf("expected filename 'scan.jpg', got %q", fileName)
	}
	if string(gotData) != string(plainData) {
		t.Fatalf("decrypted data mismatch: got %q, want %q", gotData, plainData)
	}

	// Fetch-and-burn: a second fetch of the same token must fail.
	if _, _, err := svc.FetchAndConsume(context.Background(), token, "user-1", key); err == nil {
		t.Fatal("expected an error fetching an already-consumed token")
	}
}

func TestFetchAndConsume_FailsWithWrongKey(t *testing.T) {
	svc := newTestService(newInMemoryScanHandoffStore())
	enc := encryption.NewDefaultEncryptionService()

	token, err := svc.StartHandoff(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("StartHandoff returned error: %v", err)
	}

	key, _ := enc.GenerateKey()
	wrongKey, _ := enc.GenerateKey()
	cipherBlob, err := enc.EncryptBytes([]byte("scan bytes"), key)
	if err != nil {
		t.Fatalf("failed to encrypt test payload: %v", err)
	}

	if err := svc.UploadScan(context.Background(), token, "scan.jpg", cipherBlob); err != nil {
		t.Fatalf("UploadScan returned error: %v", err)
	}

	if _, _, err := svc.FetchAndConsume(context.Background(), token, "user-1", wrongKey); err == nil {
		t.Fatal("expected an error decrypting with the wrong key")
	}
}
