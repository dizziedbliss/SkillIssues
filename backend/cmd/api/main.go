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
	"github.com/dizziedbliss/skillissues/internal/crypto"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/githubapp"
	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/profile"
	"github.com/dizziedbliss/skillissues/internal/settings"
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
	tokenCipher, err := crypto.New(cfg.TokenEncryptionKey)
	if err != nil {
		slog.Error("failed to initialize token encryption", "error", err)
		os.Exit(1)
	}

	authSvc := auth.New(pool)
	authSvc.SetTokenCipher(tokenCipher)
	ghOAuth := auth.NewGitHubOAuth(auth.GitHubConfig{
		ClientID:     cfg.GitHubClientID,
		ClientSecret: cfg.GitHubClientSecret,
		RedirectURI:  cfg.GitHubRedirectURI,
		GraphQLURL:   cfg.GitHubGraphQLURL,
	}, authSvc)

	// GitHub App (D5): optional until configured. Installation token drives
	// server-side crawling/on-demand reads; user-to-server token drives
	// fork/push/PR, replacing the OAuth App's blanket `repo` scope for
	// accounts that log in through it. Disabled (nil) leaves the OAuth
	// App/PAT login paths as the only ones, unchanged.
	var ghApp *githubapp.App
	var ghAppLogin *auth.GitHubAppLogin
	if cfg.GitHubAppID != "" {
		privKey, err := githubapp.LoadPrivateKeyFile(cfg.GitHubAppPrivateKeyPath)
		if err != nil {
			slog.Error("failed to load GitHub App private key", "error", err)
			os.Exit(1)
		}
		ghApp, err = githubapp.New(githubapp.Config{
			AppID:        cfg.GitHubAppID,
			ClientID:     cfg.GitHubAppClientID,
			ClientSecret: cfg.GitHubAppClientSecret,
			PrivateKey:   privKey,
			CallbackURL:  cfg.GitHubAppCallbackURL,
		})
		if err != nil {
			slog.Error("failed to initialize GitHub App", "error", err)
			os.Exit(1)
		}
		authSvc.SetGitHubApp(ghApp)
		ghAppLogin = auth.NewGitHubAppLogin(ghApp, authSvc, ghOAuth)
		slog.Info("GitHub App configured", "app_id", cfg.GitHubAppID)
	}
	// 5. Load live settings and start watching for changes. Operational values
	// are read from the snapshot per request, so they can change without a
	// restart; see internal/settings.
	live := settings.NewLive(ctx, pool.Raw(), cfg.DatabaseURL)
	live.Start(ctx)

	profileSvc := profile.New(pool)
	issueSvc := issue.New(pool, func() int {
		return live.Current().Int(settings.SearchPerCategoryCap)
	})
	ghClient := github.NewClient(cfg.GitHubGraphQLURL)
	contribSvc := contribution.New(pool, cacheClient, ghClient, func() int {
		return live.Current().Int(settings.ContributionVerifyPerHour)
	})
	contribSvc.SetTokenCipher(tokenCipher)
	authSvc.SetSessionTTL(func() time.Duration {
		return live.Current().Hours(settings.SessionTTLHours)
	})

	// 6. Build the HTTP router and register all routes
	mux := buildRouter(cfg, live, pool, cacheClient, authSvc, ghOAuth, ghAppLogin, profileSvc, issueSvc, contribSvc, ghClient)

	// 7. Start the HTTP server
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
	live *settings.Live,
	pool *db.Pool,
	cacheClient *cache.Client,
	authSvc *auth.Service,
	ghOAuth *auth.GitHubOAuth,
	ghAppLogin *auth.GitHubAppLogin,
	profileSvc *profile.Service,
	issueSvc *issue.Service,
	contribSvc *contribution.Service,
	ghClient *github.Client,
) http.Handler {
	mux := http.NewServeMux()

	// Handlers object bundles all dependencies for handler functions
	h := &handlers{
		cfg:        cfg,
		live:       live,
		db:         pool,
		cache:      cacheClient,
		authSvc:    authSvc,
		ghOAuth:    ghOAuth,
		ghAppLogin: ghAppLogin,
		profileSvc: profileSvc,
		issueSvc:   issueSvc,
		contribSvc: contribSvc,
		ghClient:   ghClient,
	}
	go h.sweepExpiredTickets(context.Background())

	// ----- Health & Metrics -----
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /health/detail", h.healthDetail)
	mux.Handle("GET /metrics", metrics.Default.Handler())

	// ----- Auth -----
	mux.HandleFunc("GET /auth/session", h.getSession)
	mux.HandleFunc("POST /auth/github/token", h.authGitHubToken)
	mux.HandleFunc("GET /auth/github", h.authGitHubBegin)
	mux.HandleFunc("GET /auth/github/callback", h.authGitHubCallback)
	mux.HandleFunc("GET /auth/github/poll", h.authGitHubPoll)
	mux.HandleFunc("GET /auth/github-app", h.authGitHubAppBegin)
	mux.HandleFunc("GET /auth/github-app/callback", h.authGitHubAppCallback)
	mux.HandleFunc("POST /auth/logout", h.authLogout)
	mux.HandleFunc("POST /auth/purge", h.authPurge)

	// ----- GitHub integration -----
	mux.Handle("POST /github/forks", requireAuth(authSvc, http.HandlerFunc(h.forkRepository)))
	mux.Handle("GET /github/git-token", requireAuth(authSvc, http.HandlerFunc(h.getGitToken)))

	// ----- Issues -----
	mux.Handle("GET /issues", optionalAuth(authSvc, http.HandlerFunc(h.listIssues)))
	mux.HandleFunc("GET /issues/{id}", h.getIssue) // no auth required

	// ----- Saved issues -----
	mux.Handle("GET /saved", requireAuth(authSvc, http.HandlerFunc(h.getSaved)))
	mux.Handle("POST /saved", requireAuth(authSvc, http.HandlerFunc(h.saveIssue)))
	mux.Handle("DELETE /saved/{id}", requireAuth(authSvc, http.HandlerFunc(h.unsaveIssue)))

	// ----- Recommendations -----
	mux.Handle("GET /recommendations", requireAuth(authSvc, http.HandlerFunc(h.getRecommendations)))

	// ----- User profile -----
	mux.Handle("GET /me", requireAuth(authSvc, http.HandlerFunc(h.getMe)))
	mux.Handle("PUT /profile", requireAuth(authSvc, http.HandlerFunc(h.updateProfile)))
	mux.HandleFunc("GET /preferences/options", h.getPreferenceOptions) // no auth required

	// ----- Contributions -----
	mux.Handle("POST /contributions", requireAuth(authSvc, http.HandlerFunc(h.startContribution)))
	mux.Handle("GET /contributions", requireAuth(authSvc, http.HandlerFunc(h.listContributions)))
	mux.Handle("GET /working", requireAuth(authSvc, http.HandlerFunc(h.getWorking)))
	mux.Handle("GET /contributions/{id}", requireAuth(authSvc, http.HandlerFunc(h.getContribution)))
	mux.Handle("PUT /contributions/{id}", requireAuth(authSvc, http.HandlerFunc(h.updateContribution)))
	mux.Handle("POST /contributions/{id}/submit", requireAuth(authSvc, http.HandlerFunc(h.submitContribution)))
	mux.Handle("POST /contributions/{id}/verify", requireAuth(authSvc, http.HandlerFunc(h.verifyContribution)))

	// ----- Webhooks (GitHub HMAC authenticated) -----
	mux.HandleFunc("POST /webhooks/github", h.webhookGitHub)

	// ----- Admin -----
	mux.Handle("POST /admin/sync/repository/{id}",
		requireAdmin(cfg.AdminSyncToken, http.HandlerFunc(h.adminSyncRepository)))
	mux.Handle("GET /admin/config",
		requireAdmin(cfg.AdminSyncToken, http.HandlerFunc(h.adminGetConfig)))
	mux.Handle("PUT /admin/config/{key}",
		requireAdmin(cfg.AdminSyncToken, http.HandlerFunc(h.adminPutConfig)))

	// Apply global middleware: RequestID → Logger → CORS → OptionalAuth → RateLimiter → mux
	var handler http.Handler = mux
	if cacheClient != nil {
		trustedProxies := middleware.ParseTrustedProxies(cfg.TrustedProxies)
		// Read limits per request so a settings change applies without a restart.
		handler = middleware.RateLimiter(cacheClient, func() middleware.RateLimits {
			s := live.Current()
			return middleware.RateLimits{
				Unauth: int64(s.Int(settings.RateLimitUnauthPerHour)),
				Auth:   int64(s.Int(settings.RateLimitAuthPerHour)),
				Window: s.Seconds(settings.RateLimitWindowSeconds),
			}
		}, "global", trustedProxies)(handler)
	}
	handler = middleware.OptionalAuth(authSvc)(handler)
	handler = middleware.CORS(cfg.FrontendURL)(handler)
	handler = middleware.Logger(handler)
	handler = middleware.RequestID(handler)

	return handler
}

// ---------------------------------------------------------------------------
// Middleware helper functions (thin wrappers around middleware package)
// ---------------------------------------------------------------------------

func requireAuth(authSvc *auth.Service, next http.Handler) http.Handler {
	return middleware.RequireAuth(authSvc)(next)
}

func optionalAuth(authSvc *auth.Service, next http.Handler) http.Handler {
	return middleware.OptionalAuth(authSvc)(next)
}

func requireAdmin(token string, next http.Handler) http.Handler {
	return middleware.RequireAdmin(token)(next)
}
