package scanhandoff

import (
	"context"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// requireLocalRedis skips the test if a local Redis instance isn't reachable, so this
// integration test doesn't break local runs without Redis - CI already starts one for
// the session-store tests (see .github/workflows/legacy-ci.yml).
func requireLocalRedis(t *testing.T) ccc.AppConfig {
	t.Helper()

	config := ccc.AppConfig{
		RedisAddress: "localhost:6379",
		RedisNetwork: "tcp",
		RedisSize:    5,
	}

	conn, err := redis.Dial(config.RedisNetwork, config.RedisAddress)
	if err != nil {
		t.Skipf("skipping: no local Redis reachable at %s: %v", config.RedisAddress, err)
	}
	defer conn.Close()

	return config
}

func TestRedisScanHandoffStore_SaveGetDelete(t *testing.T) {
	config := requireLocalRedis(t)

	store, err := NewRedisScanHandoffStore(config, ccc.NopLogger)
	if err != nil {
		t.Fatalf("NewRedisScanHandoffStore returned error: %v", err)
	}

	record := &StagedScan{
		Token:      "test-token-" + t.Name(),
		UserId:     "user-1",
		State:      ScanHandoffStateReady,
		FileName:   "scan.jpg",
		CipherBlob: []byte{0x01, 0x02, 0x03, 0xff},
		CreatedAt:  time.Now().UTC(),
	}
	defer store.Delete(context.Background(), record.Token)

	if err := store.Save(context.Background(), record, time.Minute); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := store.Get(context.Background(), record.Token)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got == nil {
		t.Fatal("expected a record, got nil")
	}
	if got.UserId != record.UserId || got.FileName != record.FileName || got.State != record.State {
		t.Fatalf("retrieved record does not match saved record: %+v", got)
	}
	if string(got.CipherBlob) != string(record.CipherBlob) {
		t.Fatalf("retrieved CipherBlob does not match: got %v, want %v", got.CipherBlob, record.CipherBlob)
	}

	if err := store.Delete(context.Background(), record.Token); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	got, err = store.Get(context.Background(), record.Token)
	if err != nil {
		t.Fatalf("Get after delete returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil after delete, got %+v", got)
	}
}

func TestRedisScanHandoffStore_GetReturnsNilForMissingToken(t *testing.T) {
	config := requireLocalRedis(t)

	store, err := NewRedisScanHandoffStore(config, ccc.NopLogger)
	if err != nil {
		t.Fatalf("NewRedisScanHandoffStore returned error: %v", err)
	}

	got, err := store.Get(context.Background(), "does-not-exist-"+t.Name())
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for a missing token, got %+v", got)
	}
}

func TestRedisScanHandoffStore_RecordExpiresAfterTTL(t *testing.T) {
	config := requireLocalRedis(t)

	store, err := NewRedisScanHandoffStore(config, ccc.NopLogger)
	if err != nil {
		t.Fatalf("NewRedisScanHandoffStore returned error: %v", err)
	}

	record := &StagedScan{
		Token:     "ttl-token-" + t.Name(),
		UserId:    "user-1",
		State:     ScanHandoffStatePending,
		CreatedAt: time.Now().UTC(),
	}
	defer store.Delete(context.Background(), record.Token)

	if err := store.Save(context.Background(), record, time.Second); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	time.Sleep(1200 * time.Millisecond)

	got, err := store.Get(context.Background(), record.Token)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected the record to have expired, got %+v", got)
	}
}
