package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	hotIssuesKey    = "issues:hot"
	searchKeyPrefix = "issues:search:"
)

// SetHotIssues caches the default hot issues payload.
func (c *Client) SetHotIssues(ctx context.Context, data []byte, ttl time.Duration) {
	if c == nil || c.rdb == nil {
		return
	}
	c.set(ctx, hotIssuesKey, data, ttl)
}

// GetHotIssues returns the cached hot issues payload or nil on miss.
func (c *Client) GetHotIssues(ctx context.Context) []byte {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.get(ctx, hotIssuesKey)
}

// CacheSearchKey creates a deterministic key for one signed-out catalog or
// search request. cursor is the ?cursor= token and cursorMode whether the
// request had a ?cursor= at all (an empty one asks for the first page), since
// that changes the response's shape as well as its contents.
func CacheSearchKey(difficulty, language, query string, limit, page int, cursorMode bool, cursor string) string {
	raw := fmt.Sprintf("d:%s|l:%s|q:%s|lim:%d|p:%d|cm:%t|c:%s", difficulty, language, query, limit, page, cursorMode, cursor)
	hash := sha256.Sum256([]byte(raw))
	return searchKeyPrefix + hex.EncodeToString(hash[:8])
}

// SetCachedSearch stores serialized search results.
func (c *Client) SetCachedSearch(ctx context.Context, key string, data []byte, ttl time.Duration) {
	if c == nil || c.rdb == nil {
		return
	}
	c.set(ctx, key, data, ttl)
}

// GetCachedSearch retrieves serialized search results.
func (c *Client) GetCachedSearch(ctx context.Context, key string) []byte {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.get(ctx, key)
}
