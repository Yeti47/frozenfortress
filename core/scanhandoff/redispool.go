package scanhandoff

import (
	"time"

	"github.com/gomodule/redigo/redis"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// newRedisPool builds a redigo connection pool using the same FF_REDIS_* configuration
// as the session store (see core/auth.CreateRedisStore), for direct (non-session) Redis
// access. Shared by RedisScanHandoffStore and RedisScanKeyStore.
func newRedisPool(config ccc.AppConfig) *redis.Pool {
	return &redis.Pool{
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
}
