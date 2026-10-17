// SkillIssues API Server.
//
// This is the main entry point for the Go backend. It:
//  1. Loads configuration from environment variables
//  2. Connects to PostgreSQL and Redis
//  3. Wires up all service dependencies
//  4. Registers the HTTP routes
//  5. Starts the HTTP server with graceful shutdown
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
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/flow"
	"github.com/dizziedbliss/skillissues/internal/httpx"
	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/notification"
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
	cfg, err := config.Load(config.SecretWebhook, config.SecretAdminToken)
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("SkillIssues API starting",
		"port", cfg.Port,
		"frontend_url", cfg.FrontendURL,
	)

	// 2. Connect to PostgreSQL
	dbCfg := db.DefaultConfig(cfg.DatabaseURL)
	if cfg.DBMaxConns > 0 {
		dbCfg.MaxConns = int32(cfg.DBMaxConns)
	} else {
		// Default sized for the 4GB/2vCPU prod target: api + ingestd + the
		// LISTEN connection each service opens must stay under Postgres's
		// max_connections (60 in prod) with room for migrate and superuser.
		dbCfg.MaxConns = 15
	}
	pool, err := db.New(ctx, dbCfg)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("database connected")

	metrics.RegisterAPI()
	metrics.RegisterGitHub()
	metrics.RegisterDBPool(pool.Raw())

	// 3. Instantiate service layer
	authSvc := auth.New(pool)
	// 4. Load live settings and start watching for changes. Operational values
	// are read from the snapshot per request, so they can change without a
	// restart; see internal/settings.
	live := settings.NewLive(ctx, pool.Raw(), cfg.DatabaseURL)
	live.Start(ctx)

	// The live visualizer's tap. Fire-and-forget: without Redis the API runs
	// the same, and every dropped event is counted.
	metrics.RegisterFlowPublisher()
	if pub, err := flow.NewPublisher(flow.API, cfg.RedisURL, flow.SettingsLimits(live.Current)); err != nil {
		slog.Warn("invalid REDIS_URL — live flow events are not published", "error", err)
	} else {
		flow.Install(pub)
		go pub.Run(ctx)
		defer pub.Close()
	}

	outbound := settings.OutboundClient(live, flow.Transport{})
	ghOAuth := auth.NewGitHubOAuth(auth.GitHubConfig{
		ClientID:     cfg.GitHubLoginClientID,
		ClientSecret: cfg.GitHubLoginClientSecret,
		RedirectURI:  cfg.GitHubRedirectURI,
		HTTP:         outbound,
	}, authSvc)

	// 5. Connect to Redis (non-fatal: cache operates in degraded mode if
	// unavailable). After settings, because its call deadline and retry
	// interval are live settings.
	cacheClient, err := cache.New(cfg.RedisURL, func() cache.Limits {
		s := live.Current()
		return cache.Limits{
			CallTimeout: s.Millis(settings.CacheRedisTimeoutMs),
			RetryAfter:  s.Seconds(settings.CacheRedisRetrySeconds),
		}
	})
	if err != nil {
		slog.Warn("invalid REDIS_URL — operating without cache or rate limiting", "error", err)
		cacheClient = nil
	} else if err := cacheClient.Ping(ctx); err != nil {
		// Keep the client: it reconnects on its own once Redis is up, and
		// until then calls fail open without waiting on it.
		slog.Warn("redis unreachable at startup — cache and rate limiting fail open until it is", "error", err)
	} else {
		slog.Info("redis connected")
	}
	if cacheClient != nil {
		defer cacheClient.Close()
	}

	profileSvc := profile.New(pool)
	issueSvc := issue.New(pool, func() int {
		return live.Current().Int(settings.SearchPerCategoryCap)
	})
	authSvc.SetSessionTTL(func() time.Duration {
		return live.Current().Hours(settings.SessionTTLHours)
	})
	go pruneSessionsLoop(ctx, authSvc, live.Current)

	// 6. Build the HTTP router and register all routes
	mux, err := buildRouter(cfg, live, pool, cacheClient, authSvc, ghOAuth, outbound, profileSvc, issueSvc)
	if err != nil {
		slog.Error("failed to build router", "error", err)
		os.Exit(1)
	}

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
	outbound httpx.Doer,
	profileSvc *profile.Service,
	issueSvc *issue.Service,
) (http.Handler, error) {
	mux := http.NewServeMux()        // public routes — full middleware chain
	adminMux := http.NewServeMux()   // /admin — own auth, no CORS/rate-limit
	webhookMux := http.NewServeMux() // /webhooks — HMAC verified, no CORS/rate-limit

	// Handlers object bundles all dependencies for handler functions
	h := &handlers{
		cfg:             cfg,
		live:            live,
		db:              pool,
		cache:           cacheClient,
		authSvc:         authSvc,
		ghOAuth:         ghOAuth,
		outbound:        outbound,
		profileSvc:      profileSvc,
		issueSvc:        issueSvc,
		notificationSvc: notification.NewService(pool),
	}

	// ----- Health & Metrics -----
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /health/detail", h.healthDetail)
	mux.Handle("GET /metrics", metrics.Handler())

	// ----- Auth -----
	mux.HandleFunc("GET /auth/session", h.getSession)
	mux.HandleFunc("GET /auth/github", h.authGitHubBegin)
	mux.HandleFunc("GET /auth/github/callback", h.authGitHubCallback)
	mux.HandleFunc("POST /auth/github/device", h.authGitHubDevice)
	mux.HandleFunc("POST /auth/logout", h.authLogout)
	mux.HandleFunc("POST /auth/logout-all", h.authLogoutAll)
	mux.HandleFunc("POST /auth/purge", h.authPurge)

	// ----- Issues -----
	mux.Handle("GET /issues", optionalAuth(authSvc, http.HandlerFunc(h.listIssues)))
	mux.HandleFunc("GET /issues/{id}", h.getIssue) // no auth required

	// ----- Saved issues -----
	mux.Handle("GET /saved", requireAuth(authSvc, http.HandlerFunc(h.getSaved)))
	mux.Handle("POST /saved", requireAuth(authSvc, http.HandlerFunc(h.saveIssue)))
	mux.Handle("DELETE /saved/{id}", requireAuth(authSvc, http.HandlerFunc(h.unsaveIssue)))

	// ----- Notifications -----
	mux.Handle("GET /notifications", requireAuth(authSvc, http.HandlerFunc(h.getNotifications)))
	mux.Handle("POST /notifications/read-all", requireAuth(authSvc, http.HandlerFunc(h.markAllNotificationsRead)))
	mux.Handle("POST /notifications/{id}/read", requireAuth(authSvc, http.HandlerFunc(h.markNotificationRead)))

	// ----- Recommendations -----
	mux.Handle("GET /recommendations", requireAuth(authSvc, http.HandlerFunc(h.getRecommendations)))

	// ----- User profile -----
	mux.Handle("GET /me", requireAuth(authSvc, http.HandlerFunc(h.getMe)))
	mux.Handle("PUT /profile", requireAuth(authSvc, http.HandlerFunc(h.updateProfile)))
	mux.HandleFunc("GET /preferences/options", h.getPreferenceOptions) // no auth required

	// ----- Webhooks (GitHub HMAC authenticated) -----
	webhookMux.HandleFunc("POST /webhooks/github", h.webhookGitHub)

	// ----- Admin -----
	adminMux.Handle("POST /admin/sync/repository/{id}",
		requireAdmin(cfg.AdminSyncToken, http.HandlerFunc(h.adminSyncRepository)))
	adminMux.Handle("GET /admin/config",
		requireAdmin(cfg.AdminSyncToken, http.HandlerFunc(h.adminGetConfig)))
	adminMux.Handle("PUT /admin/config/{key}",
		requireAdmin(cfg.AdminSyncToken, http.HandlerFunc(h.adminPutConfig)))

	httpBodyLimit := middleware.MaxBody(func() int64 { return int64(live.Current().Int(settings.HTTPMaxBodyBytes)) })
	webhookBodyLimit := middleware.MaxBody(func() int64 { return int64(live.Current().Int(settings.WebhookMaxBodyBytes)) })

	// Public chain: RequestID → Logger → FlowTap → MaxBody → CORS → CSRF → OptionalAuth → RateLimiter → mux
	var publicHandler http.Handler = middleware.RecordRoute(mux)
	if cacheClient != nil {
		trustedProxies, err := middleware.ParseTrustedProxies(cfg.TrustedProxies)
		if err != nil {
			return nil, err
		}
		// Read limits per request so a settings change applies without a restart.
		publicHandler = middleware.RateLimiter(cacheClient, func() middleware.RateLimits {
			s := live.Current()
			return middleware.RateLimits{
				Unauth: int64(s.Int(settings.RateLimitUnauthPerHour)),
				Auth:   int64(s.Int(settings.RateLimitAuthPerHour)),
				Window: s.Seconds(settings.RateLimitWindowSeconds),
			}
		}, "global", trustedProxies)(publicHandler)
	}
	publicHandler = middleware.OptionalAuth(authSvc)(publicHandler)
	csrf, err := middleware.CSRF(cfg.FrontendURL, sessionCookieName)
	if err != nil {
		return nil, err
	}
	publicHandler = csrf(publicHandler)
	publicHandler = middleware.CORS(cfg.FrontendURL, cfg.Production)(publicHandler)
	publicHandler = httpBodyLimit(publicHandler)
	publicHandler = middleware.FlowTap(flow.Users)(publicHandler)
	publicHandler = middleware.Logger(publicHandler)
	publicHandler = middleware.RequestID(publicHandler)

	// /admin and /webhooks authenticate themselves (admin token / HMAC signature) and
	// have no browser callers, so they skip CORS, OptionalAuth and the browsing rate
	// limiter — only RequestID/Logger apply.
	top := http.NewServeMux()
	top.Handle("/admin/", group(middleware.RecordRoute(adminMux), middleware.Logger, middleware.RequestID, httpBodyLimit))
	top.Handle("/webhooks/", group(middleware.RecordRoute(webhookMux), middleware.Logger, middleware.FlowTap(flow.GitHub), middleware.RequestID, webhookBodyLimit))
	top.Handle("/", publicHandler)

	return top, nil
}

// group applies middleware (outermost first) to a handler mounted at a path prefix,
// letting it bypass the global chain applied to the rest of the router.
func group(handler http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		handler = mw[i](handler)
	}
	return handler
}

// ---------------------------------------------------------------------------
// Middleware helper functions (thin wrappers around middleware package)
// ---------------------------------------------------------------------------

// pruneSessionsLoop deletes expired sessions and abandoned OAuth handshakes
// every api.session.prune_interval_seconds. A failure is logged and counted
// on the next pass; the tables only grow while it keeps failing.
func pruneSessionsLoop(ctx context.Context, authSvc *auth.Service, live func() *settings.Snapshot) {
	interval := live().Seconds(settings.SessionPruneIntervalSeconds)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sessions, states, err := authSvc.PruneExpired(ctx)
			if err != nil {
				slog.Error("failed to prune expired sessions", "error", err)
			} else if sessions > 0 || states > 0 {
				slog.Info("pruned expired auth rows", "sessions", sessions, "oauth_states", states)
			}
			if next := live().Seconds(settings.SessionPruneIntervalSeconds); next != interval {
				interval = next
				ticker.Reset(interval)
			}
		}
	}
}

func requireAuth(authSvc *auth.Service, next http.Handler) http.Handler {
	return middleware.RequireAuth(authSvc)(next)
}

func optionalAuth(authSvc *auth.Service, next http.Handler) http.Handler {
	return middleware.OptionalAuth(authSvc)(next)
}

func requireAdmin(token string, next http.Handler) http.Handler {
	return middleware.RequireAdmin(token)(next)
}
