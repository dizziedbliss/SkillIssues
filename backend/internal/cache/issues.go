package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	hotIssuesKey   = "issues:hot"
	searchKeyPrefix = "issues:search:"
)

// SetHotIssues caches the default hot issues payload.
func (c *Client) SetHotIssues(ctx context.Context, data []byte, ttl time.Duration) {
	if c == nil || c.rdb == nil {
		return
	}
	_ = c.rdb.Set(ctx, hotIssuesKey, data, ttl).Err()
}

// GetHotIssues returns the cached hot issues payload or nil on miss.
func (c *Client) GetHotIssues(ctx context.Context) []byte {
	if c == nil || c.rdb == nil {
		return nil
	}
	data, err := c.rdb.Get(ctx, hotIssuesKey).Bytes()
	if err != nil {
		return nil
	}
	return data
}

// InvalidateHotIssues purges cached issue search and hot issue payloads.
func (c *Client) InvalidateHotIssues(ctx context.Context) {
	if c == nil || c.rdb == nil {
		return
	}
	_ = c.rdb.Del(ctx, hotIssuesKey).Err()
}

// CacheSearchKey creates a deterministic key for an issue search query.
func CacheSearchKey(difficulty, language, query string, limit, page int) string {
	raw := fmt.Sprintf("d:%s|l:%s|q:%s|lim:%d|p:%d", difficulty, language, query, limit, page)
	hash := sha256.Sum256([]byte(raw))
	return searchKeyPrefix + hex.EncodeToString(hash[:8])
}

// SetCachedSearch stores serialized search results.
func (c *Client) SetCachedSearch(ctx context.Context, key string, data []byte, ttl time.Duration) {
	if c == nil || c.rdb == nil {
		return
	}
	_ = c.rdb.Set(ctx, key, data, ttl).Err()
}

// GetCachedSearch retrieves serialized search results.
func (c *Client) GetCachedSearch(ctx context.Context, key string) []byte {
	if c == nil || c.rdb == nil {
		return nil
	}
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil
	}
	return data
}
