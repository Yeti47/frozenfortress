// Package testutil holds the shared fakes and the test server for API handler tests.
package testutil

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/scanhandoff"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/updates"
)

// FakeSignInManager is a minimal auth.SignInManager stub.
type FakeSignInManager struct {
	User auth.UserDto
	Err  error
}

func (f *FakeSignInManager) SignIn(w http.ResponseWriter, r *http.Request, req auth.SignInRequest) (auth.SignInResponse, error) {
	return auth.SignInResponse{}, nil
}
func (f *FakeSignInManager) RecoverySignIn(w http.ResponseWriter, r *http.Request, req auth.RecoverySignInRequest) (auth.RecoverySignInResponse, error) {
	return auth.RecoverySignInResponse{}, nil
}
func (f *FakeSignInManager) SignOut(w http.ResponseWriter, r *http.Request) error { return nil }
func (f *FakeSignInManager) GetCurrentUser(r *http.Request) (auth.UserDto, error) {
	return f.User, f.Err
}
func (f *FakeSignInManager) IsSignedIn(r *http.Request) (bool, error) { return f.Err == nil, nil }

// NewAuthedSignInManager returns a SignInManager whose current user is userId.
func NewAuthedSignInManager(userId string) auth.SignInManager {
	return &FakeSignInManager{User: auth.UserDto{Id: userId, IsActive: true}}
}

// NewUnauthenticatedSignInManager returns a SignInManager that reports no signed-in user.
func NewUnauthenticatedSignInManager() auth.SignInManager {
	return &FakeSignInManager{Err: ccc.NewUnauthorizedError("not signed in")}
}

// FakeMekStore is an auth.MekStore that always returns the same MEK.
type FakeMekStore struct {
	Mek string
}

// NewFakeMekStore creates a FakeMekStore with a freshly generated MEK.
func NewFakeMekStore() *FakeMekStore {
	mek, err := encryption.NewDefaultEncryptionService().GenerateKey()
	if err != nil {
		panic("failed to generate test MEK: " + err.Error())
	}
	return &FakeMekStore{Mek: mek}
}

func (f *FakeMekStore) Store(w http.ResponseWriter, r *http.Request, mek string) error { return nil }
func (f *FakeMekStore) Retrieve(r *http.Request) (string, error)                       { return f.Mek, nil }
func (f *FakeMekStore) Delete(w http.ResponseWriter, r *http.Request) error            { return nil }

// InMemoryScanHandoffStore is a scanhandoff.ScanHandoffStore without Redis.
type InMemoryScanHandoffStore struct {
	mu      sync.Mutex
	records map[string]*scanhandoff.StagedScan
}

func NewInMemoryScanHandoffStore() *InMemoryScanHandoffStore {
	return &InMemoryScanHandoffStore{records: make(map[string]*scanhandoff.StagedScan)}
}

func (s *InMemoryScanHandoffStore) Save(ctx context.Context, record *scanhandoff.StagedScan, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := *record
	s.records[record.Token] = &clone
	return nil
}

func (s *InMemoryScanHandoffStore) Get(ctx context.Context, token string) (*scanhandoff.StagedScan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[token]
	if !ok {
		return nil, nil
	}
	clone := *record
	return &clone, nil
}

func (s *InMemoryScanHandoffStore) Delete(ctx context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, token)
	return nil
}

// InMemoryScanKeyStore is a scanhandoff.ScanKeyStore without Redis.
type InMemoryScanKeyStore struct {
	mu   sync.Mutex
	keys map[string]string
}

func NewInMemoryScanKeyStore() *InMemoryScanKeyStore {
	return &InMemoryScanKeyStore{keys: make(map[string]string)}
}

func (s *InMemoryScanKeyStore) Store(ctx context.Context, token, key string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[token] = key
	return nil
}

func (s *InMemoryScanKeyStore) Retrieve(ctx context.Context, token string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.keys[token], nil
}

func (s *InMemoryScanKeyStore) Refresh(ctx context.Context, token string, ttl time.Duration) error {
	return nil
}

func (s *InMemoryScanKeyStore) Delete(ctx context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.keys, token)
	return nil
}

// NewTestHandoffService builds a scan handoff service on in-memory stores.
func NewTestHandoffService() scanhandoff.ScanHandoffService {
	return scanhandoff.NewDefaultScanHandoffService(NewInMemoryScanHandoffStore(), NewInMemoryScanKeyStore(), encryption.NewDefaultEncryptionService(), ccc.NopLogger)
}

// FakeUpdateChecker returns a fixed release from Latest.
type FakeUpdateChecker struct {
	Release *updates.ReleaseInfo
}

func (f *FakeUpdateChecker) Check(ctx context.Context) (*updates.ReleaseInfo, error) {
	return f.Release, nil
}
func (f *FakeUpdateChecker) Latest() *updates.ReleaseInfo { return f.Release }

// FakePinger is a database stand-in for the health check.
type FakePinger struct {
	Err error
}

func (f *FakePinger) PingContext(ctx context.Context) error { return f.Err }
