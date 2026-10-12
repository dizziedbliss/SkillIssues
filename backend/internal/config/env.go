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

	// FrontendURL drives CORS and decides whether session cookies are marked
	// Secure, so it is fixed at startup rather than reloadable.
	FrontendURL string

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

	// TokenEncryptionKey is the AES-256 key (derived via SHA-256 from
	// TOKEN_ENCRYPTION_KEY, so operators can set any length secret) used to
	// encrypt users.github_access_token at rest. Losing or rotating it
	// without migrating existing rows makes stored tokens undecryptable —
	// callers treat that as "no token", not a crash.
	TokenEncryptionKey []byte
}

// Load reads and validates configuration from the environment.
func Load() (*Config, error) {
	cfg := &Config{
		Port:              intEnv("PORT", 8000),
		GitHubGraphQLURL:  strEnv("GITHUB_GRAPHQL_URL", "https://api.github.com/graphql"),
		FrontendURL:       strEnv("FRONTEND_URL", "http://localhost:5173"),
		GitHubRedirectURI: strEnv("GITHUB_REDIRECT_URI", "http://localhost:8000/auth/github/callback"),
		SyncdURL:          strEnv("SYNCD_URL", "http://ingestd:8002"),
		RedisURL:          strEnv("REDIS_URL", "redis://localhost:6379/0"),
		TrustedProxies:    strListEnv("TRUSTED_PROXIES", []string{"127.0.0.1/32", "::1/128"}),
	}

	var missing []string

	cfg.DatabaseURL = cleanToken(os.Getenv("DATABASE_URL"))
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}

	cfg.GitHubToken = cleanToken(os.Getenv("GITHUB_TOKEN"))
	cfg.GitHubWebhookSecret = strEnv("GITHUB_WEBHOOK_SECRET", "dev_local_webhook_secret")
	cfg.GitHubClientID = cleanToken(os.Getenv("GITHUB_CLIENT_ID"))
	cfg.GitHubClientSecret = cleanToken(os.Getenv("GITHUB_CLIENT_SECRET"))
	cfg.AdminSyncToken = strEnv("ADMIN_SYNC_TOKEN", "dev_local_admin_sync_token")

	tokenEncryptionSecret := strEnv("TOKEN_ENCRYPTION_KEY", "dev_local_token_encryption_key")
	keyHash := sha256.Sum256([]byte(tokenEncryptionSecret))
	cfg.TokenEncryptionKey = keyHash[:]

	// Refuse to start in production on the development placeholder secrets.
	if os.Getenv("ENV") == "production" {
		if cfg.GitHubWebhookSecret == "dev_local_webhook_secret" {
			missing = append(missing, "GITHUB_WEBHOOK_SECRET")
		}
		if cfg.AdminSyncToken == "dev_local_admin_sync_token" {
			missing = append(missing, "ADMIN_SYNC_TOKEN")
		}
		if tokenEncryptionSecret == "dev_local_token_encryption_key" {
			missing = append(missing, "TOKEN_ENCRYPTION_KEY")
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
