package scanhandoff

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// scanHandoffKeyPrefix namespaces scan handoff records in the shared Redis instance,
// keeping them distinct from the gorilla/sessions session records that live there too.
const scanHandoffKeyPrefix = "ffscanhandoff:"

// RedisScanHandoffStore is a Redis-backed ScanHandoffStore. It reuses the same Redis
// connection details as the session store (see core/auth.CreateRedisStore) but talks to
// Redis directly, since these records are not gorilla/sessions sessions.
type RedisScanHandoffStore struct {
	pool   *redis.Pool
	logger ccc.Logger
}

// NewRedisScanHandoffStore creates a new RedisScanHandoffStore using the same
// FF_REDIS_* configuration as the session store.
func NewRedisScanHandoffStore(config ccc.AppConfig, logger ccc.Logger) (*RedisScanHandoffStore, error) {
	if logger == nil {
		logger = ccc.NopLogger
	}

	logger.Info("Creating Redis scan handoff store", "redis_address", config.RedisAddress, "redis_network", config.RedisNetwork, "pool_size", config.RedisSize)

	pool := &redis.Pool{
		MaxIdle:     config.RedisSize,
		IdleTimeout: 240 * time.Second,
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
		Dial: func() (redis.Conn, error) {
			return redis.Dial(
				config.RedisNetwork,
				config.RedisAddress,
				redis.DialUsername(config.RedisUser),
				redis.DialPassword(config.RedisPassword),
			)
		},
	}

	logger.Info("Redis scan handoff store created successfully")

	return &RedisScanHandoffStore{pool: pool, logger: logger}, nil
}

func redisKey(token string) string {
	return scanHandoffKeyPrefix + token
}

// Save writes or overwrites the record for record.Token, expiring after ttl.
func (s *RedisScanHandoffStore) Save(ctx context.Context, record *StagedScan, ttl time.Duration) error {
	data, err := json.Marshal(record)
	if err != nil {
		s.logger.Error("Failed to marshal scan handoff record", "token", record.Token, "error", err)
		return fmt.Errorf("failed to marshal scan handoff record: %w", err)
	}

	conn := s.pool.Get()
	defer conn.Close()

	ttlSeconds := int(ttl.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = 1
	}

	if _, err := conn.Do("SET", redisKey(record.Token), data, "EX", ttlSeconds); err != nil {
		s.logger.Error("Failed to save scan handoff record", "token", record.Token, "error", err)
		return fmt.Errorf("failed to save scan handoff record: %w", err)
	}

	return nil
}

// Get returns the record for token, or nil if it doesn't exist or has expired.
func (s *RedisScanHandoffStore) Get(ctx context.Context, token string) (*StagedScan, error) {
	conn := s.pool.Get()
	defer conn.Close()

	data, err := redis.Bytes(conn.Do("GET", redisKey(token)))
	if err == redis.ErrNil {
		return nil, nil
	}
	if err != nil {
		s.logger.Error("Failed to get scan handoff record", "token", token, "error", err)
		return nil, fmt.Errorf("failed to get scan handoff record: %w", err)
	}

	var record StagedScan
	if err := json.Unmarshal(data, &record); err != nil {
		s.logger.Error("Failed to unmarshal scan handoff record", "token", token, "error", err)
		return nil, fmt.Errorf("failed to unmarshal scan handoff record: %w", err)
	}

	return &record, nil
}

// Delete removes the record for token. It is not an error if the record doesn't exist.
func (s *RedisScanHandoffStore) Delete(ctx context.Context, token string) error {
	conn := s.pool.Get()
	defer conn.Close()

	if _, err := conn.Do("DEL", redisKey(token)); err != nil {
		s.logger.Error("Failed to delete scan handoff record", "token", token, "error", err)
		return fmt.Errorf("failed to delete scan handoff record: %w", err)
	}

	return nil
}
