// Package cache provides a Redis client wrapper for session caching,
// rate limiting, and hot-path query results.
// The client is designed to fail open: if Redis is unavailable, all
// cache operations return their zero values without error, and callers
// fall back to the authoritative PostgreSQL store.
//
// Failing open must also be fast. Every call carries a short deadline, and
// once a call fails the client stops contacting Redis for a while, letting
// one call through per interval to probe for recovery. Without both, each
// request would wait out the go-redis dial timeout and retries (~20s) before
// failing open, which is an outage in all but name.
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

// Limits bounds how long a request can spend on Redis. Read per call, so a
// settings change applies without a restart.
type Limits struct {
	// CallTimeout is the deadline for one Redis call, dial included.
	CallTimeout time.Duration
	// RetryAfter is how long Redis is skipped after a failed call before
	// one call is let through to check whether it has recovered.
	RetryAfter time.Duration
}

// Client wraps the Redis client with domain-specific helpers.
type Client struct {
	rdb    *redis.Client
	limits func() Limits

	// down tracks whether the last Redis call failed. While it is set, calls
	// skip Redis entirely (counted in cache_skipped_total) except one probe
	// per RetryAfter. It also means an outage is logged once when it starts
	// and once when it ends instead of on every request. Every failure is
	// still counted in cache_errors_total.
	down atomic.Bool
	// probeAt is when, in Unix nanoseconds, the next probe may go to Redis
	// while it is down. Claimed with a compare-and-swap, so concurrent
	// requests send one probe between them rather than one each.
	probeAt atomic.Int64
}

// New creates a Redis client. It does not contact the server: Redis is a
// performance layer, so the API starts without it, and the caller decides
// how to report a failed Ping.
func New(redisURL string, limits func() Limits) (*Client, error) {
	if limits == nil {
		return nil, errors.New("cache: limits must not be nil")
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("cache: parse redis URL: %w", err)
	}
	return &Client{rdb: redis.NewClient(opts), limits: limits}, nil
}

// Close gracefully shuts down the Redis connection pool.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Ping checks Redis liveness. Used at startup and by /health/detail. It
// always contacts Redis, even while calls are being skipped, so health
// reports what Redis is doing now rather than what it did last; a result
// is recorded like any other call, so a successful Ping also ends an outage.
func (c *Client) Ping(ctx context.Context) error {
	callCtx, cancel := context.WithTimeout(ctx, c.limits().CallTimeout)
	defer cancel()
	err := c.rdb.Ping(callCtx).Err()
	c.observe(ctx, "ping", err)
	return err
}

// begin decides whether a call may go to Redis. It returns a context bounded
// by the call timeout, or ok=false if Redis is down and it is not yet time
// for a probe, in which case the caller fails open without waiting.
func (c *Client) begin(ctx context.Context, op string) (context.Context, context.CancelFunc, bool) {
	lim := c.limits()
	if c.down.Load() {
		now := time.Now().UnixNano()
		at := c.probeAt.Load()
		if now < at || !c.probeAt.CompareAndSwap(at, now+lim.RetryAfter.Nanoseconds()) {
			metrics.CacheSkipped.WithLabelValues(op).Inc()
			return nil, nil, false
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, lim.CallTimeout)
	return callCtx, cancel, true
}

// observe records the outcome of a Redis call. A cache miss (redis.Nil) is
// a success. Callers still fail open; this only makes the failure visible
// and decides whether the following calls skip Redis.
//
// ctx is the caller's context, not the bounded call context: a call that
// failed because the request itself was cancelled says nothing about Redis,
// and must not mark it down for everyone else.
func (c *Client) observe(ctx context.Context, op string, err error) {
	if err == nil || errors.Is(err, redis.Nil) {
		if c.down.CompareAndSwap(true, false) {
			slog.Info("redis reachable again; cache and rate limiting restored")
		}
		return
	}
	if ctx.Err() != nil {
		return
	}
	metrics.CacheErrors.WithLabelValues(op).Inc()
	// Set the probe time before the flag, so no call sees down=true with a
	// probe time already in the past and lets a burst through.
	c.probeAt.Store(time.Now().Add(c.limits().RetryAfter).UnixNano())
	if c.down.CompareAndSwap(false, true) {
		slog.Warn("redis call failed; cache and rate limiting fail open until it recovers",
			"op", op, "error", err, "retry_after", c.limits().RetryAfter)
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
	callCtx, cancel, ok := c.begin(ctx, "rate_limit")
	if !ok {
		return RateLimitResult{Allowed: true}
	}
	defer cancel()

	k := "rl:" + key
	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(callCtx, k)
	pipe.ExpireNX(callCtx, k, window)
	ttl := pipe.TTL(callCtx, k)

	_, err := pipe.Exec(callCtx)
	c.observe(ctx, "rate_limit", err)
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
	c.set(ctx, recKey(userID), data, ttl)
}

// GetRecommendations retrieves cached recommendation JSON, or nil if not cached.
func (c *Client) GetRecommendations(ctx context.Context, userID int) []byte {
	return c.get(ctx, recKey(userID))
}

// InvalidateRecommendations removes cached recommendations when profile changes.
func (c *Client) InvalidateRecommendations(ctx context.Context, userID int) {
	c.del(ctx, recKey(userID))
}

func recKey(userID int) string {
	return fmt.Sprintf("rec:%d", userID)
}

// ---------------------------------------------------------------------------
// Primitives
// ---------------------------------------------------------------------------

// get returns the value at key, or nil on a miss, a failure, or while Redis
// is being skipped.
func (c *Client) get(ctx context.Context, key string) []byte {
	callCtx, cancel, ok := c.begin(ctx, "get")
	if !ok {
		return nil
	}
	defer cancel()
	data, err := c.rdb.Get(callCtx, key).Bytes()
	c.observe(ctx, "get", err)
	if err != nil {
		return nil
	}
	return data
}

func (c *Client) set(ctx context.Context, key string, data []byte, ttl time.Duration) {
	callCtx, cancel, ok := c.begin(ctx, "set")
	if !ok {
		return
	}
	defer cancel()
	c.observe(ctx, "set", c.rdb.Set(callCtx, key, data, ttl).Err())
}

func (c *Client) del(ctx context.Context, key string) {
	callCtx, cancel, ok := c.begin(ctx, "del")
	if !ok {
		return
	}
	defer cancel()
	c.observe(ctx, "del", c.rdb.Del(callCtx, key).Err())
}
