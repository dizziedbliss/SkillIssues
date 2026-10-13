package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_AllowedOrigins(t *testing.T) {
	handler := CORS("https://skillissues.com")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	tests := []struct {
		name           string
		origin         string
		method         string
		expectedOrigin string
		expectedStatus int
	}{
		{
			name:           "localhost frontend origin",
			origin:         "http://localhost:5173",
			method:         "GET",
			expectedOrigin: "http://localhost:5173",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "127.0.0.1 frontend origin",
			origin:         "http://127.0.0.1:3000",
			method:         "GET",
			expectedOrigin: "http://127.0.0.1:3000",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "configured production origin",
			origin:         "https://skillissues.com",
			method:         "GET",
			expectedOrigin: "https://skillissues.com",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "preflight OPTIONS request from localhost",
			origin:         "http://localhost:5173",
			method:         "OPTIONS",
			expectedOrigin: "http://localhost:5173",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "unknown external origin defaults to configured frontendURL",
			origin:         "https://malicious-site.com",
			method:         "GET",
			expectedOrigin: "https://skillissues.com",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/me", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, rec.Code)
			}
			allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
			if allowOrigin != tc.expectedOrigin {
				t.Errorf("expected Access-Control-Allow-Origin %q, got %q", tc.expectedOrigin, allowOrigin)
			}
			if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
				t.Errorf("expected Access-Control-Allow-Credentials to be true")
			}
		})
	}
}

// A comma-separated FrontendURL must never be echoed back whole on the
// fallback path — a browser rejects a multi-value Access-Control-Allow-Origin
// on a credentialed request, so an unmatched origin must fall back to just
// the first configured entry.
func TestCORS_MultiValueFrontendURLFallsBackToFirstEntry(t *testing.T) {
	handler := CORS("https://a.skillissues.com,https://b.skillissues.com")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/me", nil)
	req.Header.Set("Origin", "https://malicious-site.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://a.skillissues.com" {
		t.Errorf("expected fallback to the first configured origin, got %q", got)
	}

	req2 := httptest.NewRequest("GET", "/me", nil)
	req2.Header.Set("Origin", "https://b.skillissues.com")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if got := rec2.Header().Get("Access-Control-Allow-Origin"); got != "https://b.skillissues.com" {
		t.Errorf("expected the second configured origin to be allowed, got %q", got)
	}
}
