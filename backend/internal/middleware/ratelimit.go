package middleware

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/cache"
)

// RateLimits is the live limit configuration for one request.
type RateLimits struct {
	Unauth int64
	Auth   int64
	Window time.Duration
}

// RateLimiter returns an HTTP middleware enforcing rate limits via Redis.
//
// Limits are read through the provider on every request rather than captured at
// construction, so they can be changed on a running system.
// If cache is nil or down, the middleware fails open.
func RateLimiter(c *cache.Client, limits func() RateLimits, keyPrefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c == nil || r.URL.Path == "/health" || r.URL.Path == "/health/detail" || r.URL.Path == "/metrics" || strings.HasPrefix(r.URL.Path, "/auth/") {
				next.ServeHTTP(w, r)
				return
			}

			uid := GetUserID(r.Context())
			isAuthenticated := uid > 0
			current := limits()

			var limit int64
			var rateLimitKey string
			window := current.Window

			if isAuthenticated {
				limit = current.Auth
				rateLimitKey = fmt.Sprintf("%s:u%d", keyPrefix, uid)
			} else {
				limit = current.Unauth
				clientIP := clientIdentifier(r)
				rateLimitKey = fmt.Sprintf("%s:anon:%s", keyPrefix, clientIP)
			}

			res := c.CheckRateLimit(r.Context(), rateLimitKey, limit, window)

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
			resetSecs := int64(res.ResetAfter.Seconds())
			if resetSecs <= 0 {
				resetSecs = int64(window.Seconds())
			}
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetSecs))

			if !res.Allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", resetSecs))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				msg := "Too many requests. Please try again later."
				if !isAuthenticated {
					msg = fmt.Sprintf("API rate limit exceeded for unauthenticated user (%d requests per %s limit). Please sign in with GitHub to increase your rate limit.", limit, window)
				}
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": msg,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// clientIdentifier resolves client IP for unauthenticated users
func clientIdentifier(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
