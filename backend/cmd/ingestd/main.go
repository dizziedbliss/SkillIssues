package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/dizziedbliss/skillissues/internal/config"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/githubapp"
	"github.com/dizziedbliss/skillissues/internal/ingest"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("starting SkillIssues ingestion daemon (ingestd)")

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbCfg := db.DefaultConfig(cfg.DatabaseURL)
	if cfg.DBMaxConns > 0 {
		dbCfg.MaxConns = int32(cfg.DBMaxConns)
	} else {
		// Default sized for the 4GB/2vCPU prod target, same rationale as
		// cmd/api — kept smaller here since ingestd's crawl workers are
		// bound separately by ingest.routine_workers/ingest.priority_workers.
		dbCfg.MaxConns = 10
	}
	var dbPool *db.Pool
	for attempt := 1; attempt <= 15; attempt++ {
		dbPool, err = db.New(ctx, dbCfg)
		if err == nil {
			break
		}
		slog.Info("waiting for PostgreSQL database...", "attempt", attempt, "error", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		slog.Error("database connection failed permanently", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	ghClient := github.NewClient(cfg.GitHubGraphQLURL)

	// Token: prefer the GitHub App's installation token (D5, D2 — the crawl
	// credential strictly separate from the contribution path), falling
	// back to the plain PAT while the App isn't configured. Read fresh per
	// call so a renewed/rotated token takes effect without a restart.
	tokenFn := func() string { return cfg.GitHubToken }
	if cfg.GitHubAppID != "" {
		privKey, err := githubapp.LoadPrivateKeyFile(cfg.GitHubAppPrivateKeyPath)
		if err != nil {
			slog.Error("failed to load GitHub App private key", "error", err)
			os.Exit(1)
		}
		ghApp, err := githubapp.New(githubapp.Config{
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
		tokenFn = func() string {
			tok, err := ghApp.InstallationToken(ctx)
			if err != nil {
				slog.Warn("failed to mint installation token, falling back to GITHUB_TOKEN", "error", err)
				return cfg.GitHubToken
			}
			return tok
		}
		slog.Info("ingestd crawling via GitHub App installation token", "app_id", cfg.GitHubAppID)
	} else if cfg.GitHubToken == "" {
		slog.Warn("neither GITHUB_APP_ID nor GITHUB_TOKEN is set — ingestd will skip all GitHub GraphQL syncs")
	}

	live := settings.NewLive(ctx, dbPool.Raw(), cfg.DatabaseURL)
	live.Start(ctx)

	coordinator := ingest.NewCoordinator(dbPool, ghClient, ingest.CoordinatorConfig{
		Token:    tokenFn,
		Settings: live.Current,
	})
	live.OnChange(coordinator.ApplySettings)

	coordinator.Start()
	defer func() {
		_ = coordinator.Shutdown(5 * time.Second)
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"service": "ingestd",
			"time":    time.Now().UTC(),
		})
	})

	mux.HandleFunc("GET /sync/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(coordinator.Status())
	})

	// Manual admin trigger: enqueue up to `limit` eligible repositories that
	// have no ingest_jobs row yet. Not a priority mechanism — it's the same
	// FIFO queue everything else uses, just nudged to check sooner. See D9;
	// no repository priority/ranking lives in the crawl path.
	mux.HandleFunc("POST /sync/issues", func(w http.ResponseWriter, r *http.Request) {
		count, err := ingest.EnqueueInitialJobs(r.Context(), dbPool,
			live.Current().Int(settings.IngestEnqueueBatchSize))
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accepted": true,
			"enqueued": count,
		})
	})

	// Manual admin trigger for one repository: enqueues an "initial" job
	// due immediately, or bumps an existing job's next_retry_at to now.
	mux.HandleFunc("POST /sync/priority/{id}", func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		repoID, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, `{"error":"invalid repository id"}`, http.StatusBadRequest)
			return
		}

		if err := dbPool.Exec(r.Context(),
			`INSERT INTO ingest_jobs (repository_id, job_type, status, next_retry_at)
			 VALUES ($1, 'initial', 'pending', now())
			 ON CONFLICT (repository_id) WHERE status IN ('pending', 'running')
			 DO UPDATE SET next_retry_at = now()`,
			repoID,
		); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accepted":      true,
			"repository_id": repoID,
		})
	})

	port := 8002
	if pStr := os.Getenv("SYNCD_PORT"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil {
			port = p
		}
	} else if pStr := os.Getenv("INGESTION_PORT"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil {
			port = p
		}
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		slog.Info("ingestd HTTP server listening", "port", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("ingestd HTTP server failed", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("ingestd shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("ingestd server forced shutdown", "error", err)
	}

	slog.Info("ingestd shutdown complete")
}
