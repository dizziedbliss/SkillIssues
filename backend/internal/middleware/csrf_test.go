package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCSRF(t *testing.T) {
	mw, err := CSRF("https://app.example", "sid")
	if err != nil {
		t.Fatal(err)
	}
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	do := func(method string, headers map[string]string, cookie bool) int {
		req := httptest.NewRequest(method, "https://api.example/saved", strings.NewReader("{}"))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if cookie {
			req.AddCookie(&http.Cookie{Name: "sid", Value: "x"})
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	tests := []struct {
		name    string
		method  string
		headers map[string]string
		cookie  bool
		want    int
	}{
		{"cross-site text/plain form post is refused", "POST",
			map[string]string{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site", "Content-Type": "text/plain"}, true, http.StatusForbidden},
		{"cross-site json post is refused", "POST",
			map[string]string{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site", "Content-Type": "application/json"}, true, http.StatusForbidden},
		{"trusted frontend origin with json passes", "POST",
			map[string]string{"Origin": "https://app.example", "Sec-Fetch-Site": "cross-site", "Content-Type": "application/json"}, true, http.StatusOK},
		{"same-origin cookie post without json is refused", "POST",
			map[string]string{"Sec-Fetch-Site": "same-origin", "Content-Type": "text/plain"}, true, http.StatusUnsupportedMediaType},
		{"same-origin cookie post with json charset passes", "POST",
			map[string]string{"Sec-Fetch-Site": "same-origin", "Content-Type": "application/json; charset=utf-8"}, true, http.StatusOK},
		{"cookie delete without json is refused", "DELETE", map[string]string{}, true, http.StatusUnsupportedMediaType},
		{"bearer post passes with any type", "POST",
			map[string]string{"Authorization": "Bearer t", "Content-Type": "text/plain"}, false, http.StatusOK},
		{"bearer post from a foreign origin passes", "POST",
			map[string]string{"Authorization": "Bearer t", "Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"}, false, http.StatusOK},
		{"no cookie, no bearer, json-less post passes (nothing to ride)", "POST", map[string]string{}, false, http.StatusOK},
		{"cross-site GET is not blocked", "GET",
			map[string]string{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"}, true, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := do(tc.method, tc.headers, tc.cookie); got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCSRFRejectsMalformedTrustedOrigin(t *testing.T) {
	if _, err := CSRF("not-an-origin", "sid"); err == nil {
		t.Fatal("want error for a malformed trusted origin")
	}
}
