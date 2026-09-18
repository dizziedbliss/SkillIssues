// SkillIssues API Server — Go implementation
//
// This is the main entry point for the Go backend. It:
//   1. Loads configuration from environment variables
//   2. Connects to PostgreSQL and Redis
//   3. Wires up all service dependencies
//   4. Registers all 28 HTTP routes (matching the Python FastAPI spec exactly)
//   5. Starts the HTTP server with graceful shutdown
//
// During the migration phase, this Go API runs alongside the Python API.
// Once all routes are verified, the Python api service is decommissioned.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dizziedbliss/skillissues/internal/auth"
	"github.com/dizziedbliss/skillissues/internal/cache"
	"github.com/dizziedbliss/skillissues/internal/config"
	"github.com/dizziedbliss/skillissues/internal/contribution"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/profile"
)

func main() {
	// Configure structured JSON logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	ctx := context.Background()

	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("SkillIssues API starting",
		"port", cfg.Port,
		"demo_mode", cfg.DemoMode,
		"frontend_url", cfg.FrontendURL,
	)

	// 2. Connect to PostgreSQL
	pool, err := db.New(ctx, db.DefaultConfig(cfg.DatabaseURL))
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("database connected")

	// 3. Connect to Redis (non-fatal: cache operates in degraded mode if unavailable)
	cacheClient, err := cache.New(ctx, cfg.RedisURL)
	if err != nil {
		slog.Warn("redis connection failed — operating without cache", "error", err)
		cacheClient = nil
	} else {
		slog.Info("redis connected")
	}
	if cacheClient != nil {
		defer cacheClient.Close()
	}

	// 4. Instantiate service layer
	authSvc := auth.New(pool, cacheClient)
	ghOAuth := auth.NewGitHubOAuth(auth.GitHubConfig{
		ClientID:     cfg.GitHubClientID,
		ClientSecret: cfg.GitHubClientSecret,
		RedirectURI:  cfg.GitHubRedirectURI,
		GraphQLURL:   cfg.GitHubGraphQLURL,
	}, authSvc)
	profileSvc := profile.New(pool)
	issueSvc := issue.New(pool)
	ghClient := github.NewClient(cfg.GitHubGraphQLURL)
	contribSvc := contribution.New(pool, cacheClient, ghClient)

	// 5. Build the HTTP router and register all routes
	mux := buildRouter(cfg, pool, cacheClient, authSvc, ghOAuth, profileSvc, issueSvc, contribSvc, ghClient)

	// 6. Start the HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown: wait for SIGINT or SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
	slog.Info("server stopped")
}

// buildRouter constructs the ServeMux with all routes and middleware chains.
// Route registration mirrors the Python FastAPI spec exactly.
func buildRouter(
	cfg *config.Config,
	pool *db.Pool,
	cacheClient *cache.Client,
	authSvc *auth.Service,
	ghOAuth *auth.GitHubOAuth,
	profileSvc *profile.Service,
	issueSvc *issue.Service,
	contribSvc *contribution.Service,
	ghClient *github.Client,
) http.Handler {
	mux := http.NewServeMux()

	// Handlers object bundles all dependencies for handler functions
	h := &handlers{
		cfg:        cfg,
		db:         pool,
		cache:      cacheClient,
		authSvc:    authSvc,
		ghOAuth:    ghOAuth,
		profileSvc: profileSvc,
		issueSvc:   issueSvc,
		contribSvc: contribSvc,
		ghClient:   ghClient,
	}

	// ----- Health & Metrics -----
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /health/detail", h.healthDetail)
	mux.Handle("GET /metrics", metrics.Default.Handler())

	// ----- Auth -----
	mux.HandleFunc("GET /auth/session", h.getSession)
	mux.HandleFunc("POST /auth/github/token", h.authGitHubToken)
	mux.HandleFunc("GET /auth/github", h.authGitHubBegin)
	mux.HandleFunc("POST /auth/github", h.authGitHubBegin) // both GET and POST per spec
	mux.HandleFunc("GET /auth/github/callback", h.authGitHubCallback)
	mux.HandleFunc("GET /auth/github/poll", h.authGitHubPoll)
	mux.HandleFunc("POST /auth/logout", h.authLogout)
	mux.HandleFunc("POST /auth/purge", h.authPurge)
	if cfg.DemoMode {
		mux.HandleFunc("POST /auth/demo", h.authDemo)
	}

	// ----- GitHub integration -----
	mux.Handle("POST /github/forks", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.forkRepository)))
	mux.Handle("GET /github/git-token", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.getGitToken)))

	// ----- Issues -----
	mux.Handle("POST /issues/sync", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.syncIssues)))
	mux.Handle("GET /issues", optionalAuth(authSvc, cacheClient, http.HandlerFunc(h.listIssues)))
	mux.HandleFunc("GET /issues/{id}", h.getIssue) // no auth required

	// ----- Saved issues -----
	mux.Handle("GET /saved", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.getSaved)))
	mux.Handle("POST /saved", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.saveIssue)))
	mux.Handle("DELETE /saved/{id}", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.unsaveIssue)))

	// ----- Recommendations -----
	mux.Handle("GET /recommendations", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.getRecommendations)))

	// ----- User profile -----
	mux.Handle("GET /me", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.getMe)))
	mux.Handle("GET /profile", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.getMe))) // alias
	mux.Handle("PUT /profile", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.updateProfile)))
	mux.Handle("GET /preferences", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.getPreferences)))
	mux.Handle("PUT /preferences", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.updatePreferences)))
	mux.HandleFunc("GET /preferences/options", h.getPreferenceOptions) // no auth required
	mux.Handle("GET /progress", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.getProgress)))

	// ----- Contributions -----
	mux.Handle("POST /contributions", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.startContribution)))
	mux.Handle("GET /contributions", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.listContributions)))
	mux.Handle("GET /working", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.getWorking)))
	mux.Handle("GET /contributions/{id}", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.getContribution)))
	mux.Handle("PUT /contributions/{id}", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.updateContribution)))
	mux.Handle("POST /contributions/{id}/submit", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.submitContribution)))
	mux.Handle("POST /contributions/{id}/verify", requireAuth(authSvc, cacheClient, http.HandlerFunc(h.verifyContribution)))

	// ----- Webhooks (GitHub HMAC authenticated) -----
	mux.HandleFunc("POST /webhooks/github", h.webhookGitHub)

	// ----- Admin -----
	mux.Handle("POST /admin/sync/repository/{id}",
		requireAdmin(cfg.AdminSyncToken, http.HandlerFunc(h.adminSyncRepository)))

	// Apply global middleware: RequestID → Logger → CORS → RateLimiter → mux
	var handler http.Handler = mux
	if cacheClient != nil {
		handler = middleware.RateLimiter(cacheClient, 120, time.Minute, "global")(handler)
	}
	handler = middleware.CORS(cfg.FrontendURL)(handler)
	handler = middleware.Logger(handler)
	handler = middleware.RequestID(handler)

	return handler
}

// ---------------------------------------------------------------------------
// Middleware helper functions (thin wrappers around middleware package)
// ---------------------------------------------------------------------------

func requireAuth(authSvc *auth.Service, c *cache.Client, next http.Handler) http.Handler {
	return middleware.RequireAuth(authSvc, c)(next)
}

func optionalAuth(authSvc *auth.Service, c *cache.Client, next http.Handler) http.Handler {
	return middleware.OptionalAuth(authSvc, c)(next)
}

func requireAdmin(token string, next http.Handler) http.Handler {
	return middleware.RequireAdmin(token)(next)
}
