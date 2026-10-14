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
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/redis/go-redis/v9"
)

// Client wraps the Redis client with domain-specific helpers.
type Client struct {
	rdb *redis.Client

	// down tracks whether the last Redis call failed, so an outage is
	// logged once when it starts and once when it ends instead of on every
	// request. Every failure is still counted in cache_errors_total.
	down atomic.Bool
}

// New creates a Redis client. It does not contact the server: Redis is a
// performance layer, so the API starts without it, and the caller decides
// how to report a failed Ping.
func New(redisURL string) (*Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("cache: parse redis URL: %w", err)
	}
	return &Client{rdb: redis.NewClient(opts)}, nil
}

// Close gracefully shuts down the Redis connection pool.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Ping checks Redis liveness. Used at startup and by /health/detail.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// observe records the outcome of a Redis call. A cache miss (redis.Nil) is
// a success. Callers still fail open; this only makes the failure visible.
func (c *Client) observe(op string, err error) {
	if err == nil || errors.Is(err, redis.Nil) {
		if c.down.CompareAndSwap(true, false) {
			slog.Info("redis reachable again; cache and rate limiting restored")
		}
		return
	}
	metrics.CacheErrors.WithLabelValues(op).Inc()
	if c.down.CompareAndSwap(false, true) {
		slog.Warn("redis call failed; cache and rate limiting fail open until it recovers", "op", op, "error", err)
	}
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

// CheckRateLimit is a fixed-window counter: the first request in a window
// creates the key and starts its expiry, later ones only INCR it. The expiry
// is set with NX, so it is never pushed back by later requests — the count
// resets `window` after the first request, however busy the client is. NX
// also repairs a key left without an expiry.
//
// Fails open: if Redis is down, always returns Allowed=true.
func (c *Client) CheckRateLimit(ctx context.Context, key string, limit int64, window time.Duration) RateLimitResult {
	k := "rl:" + key
	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(ctx, k)
	pipe.ExpireNX(ctx, k, window)
	ttl := pipe.TTL(ctx, k)

	_, err := pipe.Exec(ctx)
	c.observe("rate_limit", err)
	if err != nil {
		return RateLimitResult{Allowed: true}
	}

	count := incr.Val()
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	return RateLimitResult{
		Allowed:    count <= limit,
		Remaining:  remaining,
		ResetAfter: ttl.Val(),
	}
}

// ---------------------------------------------------------------------------
// Recommendation cache
// ---------------------------------------------------------------------------

// SetRecommendations caches serialized recommendation JSON for a user.
func (c *Client) SetRecommendations(ctx context.Context, userID int, data []byte, ttl time.Duration) {
	c.observe("set", c.rdb.Set(ctx, recKey(userID), data, ttl).Err())
}

// GetRecommendations retrieves cached recommendation JSON, or nil if not cached.
func (c *Client) GetRecommendations(ctx context.Context, userID int) []byte {
	data, err := c.rdb.Get(ctx, recKey(userID)).Bytes()
	c.observe("get", err)
	if err != nil {
		return nil
	}
	return data
}

// InvalidateRecommendations removes cached recommendations when profile changes.
func (c *Client) InvalidateRecommendations(ctx context.Context, userID int) {
	c.observe("del", c.rdb.Del(ctx, recKey(userID)).Err())
}

func recKey(userID int) string {
	return fmt.Sprintf("rec:%d", userID)
}
