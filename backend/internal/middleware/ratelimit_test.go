package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIdentifier(t *testing.T) {
	proxies, err := ParseTrustedProxies([]string{"172.28.0.0/24", "::1/128"})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, remote, xff, realIP, want string
	}{
		{"direct peer, headers ignored", "203.0.113.9:5000", "198.51.100.1", "198.51.100.2", "203.0.113.9"},
		{"proxy appends the client", "172.28.0.5:1", "203.0.113.7", "", "203.0.113.7"},
		{"spoofed prefix is ignored", "172.28.0.5:1", "1.2.3.4, 203.0.113.7", "", "203.0.113.7"},
		{"trusted hop skipped", "172.28.0.5:1", "203.0.113.7, 172.28.0.9", "", "203.0.113.7"},
		{"all hops trusted falls back to X-Real-IP", "172.28.0.5:1", "172.28.0.9", "203.0.113.8", "203.0.113.8"},
		{"all hops trusted, nothing else, uses the peer", "172.28.0.5:1", "172.28.0.9", "", "172.28.0.5"},
		{"garbage entry stops the walk", "172.28.0.5:1", "9.9.9.9, not-an-ip", "", "172.28.0.5"},
		{"ipv6 client keyed by /64", "172.28.0.5:1", "2001:db8:1:2:aaaa:bbbb:cccc:dddd", "", "2001:db8:1:2::/64"},
		{"same /64, different host, same bucket", "172.28.0.5:1", "2001:db8:1:2:1::9", "", "2001:db8:1:2::/64"},
		{"ipv6 direct peer keyed by /64", "[2001:db8:9:9::7]:80", "", "", "2001:db8:9:9::/64"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.remote
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.realIP != "" {
				r.Header.Set("X-Real-IP", tc.realIP)
			}
			if got := clientIdentifier(r, proxies); got != tc.want {
				t.Errorf("clientIdentifier = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseTrustedProxiesRejectsBadCIDR(t *testing.T) {
	if _, err := ParseTrustedProxies([]string{"127.0.0.1/32", "172.28.0.0"}); err == nil {
		t.Fatal("expected an error for an entry that is not a CIDR")
	}
}

func TestExempt(t *testing.T) {
	cases := []struct {
		method, path string
		want         bool
	}{
		{http.MethodGet, "/health", true},
		{http.MethodGet, "/metrics", true},
		{http.MethodGet, "/stats", true},
		{http.MethodHead, "/stats", true},
		{http.MethodPost, "/stats", false},
		{http.MethodGet, "/stats/", false},
		{http.MethodGet, "/issues", false},
		{http.MethodGet, "/auth/github", false},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		if got := exempt(r); got != tc.want {
			t.Errorf("exempt(%s %s) = %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}
