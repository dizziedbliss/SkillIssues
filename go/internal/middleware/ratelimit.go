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

// RateLimiter returns an HTTP middleware enforcing rate limits via Redis.
// If cache is nil or down, the middleware fails open.
func RateLimiter(c *cache.Client, limit int64, window time.Duration, keyPrefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c == nil || r.URL.Path == "/health" || r.URL.Path == "/health/detail" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			// Identify client: authenticated UserID preferred, otherwise client IP
			clientKey := clientIdentifier(r)
			rateLimitKey := fmt.Sprintf("%s:%s", keyPrefix, clientKey)

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
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "Too many requests. Please try again later.",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// clientIdentifier resolves client IP or user context
func clientIdentifier(r *http.Request) string {
	if uid := GetUserID(r.Context()); uid > 0 {
		return fmt.Sprintf("u%d", uid)
	}

	// Fallback to IP address
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
