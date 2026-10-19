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

// ClientKey is the identity the rate limiter counts a request against: the
// user for a signed-in request, the client address otherwise. Other per-caller
// caps (the issue body fallback) key on it too, so one caller is one caller
// everywhere.
func ClientKey(r *http.Request, trustedProxies []*net.IPNet) string {
	if uid := GetUserID(r.Context()); uid > 0 {
		return fmt.Sprintf("u%d", uid)
	}
	return "anon:" + clientIdentifier(r, trustedProxies)
}

// clientIdentifier resolves the rate-limit identity of an unauthenticated caller.
//
// X-Forwarded-For is honoured only when the immediate peer (r.RemoteAddr) is a
// trusted proxy. It is then read from the right: each proxy appends the address
// it received the request from, so the rightmost entries are the ones our own
// proxies wrote, and the first entry that is not one of them is the client.
// Entries further left are whatever the client chose to send and are never
// trusted — taking the leftmost let a caller pick its own bucket by prepending
// a value. X-Real-IP is used only when there is no usable X-Forwarded-For.
//
// An IPv6 client is keyed by its /64: one subscriber is routinely handed a whole
// /64, so keying by the full address lets it mint a fresh bucket per request.
func clientIdentifier(r *http.Request, trustedProxies []*net.IPNet) string {
	peerHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peerHost = r.RemoteAddr
	}

	if isTrustedProxy(peerHost, trustedProxies) {
		parts := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
		for i := len(parts) - 1; i >= 0; i-- {
			entry := strings.TrimSpace(parts[i])
			ip := net.ParseIP(entry)
			if ip == nil {
				// A malformed entry is where the proxies' trustworthy part ends.
				break
			}
			if !isTrustedProxy(entry, trustedProxies) {
				return rateLimitSubject(ip)
			}
		}
		if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil && !isTrustedProxy(ip.String(), trustedProxies) {
			return rateLimitSubject(ip)
		}
	}

	if ip := net.ParseIP(peerHost); ip != nil {
		return rateLimitSubject(ip)
	}
	return peerHost
}

// rateLimitSubject is the string a client's bucket is keyed by: the address for
// IPv4, the /64 network for IPv6.
func rateLimitSubject(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

// ParseTrustedProxies parses CIDR strings. An invalid entry is an error: a
// mistyped proxy range would silently leave every request keyed by the
// proxy's own address, putting all users in one bucket.
func ParseTrustedProxies(cidrs []string) ([]*net.IPNet, error) {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, ipnet, err := net.ParseCIDR(strings.TrimSpace(c))
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES: %q is not a CIDR: %w", c, err)
		}
		out = append(out, ipnet)
	}
	return out, nil
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
