package scanhandoff

import (
	"context"
	"testing"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

func TestRedisScanKeyStore_StoreThenRetrieve(t *testing.T) {
	config := requireLocalRedis(t)
	store := NewRedisScanKeyStore(config, ccc.NopLogger)
	token := "test-token-" + t.Name()
	defer store.Delete(context.Background(), token)

	if err := store.Store(context.Background(), token, "the-key", time.Minute); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}

	key, err := store.Retrieve(context.Background(), token)
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if key != "the-key" {
		t.Fatalf("expected %q, got %q", "the-key", key)
	}
}

func TestRedisScanKeyStore_RetrieveReturnsEmptyForMissingToken(t *testing.T) {
	config := requireLocalRedis(t)
	store := NewRedisScanKeyStore(config, ccc.NopLogger)

	key, err := store.Retrieve(context.Background(), "does-not-exist-"+t.Name())
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if key != "" {
		t.Fatalf("expected an empty key for a missing token, got %q", key)
	}
}

func TestRedisScanKeyStore_DeleteRemovesKey(t *testing.T) {
	config := requireLocalRedis(t)
	store := NewRedisScanKeyStore(config, ccc.NopLogger)
	token := "test-token-" + t.Name()

	if err := store.Store(context.Background(), token, "the-key", time.Minute); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}
	if err := store.Delete(context.Background(), token); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	key, err := store.Retrieve(context.Background(), token)
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if key != "" {
		t.Fatalf("expected an empty key after deletion, got %q", key)
	}
}

func TestRedisScanKeyStore_RefreshExtendsExpiry(t *testing.T) {
	config := requireLocalRedis(t)
	store := NewRedisScanKeyStore(config, ccc.NopLogger)
	token := "test-token-" + t.Name()
	defer store.Delete(context.Background(), token)

	if err := store.Store(context.Background(), token, "the-key", 500*time.Millisecond); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}
	if err := store.Refresh(context.Background(), token, time.Minute); err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}

	// Past the original short TTL, but well within the refreshed one - proves Refresh
	// actually extended the expiry rather than being a no-op.
	time.Sleep(700 * time.Millisecond)

	key, err := store.Retrieve(context.Background(), token)
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if key != "the-key" {
		t.Fatalf("expected the key to survive past its original TTL after refresh, got %q", key)
	}
}

func TestRedisScanKeyStore_KeyExpiresAfterTTL(t *testing.T) {
	config := requireLocalRedis(t)
	store := NewRedisScanKeyStore(config, ccc.NopLogger)
	token := "test-token-" + t.Name()
	defer store.Delete(context.Background(), token)

	if err := store.Store(context.Background(), token, "the-key", time.Second); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}

	time.Sleep(1200 * time.Millisecond)

	key, err := store.Retrieve(context.Background(), token)
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if key != "" {
		t.Fatalf("expected the key to have expired, got %q", key)
	}
}
