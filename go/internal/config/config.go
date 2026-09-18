// Package config provides typed configuration loaded from environment variables.
// All service configuration is centralised here; no other package reads os.Getenv directly.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration for the SkillIssues backend.
// Values are read from environment variables at startup; missing required values cause a fatal error.
type Config struct {
	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// HTTP server
	Port int

	// CORS / Frontend
	FrontendURL string

	// Demo mode: disables webhook HMAC checks and enables /auth/demo endpoint
	DemoMode bool

	// GitHub integration
	GitHubToken         string // Server-side PAT for catalog ingestion and fallback operations
	GitHubWebhookSecret string // HMAC secret for verifying GitHub webhook payloads
	GitHubClientID      string // OAuth App client ID
	GitHubClientSecret  string // OAuth App client secret
	GitHubRedirectURI   string // OAuth callback URL (must match GitHub App settings)
	GitHubGraphQLURL    string // GitHub GraphQL endpoint (default: https://api.github.com/graphql)

	// Admin
	AdminSyncToken string // Bearer token for /admin/* endpoints

	// Sync coordinator
	WorkerCount         int // Routine sync goroutines (default: 4)
	PriorityWorkerCount int // Priority sync goroutines (default: 2)
	OnDemandWorkerCount int // On-demand goroutines (default: 8)
	SyncIntervalSecs    int // Seconds between routine sync ticks (default: 900)
	SyncdURL            string // URL to Go syncd service (default: http://syncd:8002)

	// Compatibility: external service URLs retained during Python→Go migration
	// These will be removed once all services are ported to Go.
	RecommendationURL string
	ContributionURL   string

	// Kaggle ETL (used by Python ingestion pipeline only)
	KaggleUsername     string
	KaggleKey          string
	MaxRepositories    int
	MinStars           int
}

// Load reads and validates all configuration from environment variables.
// It returns an error if any required variable is missing or malformed.
func Load() (*Config, error) {
	cfg := &Config{
		Port:                intEnv("PORT", 8000),
		DemoMode:            boolEnv("DEMO_MODE", false),
		GitHubGraphQLURL:    strEnv("GITHUB_GRAPHQL_URL", "https://api.github.com/graphql"),
		FrontendURL:         strEnv("FRONTEND_URL", "http://localhost:5173"),
		GitHubRedirectURI:   strEnv("GITHUB_REDIRECT_URI", "http://localhost:8000/auth/github/callback"),
		WorkerCount:         intEnv("WORKER_COUNT", 4),
		PriorityWorkerCount: intEnv("PRIORITY_WORKER_COUNT", 2),
		OnDemandWorkerCount: intEnv("ON_DEMAND_WORKER_COUNT", 8),
		SyncIntervalSecs:    intEnv("SYNC_INTERVAL_SECONDS", 900),
		SyncdURL:            strEnv("SYNCD_URL", "http://syncd:8002"),
		MaxRepositories:     intEnv("MAX_REPOSITORIES", 500),
		MinStars:            intEnv("MIN_STARS", 50),
		RecommendationURL:   strEnv("RECOMMENDATION_URL", "http://recommendation:8000"),
		ContributionURL:     strEnv("CONTRIBUTION_URL", "http://contribution:8000"),
		KaggleUsername:      strEnv("KAGGLE_USERNAME", ""),
		KaggleKey:           strEnv("KAGGLE_KEY", ""),
	}

	// Required variables
	var missing []string

	cfg.DatabaseURL = cleanToken(os.Getenv("DATABASE_URL"))
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}

	cfg.RedisURL = strEnv("REDIS_URL", "redis://localhost:6379/0")

	cfg.GitHubToken = cleanToken(os.Getenv("GITHUB_TOKEN"))
	cfg.GitHubWebhookSecret = strEnv("GITHUB_WEBHOOK_SECRET", "dev_local_webhook_secret")
	cfg.GitHubClientID = cleanToken(os.Getenv("GITHUB_CLIENT_ID"))
	cfg.GitHubClientSecret = cleanToken(os.Getenv("GITHUB_CLIENT_SECRET"))
	cfg.AdminSyncToken = strEnv("ADMIN_SYNC_TOKEN", "dev_local_admin_sync_token")

	// In strict production mode, verify they are explicitly configured
	if !cfg.DemoMode && os.Getenv("ENV") == "production" {
		if cfg.GitHubWebhookSecret == "dev_local_webhook_secret" {
			missing = append(missing, "GITHUB_WEBHOOK_SECRET")
		}
		if cfg.AdminSyncToken == "dev_local_admin_sync_token" {
			missing = append(missing, "ADMIN_SYNC_TOKEN")
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

func intEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func boolEnv(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
