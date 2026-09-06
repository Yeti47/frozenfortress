package scanhandoff

import (
	"context"
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// scanKeyRedisKeyPrefix namespaces scan key records separately from ffscanhandoff:
// records, so a single Redis key never holds both a ciphertext blob and the key that
// decrypts it.
const scanKeyRedisKeyPrefix = "ffscankey:"

// RedisScanKeyStore is a Redis-backed ScanKeyStore, relying on Redis's native per-key
// TTL for expiry rather than any active cleanup step - an abandoned handoff's key
// disappears on its own even if nothing ever calls Retrieve or Delete for it.
type RedisScanKeyStore struct {
	pool   *redis.Pool
	logger ccc.Logger
}

// NewRedisScanKeyStore creates a new RedisScanKeyStore using the same FF_REDIS_*
// configuration as the session store.
func NewRedisScanKeyStore(config ccc.AppConfig, logger ccc.Logger) *RedisScanKeyStore {
	if logger == nil {
		logger = ccc.NopLogger
	}

	logger.Info("Creating Redis scan key store", "redis_address", config.RedisAddress, "redis_network", config.RedisNetwork, "pool_size", config.RedisSize)

	return &RedisScanKeyStore{pool: newRedisPool(config), logger: logger}
}

func scanKeyRedisKey(token string) string {
	return scanKeyRedisKeyPrefix + token
}

func ttlSeconds(ttl time.Duration) int {
	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		return 1
	}
	return seconds
}

// Store saves key for token, expiring after ttl.
func (s *RedisScanKeyStore) Store(ctx context.Context, token, key string, ttl time.Duration) error {
	conn := s.pool.Get()
	defer conn.Close()

	if _, err := conn.Do("SET", scanKeyRedisKey(token), key, "EX", ttlSeconds(ttl)); err != nil {
		s.logger.Error("Failed to save scan key", "token", token, "error", err)
		return fmt.Errorf("failed to save scan key: %w", err)
	}

	return nil
}

// Retrieve returns the key for token, or "" if it doesn't exist or has expired.
func (s *RedisScanKeyStore) Retrieve(ctx context.Context, token string) (string, error) {
	conn := s.pool.Get()
	defer conn.Close()

	key, err := redis.String(conn.Do("GET", scanKeyRedisKey(token)))
	if err == redis.ErrNil {
		return "", nil
	}
	if err != nil {
		s.logger.Error("Failed to retrieve scan key", "token", token, "error", err)
		return "", fmt.Errorf("failed to retrieve scan key: %w", err)
	}

	return key, nil
}

// Refresh extends token's expiry to ttl without needing to know its current value.
func (s *RedisScanKeyStore) Refresh(ctx context.Context, token string, ttl time.Duration) error {
	conn := s.pool.Get()
	defer conn.Close()

	if _, err := conn.Do("EXPIRE", scanKeyRedisKey(token), ttlSeconds(ttl)); err != nil {
		s.logger.Error("Failed to refresh scan key expiry", "token", token, "error", err)
		return fmt.Errorf("failed to refresh scan key expiry: %w", err)
	}

	return nil
}

// Delete removes the key for token. It is not an error if it doesn't exist.
func (s *RedisScanKeyStore) Delete(ctx context.Context, token string) error {
	conn := s.pool.Get()
	defer conn.Close()

	if _, err := conn.Do("DEL", scanKeyRedisKey(token)); err != nil {
		s.logger.Error("Failed to delete scan key", "token", token, "error", err)
		return fmt.Errorf("failed to delete scan key: %w", err)
	}

	return nil
}
