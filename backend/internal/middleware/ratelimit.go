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
//
// trustedProxies lists the CIDRs permitted to set X-Forwarded-For/X-Real-IP;
// a request from any other peer has those headers ignored so a client can't
// spoof its own rate-limit identity. auth/health/metrics endpoints are not
// blanket-exempt — only /health, /health/detail and /metrics are, so PAT and
// OAuth login are rate limited like everything else.
func RateLimiter(c *cache.Client, limits func() RateLimits, keyPrefix string, trustedProxies []*net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c == nil || r.URL.Path == "/health" || r.URL.Path == "/health/detail" || r.URL.Path == "/metrics" {
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
				clientIP := clientIdentifier(r, trustedProxies)
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

// clientIdentifier resolves client IP for unauthenticated users.
//
// X-Forwarded-For/X-Real-IP are only honored when the immediate peer
// (r.RemoteAddr) is a trusted proxy; otherwise any client could set those
// headers itself and pick its own rate-limit bucket.
func clientIdentifier(r *http.Request, trustedProxies []*net.IPNet) string {
	peerHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peerHost = r.RemoteAddr
	}

	if isTrustedProxy(peerHost, trustedProxies) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	return peerHost
}

// ParseTrustedProxies parses CIDR strings (invalid entries are skipped).
func ParseTrustedProxies(cidrs []string) []*net.IPNet {
	var out []*net.IPNet
	for _, c := range cidrs {
		if _, ipnet, err := net.ParseCIDR(c); err == nil {
			out = append(out, ipnet)
		}
	}
	return out
}

func isTrustedProxy(host string, trusted []*net.IPNet) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, n := range trusted {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
