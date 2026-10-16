package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxBody(t *testing.T) {
	var readErr error
	h := MaxBody(func() int64 { return 10 })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("within the cap", func(t *testing.T) {
		readErr = nil
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789")))
		if rec.Code != http.StatusOK || readErr != nil {
			t.Fatalf("code=%d readErr=%v, want 200 and no error", rec.Code, readErr)
		}
	})

	t.Run("declared length over the cap is refused unread", func(t *testing.T) {
		readErr = nil
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789ab")))
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("code=%d, want 413", rec.Code)
		}
	})

	t.Run("undeclared length over the cap fails the read", func(t *testing.T) {
		readErr = nil
		req := httptest.NewRequest(http.MethodPost, "/", io.NopCloser(strings.NewReader("0123456789ab")))
		req.ContentLength = -1
		h.ServeHTTP(httptest.NewRecorder(), req)
		var tooLarge *http.MaxBytesError
		if !errors.As(readErr, &tooLarge) {
			t.Fatalf("readErr=%v, want *http.MaxBytesError", readErr)
		}
	})
}
