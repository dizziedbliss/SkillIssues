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

	"github.com/dizziedbliss/skillissues/internal/crypto"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/githubapp"
	"github.com/jackc/pgx/v5"
)

// refreshMargin is how much life a GitHub App user token must have left
// before GetAccessToken proactively refreshes it, so a caller never
// receives a token that expires mid-request.
const refreshMargin = 2 * time.Minute

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

	// tokenCipher encrypts/decrypts github_access_token before it reaches
	// Postgres. Nil only in tests that don't set it, in which case tokens
	// pass through unencrypted.
	tokenCipher *crypto.Cipher

	// githubApp refreshes GitHub-App-issued user tokens before they expire.
	// Nil when the GitHub App isn't configured, in which case stored tokens
	// are assumed to be classic OAuth App tokens, which don't expire.
	githubApp *githubapp.App
}

// New creates an auth.Service. Auth has no Redis dependency (D11) — it
// never touched the cache field for anything but sessions, which now go
// straight to Postgres.
func New(db *db.Pool) *Service {
	return &Service{db: db, sessionTTL: func() time.Duration { return SessionDuration }}
}

// SetTokenCipher installs the cipher used to encrypt/decrypt stored GitHub
// access tokens. Safe to call only during startup wiring.
func (s *Service) SetTokenCipher(c *crypto.Cipher) {
	s.tokenCipher = c
}

// SetGitHubApp installs the GitHub App used to refresh expiring
// user-to-server tokens. Safe to call only during startup wiring.
func (s *Service) SetGitHubApp(app *githubapp.App) {
	s.githubApp = app
}

// ErrGitHubReauthRequired means the user has stored GitHub authorization but
// it can no longer be used: it does not decrypt under the current
// TOKEN_ENCRYPTION_KEY, or GitHub rejected its refresh token. The user must
// connect GitHub again; retrying will not help.
var ErrGitHubReauthRequired = errors.New("auth: GitHub authorization must be renewed")

// decryptToken reverses the encryption UpsertUser applies. A value that
// cannot be decrypted is an error, never handed back as if it were a token.
func (s *Service) decryptToken(encrypted string) (string, error) {
	if encrypted == "" || s.tokenCipher == nil {
		return encrypted, nil
	}
	return s.tokenCipher.Decrypt(encrypted)
}

// GetAccessToken fetches and decrypts a user's stored GitHub token. If it's a
// GitHub-App-issued token nearing or past its 8-hour expiry, this refreshes it
// first via the stored refresh token and persists the result — callers never
// receive a token that's about to stop working mid-request.
//
// It returns ("", nil) when the user never connected GitHub,
// ErrGitHubReauthRequired when the stored authorization is unusable, and any
// other error for a failure worth retrying (database, network, GitHub outage).
func (s *Service) GetAccessToken(ctx context.Context, userID int) (string, error) {
	var encToken, encRefresh string
	var expiresAt *time.Time
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(github_access_token, ''), COALESCE(github_refresh_token, ''), github_token_expires_at
		 FROM users WHERE id = $1`,
		[]any{userID},
		func(row pgx.Row) error { return row.Scan(&encToken, &encRefresh, &expiresAt) },
	)
	if err != nil {
		return "", fmt.Errorf("auth: load GitHub token: %w", err)
	}

	if expiresAt == nil || time.Until(*expiresAt) > refreshMargin || encRefresh == "" || s.githubApp == nil {
		token, err := s.decryptToken(encToken)
		if err != nil {
			return "", fmt.Errorf("%w: access token does not decrypt: %v", ErrGitHubReauthRequired, err)
		}
		return token, nil
	}

	refreshToken, err := s.decryptToken(encRefresh)
	if err != nil {
		return "", fmt.Errorf("%w: refresh token does not decrypt: %v", ErrGitHubReauthRequired, err)
	}
	fresh, err := s.githubApp.RefreshUserToken(ctx, refreshToken)
	if errors.Is(err, githubapp.ErrTokenRejected) {
		return "", fmt.Errorf("%w: %v", ErrGitHubReauthRequired, err)
	}
	if err != nil {
		return "", fmt.Errorf("auth: refresh GitHub token: %w", err)
	}
	if err := s.storeUserToken(ctx, userID, fresh); err != nil {
		// GitHub has already rotated the refresh token, so the stored one is
		// now dead and the next refresh will ask the user to reconnect.
		// Surface that now instead of hiding it behind one working request.
		return "", fmt.Errorf("auth: persist refreshed GitHub token: %w", err)
	}
	return fresh.AccessToken, nil
}

// storeUserToken encrypts and persists a GitHub App user token (access
// token, refresh token, expiry) for userID.
func (s *Service) storeUserToken(ctx context.Context, userID int, tok *githubapp.UserToken) error {
	encToken, err := s.encryptToken(tok.AccessToken)
	if err != nil {
		return err
	}
	encRefresh, err := s.encryptToken(tok.RefreshToken)
	if err != nil {
		return err
	}
	return s.db.Exec(ctx,
		`UPDATE users SET github_access_token = $1, github_refresh_token = $2, github_token_expires_at = $3
		 WHERE id = $4`,
		encToken, encRefresh, tok.ExpiresAt, userID,
	)
}

func (s *Service) encryptToken(plain string) (string, error) {
	if plain == "" || s.tokenCipher == nil {
		return plain, nil
	}
	return s.tokenCipher.Encrypt(plain)
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

	// Encrypt before it ever reaches a query: github_access_token in Postgres
	// must never hold a usable plaintext token.
	if s.tokenCipher != nil && accessToken != "" {
		encrypted, err := s.tokenCipher.Encrypt(accessToken)
		if err != nil {
			return 0, fmt.Errorf("auth: encrypt access token: %w", err)
		}
		accessToken = encrypted
	}

	// First, attempt to match an existing user record by GitHub ID or case-insensitive username
	var existingID int
	err := s.db.QueryRow(ctx,
		`SELECT id FROM users 
		 WHERE ($1 != '' AND github_id = $1) OR lower(username) = lower($2) 
		 ORDER BY CASE WHEN github_id = $1 THEN 0 ELSE 1 END, id ASC 
		 LIMIT 1`,
		[]any{githubID, username},
		func(row pgx.Row) error {
			return row.Scan(&existingID)
		},
	)

	if err == nil && existingID > 0 {
		// Existing account found: merge and link GitHub ID, update token, avatar, and username
		err = s.db.ExecReturning(ctx,
			`UPDATE users 
			 SET github_id           = COALESCE(NULLIF($1, ''), github_id),
			     username            = $2,
			     avatar_url          = COALESCE(NULLIF($3, ''), avatar_url),
			     github_access_token = COALESCE(NULLIF($4, ''), github_access_token)
			 WHERE id = $5
			 RETURNING id`,
			[]any{githubID, username, avatarURL, accessToken, existingID},
			func(row pgx.Row) error {
				return row.Scan(&userID)
			},
		)
		if err != nil {
			return 0, fmt.Errorf("auth: update linked user: %w", err)
		}
	} else {
		// New account: insert new user
		err = s.db.ExecReturning(ctx,
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
			return 0, fmt.Errorf("auth: insert user: %w", err)
		}
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

// UpsertUserWithAppToken is UpsertUser for the GitHub App login flow: it
// additionally persists the refresh token and expiry a user-to-server token
// carries, so GetAccessToken can refresh it before it expires.
func (s *Service) UpsertUserWithAppToken(ctx context.Context, githubID, username, avatarURL string, tok *githubapp.UserToken) (int, error) {
	userID, err := s.UpsertUser(ctx, githubID, username, avatarURL, tok.AccessToken)
	if err != nil {
		return 0, err
	}
	if err := s.storeUserToken(ctx, userID, tok); err != nil {
		return 0, fmt.Errorf("auth: store app token: %w", err)
	}
	return userID, nil
}
