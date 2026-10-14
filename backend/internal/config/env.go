// Package config provides the startup configuration read from the environment.
//
// This is layers 1 and 2 only: bootstrap values needed before the database
// exists, and secrets. Operational tunables — limits, page sizes, intervals,
// worker counts, cache TTLs — live in the `settings` table and are served by
// internal/settings, which reloads them without a restart.
//
// Adding an operational knob here is almost always the wrong move; add a key to
// the settings registry instead. Secrets, conversely, must never go in the
// settings table: it is readable through an admin endpoint and reproduced in
// audit rows and database backups.
package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds startup configuration for the SkillIssues backend.
type Config struct {
	// --- Layer 1: bootstrap, needed before the database is reachable ---

	DatabaseURL string
	RedisURL    string
	Port        int

	// DBMaxConns overrides the connection pool's max size (db.DefaultConfig
	// otherwise defaults to 20). Pool size is fixed at creation, so changing
	// it needs a restart regardless — it stays a bootstrap env var rather
	// than a live `settings` row. 0 means "unset"; callers apply their own
	// per-binary default (api and ingestd size differently against the same
	// Postgres max_connections ceiling).
	DBMaxConns int

	// FrontendURL drives CORS — it may be a comma-separated list of allowed
	// origins — and is fixed at startup rather than reloadable.
	FrontendURL string

	// FrontendURLPrimary is FrontendURL's first entry, trimmed. Redirects
	// (OAuth callbacks) and the cookie Secure/SameSite decision must pick
	// exactly one origin, never the raw comma-joined value — a browser
	// rejects a multi-value Location or Access-Control-Allow-Origin.
	FrontendURLPrimary string

	// SessionCookieDomain optionally sets the session cookie's Domain
	// attribute, so it is shared across a parent domain (e.g. a web frontend
	// on one subdomain and the API on another). Empty means "unset" — the
	// browser defaults to the exact host that set the cookie.
	SessionCookieDomain string

	// SyncdURL is the ingestion service address used to forward sync requests.
	SyncdURL string

	// TrustedProxies lists the CIDRs allowed to set X-Forwarded-For/X-Real-IP
	// for rate-limit identification. A request from any other peer has those
	// headers ignored, since otherwise any client could spoof its rate-limit
	// identity by setting them itself. Defaults to loopback only (the
	// Docker Compose topology puts no proxy in front of the API).
	TrustedProxies []string

	// GitHubGraphQLURL is overridden only by tests and GitHub Enterprise.
	GitHubGraphQLURL string

	// --- Layer 2: secrets ---

	GitHubToken         string // Server-side token for ingestion
	GitHubWebhookSecret string // HMAC secret for verifying webhook payloads
	GitHubClientID      string // OAuth App client ID
	GitHubClientSecret  string // OAuth App client secret
	GitHubRedirectURI   string // OAuth callback URL; must match the GitHub App
	AdminSyncToken      string // Bearer token for /admin/* endpoints

	// GitHub App (plan D5) — separate from the OAuth App above. Installation
	// token drives server-side crawling/reads; the user-to-server token
	// drives fork/push/PR. All empty means the App isn't configured yet and
	// those code paths stay disabled, falling back to the OAuth App/PAT.
	GitHubAppID             string
	GitHubAppClientID       string
	GitHubAppClientSecret   string
	GitHubAppPrivateKeyPath string
	GitHubAppCallbackURL    string

	// TokenEncryptionKey is the AES-256 key (derived via SHA-256 from
	// TOKEN_ENCRYPTION_KEY, so operators can set any length secret) used to
	// encrypt users.github_access_token at rest. Losing or rotating it
	// without migrating existing rows makes stored tokens undecryptable —
	// users are then asked to reconnect GitHub (auth.ErrGitHubReauthRequired).
	TokenEncryptionKey []byte
}

// Secret names a secret a binary needs. With ENV=production, Load refuses to
// start while any secret the caller requires still has its development
// placeholder value. Each binary requires only the secrets it uses, so a
// deployment never has to hand a service a credential it has no use for.
type Secret string

const (
	SecretWebhook         Secret = "GITHUB_WEBHOOK_SECRET"
	SecretAdminToken      Secret = "ADMIN_SYNC_TOKEN"
	SecretTokenEncryption Secret = "TOKEN_ENCRYPTION_KEY"
)

// placeholders are the development defaults for each Secret.
var placeholders = map[Secret]string{
	SecretWebhook:         "dev_local_webhook_secret",
	SecretAdminToken:      "dev_local_admin_sync_token",
	SecretTokenEncryption: "dev_local_token_encryption_key",
}

// Load reads and validates configuration from the environment. required
// lists the secrets this binary uses (see Secret).
func Load(required ...Secret) (*Config, error) {
	cfg := &Config{
		Port:                intEnv("PORT", 8000),
		GitHubGraphQLURL:    strEnv("GITHUB_GRAPHQL_URL", "https://api.github.com/graphql"),
		FrontendURL:         strEnv("FRONTEND_URL", "http://localhost:5173"),
		GitHubRedirectURI:   strEnv("GITHUB_REDIRECT_URI", "http://localhost:8000/auth/github/callback"),
		SyncdURL:            strEnv("SYNCD_URL", "http://ingestd:8002"),
		RedisURL:            strEnv("REDIS_URL", "redis://localhost:6379/0"),
		TrustedProxies:      strListEnv("TRUSTED_PROXIES", []string{"127.0.0.1/32", "::1/128"}),
		DBMaxConns:          intEnv("DB_MAX_CONNS", 0),
		SessionCookieDomain: strEnv("SESSION_COOKIE_DOMAIN", ""),
	}
	cfg.FrontendURLPrimary = strings.TrimSpace(strings.SplitN(cfg.FrontendURL, ",", 2)[0])
	if cfg.FrontendURLPrimary == "" {
		cfg.FrontendURLPrimary = "http://localhost:5173"
	}

	var missing []string

	cfg.DatabaseURL = cleanToken(os.Getenv("DATABASE_URL"))
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}

	cfg.GitHubToken = cleanToken(os.Getenv("GITHUB_TOKEN"))
	cfg.GitHubWebhookSecret = strEnv(string(SecretWebhook), placeholders[SecretWebhook])
	cfg.GitHubClientID = cleanToken(os.Getenv("GITHUB_CLIENT_ID"))
	cfg.GitHubClientSecret = cleanToken(os.Getenv("GITHUB_CLIENT_SECRET"))
	cfg.AdminSyncToken = strEnv(string(SecretAdminToken), placeholders[SecretAdminToken])

	cfg.GitHubAppID = cleanToken(os.Getenv("GITHUB_APP_ID"))
	cfg.GitHubAppClientID = cleanToken(os.Getenv("GITHUB_APP_CLIENT_ID"))
	cfg.GitHubAppClientSecret = cleanToken(os.Getenv("GITHUB_APP_CLIENT_SECRET"))
	cfg.GitHubAppPrivateKeyPath = cleanToken(os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH"))
	cfg.GitHubAppCallbackURL = strEnv("GITHUB_APP_CALLBACK_URL", "http://localhost:8000/auth/github-app/callback")

	tokenEncryptionSecret := strEnv(string(SecretTokenEncryption), placeholders[SecretTokenEncryption])
	keyHash := sha256.Sum256([]byte(tokenEncryptionSecret))
	cfg.TokenEncryptionKey = keyHash[:]

	// Refuse to start in production on the development placeholder of any
	// secret this binary uses.
	if os.Getenv("ENV") == "production" {
		values := map[Secret]string{
			SecretWebhook:         cfg.GitHubWebhookSecret,
			SecretAdminToken:      cfg.AdminSyncToken,
			SecretTokenEncryption: tokenEncryptionSecret,
		}
		for _, sec := range required {
			if values[sec] == placeholders[sec] {
				missing = append(missing, string(sec))
			}
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return cfg, nil
}

func cleanToken(s string) string {
	return strings.Trim(strings.TrimSpace(s), "\"' \t\r\n")
}

func strEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return cleanToken(v)
	}
	return fallback
}

func strListEnv(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var out []string
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func intEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
