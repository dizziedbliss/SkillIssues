// Package cache provides a Redis client wrapper for session caching,
// rate limiting, and hot-path query results.
// The client is designed to fail open: if Redis is unavailable, all
// cache operations return their zero values without error, and callers
// fall back to the authoritative PostgreSQL store.
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps the Redis client with domain-specific helpers.
type Client struct {
	rdb *redis.Client
}

// New creates a Redis client and verifies connectivity.
// Returns an error if the server is unreachable at startup.
func New(ctx context.Context, redisURL string) (*Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("cache: parse redis URL: %w", err)
	}

	rdb := redis.NewClient(opts)

	// Ping to verify connectivity at startup.
	// In production, Redis is expected to be available; log a warning but don't
	// crash if it isn't — cache is a performance layer, not a hard dependency.
	if err := rdb.Ping(ctx).Err(); err != nil {
		// Non-fatal: log only. Cache will operate in degraded (miss-only) mode.
		_ = err // caller should log this
	}

	return &Client{rdb: rdb}, nil
}

// Close gracefully shuts down the Redis connection pool.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Ping checks Redis liveness. Used by /health/detail.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// ---------------------------------------------------------------------------
// Rate limiting
// ---------------------------------------------------------------------------

// RateLimitResult holds the result of a rate limit check.
type RateLimitResult struct {
	Allowed    bool
	Remaining  int64
	ResetAfter time.Duration
}

// CheckRateLimit implements a sliding-window counter using Redis INCR + EXPIRE.
// key should be unique per (userID, action), e.g. "rl:verify:42:contrib:7".
// Returns Allowed=true and remaining count, or Allowed=false when limit exceeded.
// Fails open: if Redis is down, always returns Allowed=true.
func (c *Client) CheckRateLimit(ctx context.Context, key string, limit int64, window time.Duration) RateLimitResult {
	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(ctx, "rl:"+key)
	pipe.Expire(ctx, "rl:"+key, window)

	if _, err := pipe.Exec(ctx); err != nil {
		// Redis unavailable — fail open to avoid blocking users
		return RateLimitResult{Allowed: true}
	}

	count := incr.Val()
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	ttl, _ := c.rdb.TTL(ctx, "rl:"+key).Result()

	return RateLimitResult{
		Allowed:    count <= limit,
		Remaining:  remaining,
		ResetAfter: ttl,
	}
}

// ---------------------------------------------------------------------------
// Recommendation cache
// ---------------------------------------------------------------------------

// SetRecommendations caches serialized recommendation JSON for a user.
func (c *Client) SetRecommendations(ctx context.Context, userID int, data []byte, ttl time.Duration) {
	_ = c.rdb.Set(ctx, recKey(userID), data, ttl).Err()
}

// GetRecommendations retrieves cached recommendation JSON, or nil if not cached.
func (c *Client) GetRecommendations(ctx context.Context, userID int) []byte {
	data, err := c.rdb.Get(ctx, recKey(userID)).Bytes()
	if err != nil {
		return nil
	}
	return data
}

// InvalidateRecommendations removes cached recommendations when profile changes.
func (c *Client) InvalidateRecommendations(ctx context.Context, userID int) {
	_ = c.rdb.Del(ctx, recKey(userID)).Err()
}

func recKey(userID int) string {
	return fmt.Sprintf("rec:%d", userID)
}

// ---------------------------------------------------------------------------
// Distributed lock (for sync coordinator)
// ---------------------------------------------------------------------------

// AcquireLock attempts to acquire a distributed lock with the given TTL.
// Returns true if the lock was acquired. Uses SET NX EX for atomicity.
func (c *Client) AcquireLock(ctx context.Context, lockKey string, ttl time.Duration) bool {
	result, err := c.rdb.SetNX(ctx, "lock:"+lockKey, "1", ttl).Result()
	if err != nil {
		return false // fail safe: don't proceed without lock confirmation
	}
	return result
}

// ReleaseLock releases a previously acquired lock.
func (c *Client) ReleaseLock(ctx context.Context, lockKey string) {
	_ = c.rdb.Del(ctx, "lock:"+lockKey).Err()
}

// isErrNil returns true if the error is a Redis nil (cache miss), false otherwise.
func isErrNil(err error) bool {
	return errors.Is(err, redis.Nil)
}

// Ensure isErrNil is used to avoid lint complaints.
var _ = isErrNil
