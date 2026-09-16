// Package auth provides session management, GitHub OAuth PKCE flow,
// and PAT (Personal Access Token) authentication.
//
// Authentication strategy (identical to Python implementation):
//   - Raw session token: 32 cryptographically random bytes, base64url encoded
//   - Stored token: SHA-256 hash of the raw token (prevents DB compromise revealing live tokens)
//   - Transport: "Authorization: Bearer <raw token>" header (Electron) OR
//     "skillissues_session" httponly cookie (web frontend)
//   - Expiry: 7 days, enforced at DB level via expires_at column
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/dizziedbliss/skillissues/internal/cache"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

// SessionDuration is how long a session stays valid after creation.
const SessionDuration = 7 * 24 * time.Hour

// ErrUnauthorized is returned when a session token is invalid or expired.
var ErrUnauthorized = errors.New("auth: unauthorized")

// Service handles session lifecycle and authentication.
type Service struct {
	db    *db.Pool
	cache *cache.Client
}

// New creates an auth.Service.
func New(db *db.Pool, cacheClient ...*cache.Client) *Service {
	s := &Service{db: db}
	if len(cacheClient) > 0 {
		s.cache = cacheClient[0]
	}
	return s
}

// ---------------------------------------------------------------------------
// Session management
// ---------------------------------------------------------------------------

// CreateSession generates a new session for the given user and stores it.
// Returns the raw (unhashed) token to be sent to the client.
func (s *Service) CreateSession(ctx context.Context, userID int, provider string) (string, error) {
	raw, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("auth: generate token: %w", err)
	}

	hashed := hashToken(raw)

	err = s.db.Exec(ctx,
		`INSERT INTO auth_sessions (token, user_id, provider, expires_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (token) DO NOTHING`,
		hashed, userID, provider, time.Now().Add(SessionDuration),
	)
	if err != nil {
		return "", fmt.Errorf("auth: store session: %w", err)
	}

	if s.cache != nil {
		s.cache.SetSession(ctx, hashed, userID, SessionDuration)
	}

	return raw, nil
}

// ValidateSession looks up a raw session token and returns the owning user ID.
// Returns ErrUnauthorized if the token is missing, expired, or invalid.
func (s *Service) ValidateSession(ctx context.Context, rawToken string) (int, error) {
	hashed := hashToken(rawToken)

	// Redis Fast Path (< 1ms)
	if s.cache != nil {
		if cachedUID := s.cache.GetSession(ctx, hashed); cachedUID > 0 {
			return cachedUID, nil
		}
	}

	var userID int
	err := s.db.QueryRow(ctx,
		`SELECT user_id FROM auth_sessions
		 WHERE token = $1 AND expires_at > now()`,
		[]any{hashed},
		func(row pgx.Row) error {
			return row.Scan(&userID)
		},
	)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return 0, ErrUnauthorized
		}
		return 0, fmt.Errorf("auth: validate session: %w", err)
	}

	if s.cache != nil {
		s.cache.SetSession(ctx, hashed, userID, SessionDuration)
	}

	return userID, nil
}

// DeleteSession invalidates a single session (logout).
func (s *Service) DeleteSession(ctx context.Context, rawToken string) error {
	hashed := hashToken(rawToken)
	if s.cache != nil {
		s.cache.DeleteSession(ctx, hashed)
	}
	return s.db.Exec(ctx,
		`DELETE FROM auth_sessions WHERE token = $1`,
		hashed,
	)
}

// DeleteAllSessions invalidates all sessions for a user (purge/account deletion).
func (s *Service) DeleteAllSessions(ctx context.Context, userID int) error {
	return s.db.Exec(ctx,
		`DELETE FROM auth_sessions WHERE user_id = $1`,
		userID,
	)
}

// HashedToken returns the stored (hashed) form of a raw token.
// Exported for use by the cache layer which keys on the hashed form.
func HashedToken(rawToken string) string {
	return hashToken(rawToken)
}

// ---------------------------------------------------------------------------
// Token helpers
// ---------------------------------------------------------------------------

// generateToken creates a cryptographically random 32-byte URL-safe base64 token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken computes the SHA-256 digest of a raw token, returned as a lowercase hex string.
// This matches the Python implementation: hashlib.sha256(token.encode()).hexdigest()
func hashToken(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// Webhook HMAC verification
// ---------------------------------------------------------------------------

// VerifyWebhookSignature verifies a GitHub webhook payload using the HMAC-SHA256
// signature in the X-Hub-Signature-256 header.
//
// sigHeader format: "sha256=<hex digest>"
// Matches Python: hmac.new(secret, body, sha256).hexdigest()
func VerifyWebhookSignature(secret, body []byte, sigHeader string) bool {
	const prefix = "sha256="
	if len(sigHeader) <= len(prefix) {
		return false
	}

	got, err := hex.DecodeString(sigHeader[len(prefix):])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	expected := mac.Sum(nil)

	return hmac.Equal(got, expected)
}

// ---------------------------------------------------------------------------
// User upsert helper (used by both OAuth and PAT auth flows)
// ---------------------------------------------------------------------------

// UpsertUser inserts or updates a GitHub user record.
// Returns the database user ID.
func (s *Service) UpsertUser(ctx context.Context, githubID, username, avatarURL, accessToken string) (int, error) {
	var userID int
	err := s.db.ExecReturning(ctx,
		`INSERT INTO users (github_id, username, avatar_url, github_access_token)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (github_id) DO UPDATE
		   SET username            = EXCLUDED.username,
		       avatar_url          = EXCLUDED.avatar_url,
		       github_access_token = EXCLUDED.github_access_token
		 RETURNING id`,
		[]any{githubID, username, avatarURL, accessToken},
		func(row pgx.Row) error {
			return row.Scan(&userID)
		},
	)
	if err != nil {
		return 0, fmt.Errorf("auth: upsert user: %w", err)
	}

	// Ensure developer_profiles and developer_preferences rows exist (idempotent).
	_ = s.db.Exec(ctx,
		`INSERT INTO developer_profiles (user_id) VALUES ($1) ON CONFLICT DO NOTHING`,
		userID,
	)
	_ = s.db.Exec(ctx,
		`INSERT INTO developer_preferences (user_id) VALUES ($1) ON CONFLICT DO NOTHING`,
		userID,
	)

	return userID, nil
}
