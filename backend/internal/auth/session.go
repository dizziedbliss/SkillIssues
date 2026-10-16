// Package auth provides session management and the GitHub sign-in flows.
//
// The server never stores a GitHub credential: sign-in reads the user's
// numeric GitHub id, login and avatar, then discards the token it used to
// ask. A SkillIssues session is the only thing that identifies a user here.
//
// Authentication strategy:
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

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

// SessionDuration is the built-in session lifetime, used when no provider is
// configured via SetSessionTTL.
const SessionDuration = 7 * 24 * time.Hour

// SetSessionTTL installs a provider for the session lifetime so it can be
// retuned without a restart. Safe to call only during startup wiring.
func (s *Service) SetSessionTTL(ttl func() time.Duration) {
	if ttl != nil {
		s.sessionTTL = ttl
	}
}

// ttl resolves the configured session lifetime, falling back to the built-in
// value if a provider yields something nonsensical.
func (s *Service) ttl() time.Duration {
	if s.sessionTTL == nil {
		return SessionDuration
	}
	if d := s.sessionTTL(); d > 0 {
		return d
	}
	return SessionDuration
}

// ErrUnauthorized is returned when a session token is invalid or expired.
var ErrUnauthorized = errors.New("auth: unauthorized")

// Service handles session lifecycle and authentication.
type Service struct {
	db *db.Pool

	// sessionTTL is read when a session is created. Changing it affects newly
	// issued sessions only; existing ones keep the expiry they were given.
	sessionTTL func() time.Duration
}

// New creates an auth.Service. Auth has no Redis dependency (D11) — it
// never touched the cache field for anything but sessions, which now go
// straight to Postgres.
func New(db *db.Pool) *Service {
	return &Service{db: db, sessionTTL: func() time.Duration { return SessionDuration }}
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
		hashed, userID, provider, time.Now().Add(s.ttl()),
	)
	if err != nil {
		return "", fmt.Errorf("auth: store session: %w", err)
	}

	return raw, nil
}

// ValidateSession looks up a raw session token and returns the owning user ID.
// Returns ErrUnauthorized if the token is missing, expired, or invalid.
//
// Goes straight to Postgres (D11) — a lookup on a hashed-token primary key
// is already fast, and this is what makes "sessions survive a Redis
// outage" structural rather than accidental: auth touches Redis zero times.
func (s *Service) ValidateSession(ctx context.Context, rawToken string) (int, error) {
	hashed := hashToken(rawToken)

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

	return userID, nil
}

// DeleteSession invalidates a single session (logout).
func (s *Service) DeleteSession(ctx context.Context, rawToken string) error {
	hashed := hashToken(rawToken)
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

// PruneExpired deletes sessions past their expiry and OAuth handshakes that
// were never completed, returning how many of each it removed. Expired
// sessions are already refused at validation; this only keeps the tables from
// growing without bound.
func (s *Service) PruneExpired(ctx context.Context) (sessions, states int64, err error) {
	tag, err := s.db.Raw().Exec(ctx, `DELETE FROM auth_sessions WHERE expires_at <= now()`)
	if err != nil {
		return 0, 0, fmt.Errorf("auth: prune sessions: %w", err)
	}
	sessions = tag.RowsAffected()
	tag, err = s.db.Raw().Exec(ctx, `DELETE FROM oauth_states WHERE expires_at <= now()`)
	if err != nil {
		return sessions, 0, fmt.Errorf("auth: prune oauth states: %w", err)
	}
	return sessions, tag.RowsAffected(), nil
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
// User upsert
// ---------------------------------------------------------------------------

// ErrInvalidGitHubIdentity means a sign-in produced no usable GitHub id.
var ErrInvalidGitHubIdentity = errors.New("auth: GitHub identity has no numeric id")

// UpsertUser inserts or updates the user identified by githubID and returns
// the database user id.
//
// The numeric GitHub id is the only identity. A username is display data
// that GitHub lets another account claim after a rename, so it is never used
// to find an existing user: matching on it would hand one person's account
// to whoever later takes their old login. If another row still holds this
// username (case-insensitively) under a different GitHub id, that row's
// username is cleared first — the unique index on lower(username) would
// otherwise reject the write — and is refreshed the next time its owner
// signs in.
func (s *Service) UpsertUser(ctx context.Context, githubID, username, avatarURL string) (int, error) {
	if githubID == "" || githubID == "0" {
		return 0, ErrInvalidGitHubIdentity
	}
	if username == "" {
		return 0, fmt.Errorf("auth: GitHub login is empty for id %s", githubID)
	}

	var userID int
	err := s.db.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`UPDATE users SET username = NULL
			 WHERE lower(username) = lower($1) AND github_id <> $2`,
			username, githubID,
		); err != nil {
			return fmt.Errorf("release username: %w", err)
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO users (github_id, username, avatar_url)
			 VALUES ($1, $2, NULLIF($3, ''))
			 ON CONFLICT (github_id) DO UPDATE
			   SET username   = EXCLUDED.username,
			       avatar_url = COALESCE(EXCLUDED.avatar_url, users.avatar_url)
			 RETURNING id`,
			githubID, username, avatarURL,
		).Scan(&userID); err != nil {
			return fmt.Errorf("upsert user: %w", err)
		}
		// Idempotent: the profile and preference rows every user has.
		if _, err := tx.Exec(ctx,
			`INSERT INTO developer_profiles (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, userID,
		); err != nil {
			return fmt.Errorf("create profile: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO developer_preferences (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, userID,
		); err != nil {
			return fmt.Errorf("create preferences: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("auth: %w", err)
	}
	return userID, nil
}
