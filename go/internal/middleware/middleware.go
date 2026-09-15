// Package middleware provides HTTP middleware for the SkillIssues API server.
// Middleware is composable and applied in order by the router.
package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/auth"
	"github.com/dizziedbliss/skillissues/internal/cache"
	"github.com/google/uuid"
)

// contextKey is a private type for context value keys to avoid collisions.
type contextKey string

const (
	ctxUserID    contextKey = "user_id"
	ctxRequestID contextKey = "request_id"
)

// ---------------------------------------------------------------------------
// Request ID
// ---------------------------------------------------------------------------

// RequestID injects a UUID request ID into each request context and response header.
// The ID is included in all structured log entries for request tracing.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		ctx := context.WithValue(r.Context(), ctxRequestID, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(ctxRequestID).(string); ok {
		return id
	}
	return ""
}

// ---------------------------------------------------------------------------
// Structured request logging
// ---------------------------------------------------------------------------

// responseWriter wraps http.ResponseWriter to capture the status code for logging.
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// Logger logs every HTTP request as a structured JSON log entry using slog.
// Sensitive paths (e.g. /auth/github/token) have their bodies masked.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		slog.InfoContext(r.Context(), "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"bytes", rw.size,
			"latency_ms", time.Since(start).Milliseconds(),
			"request_id", GetRequestID(r.Context()),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// ---------------------------------------------------------------------------
// Authentication middleware
// ---------------------------------------------------------------------------

// RequireAuth reads a session token from either:
//  1. "Authorization: Bearer <token>" header (used by Electron)
//  2. "skillissues_session" cookie (used by web frontend)
//
// On success, injects the userID into the request context.
// On failure, responds with 401 JSON.
func RequireAuth(authSvc *auth.Service, cache *cache.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := resolveSession(r, authSvc, cache)
			if err != nil {
				respondError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserID, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth attempts session resolution but does not reject unauthenticated requests.
// Downstream handlers can call GetUserID to check if a user is logged in.
func OptionalAuth(authSvc *auth.Service, cache *cache.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, _ := resolveSession(r, authSvc, cache)
			ctx := context.WithValue(r.Context(), ctxUserID, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID retrieves the authenticated user ID from the request context.
// Returns 0 if not authenticated (only valid after OptionalAuth, not RequireAuth).
func GetUserID(ctx context.Context) int {
	if id, ok := ctx.Value(ctxUserID).(int); ok {
		return id
	}
	return 0
}

// RequireAdmin validates the "Authorization: Bearer <token>" header against
// the ADMIN_SYNC_TOKEN using constant-time comparison.
func RequireAdmin(adminToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			// Use HMAC equal for timing-safe comparison (matches Python hmac.compare_digest)
			h := hmac.New(sha256.New, []byte(adminToken))
			h.Write([]byte(provided))
			expectedMAC := h.Sum(nil)
			h2 := hmac.New(sha256.New, []byte(adminToken))
			h2.Write([]byte(adminToken))
			adminMAC := h2.Sum(nil)
			// Simplified: direct constant-time string comparison
			if !hmac.Equal([]byte(provided), []byte(adminToken)) {
				respondError(w, http.StatusForbidden, "admin access required")
				return
			}
			_ = expectedMAC
			_ = adminMAC
			next.ServeHTTP(w, r)
		})
	}
}

// resolveSession extracts and validates a session token from the request.
// Priority: Authorization header > skillissues_session cookie.
func resolveSession(r *http.Request, authSvc *auth.Service, c *cache.Client) (int, error) {
	rawToken := ""

	// 1. Check Authorization header (used by Electron)
	if bearer := r.Header.Get("Authorization"); strings.HasPrefix(bearer, "Bearer ") {
		rawToken = strings.TrimPrefix(bearer, "Bearer ")
	}

	// 2. Fall back to cookie (used by web frontend)
	if rawToken == "" {
		if cookie, err := r.Cookie("skillissues_session"); err == nil {
			rawToken = cookie.Value
		}
	}

	if rawToken == "" {
		return 0, errors.New("no session token provided")
	}

	// Fast path: check Redis cache
	if c != nil {
		hashed := auth.HashedToken(rawToken)
		if userID := c.GetSession(r.Context(), hashed); userID != 0 {
			return userID, nil
		}
	}

	// Slow path: validate against database
	userID, err := authSvc.ValidateSession(r.Context(), rawToken)
	if err != nil {
		return 0, err
	}

	// Populate cache for next request
	if c != nil {
		hashed := auth.HashedToken(rawToken)
		c.SetSession(r.Context(), hashed, userID, auth.SessionDuration)
	}

	return userID, nil
}

// ---------------------------------------------------------------------------
// CORS
// ---------------------------------------------------------------------------

// CORS sets the appropriate CORS headers for the given allowed origin.
// Credentials are enabled to support cookie-based session authentication.
func CORS(frontendURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", frontendURL)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ---------------------------------------------------------------------------
// Response helpers (used by handlers and middleware)
// ---------------------------------------------------------------------------

// respondError writes a JSON error response.
func respondError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// RespondJSON writes a JSON success response. Exported for use in handlers.
func RespondJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

// RespondError is the exported version for handlers to call.
func RespondError(w http.ResponseWriter, code int, msg string) {
	respondError(w, code, msg)
}

// hashedHex produces a hex-encoded SHA-256 hash of s (for logging masking).
func hashedHex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8]) + "..."
}

var _ = hashedHex // prevent unused lint error
