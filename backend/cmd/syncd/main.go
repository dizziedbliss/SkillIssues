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

	"github.com/dizziedbliss/skillissues/internal/cache"
	"github.com/dizziedbliss/skillissues/internal/config"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	syncd "github.com/dizziedbliss/skillissues/internal/sync"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("starting SkillIssues Sync Coordinator (syncd)")

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Log presence only. Never log the token, a prefix of it, or its length:
	// a prefix leaks secret bytes and the length identifies the credential type.
	if cfg.GitHubToken == "" {
		slog.Warn("GITHUB_TOKEN is not set — syncd will skip all GitHub GraphQL syncs")
	} else {
		slog.Info("github token loaded")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize Database Pool with retries
	dbCfg := db.DefaultConfig(cfg.DatabaseURL)
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

	// Initialize Redis Cache
	cacheClient, err := cache.New(ctx, cfg.RedisURL)
	if err != nil {
		slog.Warn("redis connection warning (proceeding in degraded mode)", "error", err)
	} else {
		defer cacheClient.Close()
	}

	// Initialize GitHub Client
	ghClient := github.NewClient(cfg.GitHubGraphQLURL)

	// Initialize Sync Coordinator
	coordCfg := syncd.CoordinatorConfig{
		WorkerCount:         cfg.WorkerCount,
		PriorityWorkerCount: cfg.PriorityWorkerCount,
		OnDemandWorkerCount: cfg.OnDemandWorkerCount,
		SyncInterval:        time.Duration(cfg.SyncIntervalSecs) * time.Second,
		GitHubToken:         cfg.GitHubToken,
	}
	coordinator := syncd.NewCoordinator(dbPool, cacheClient, ghClient, coordCfg)
	coordinator.Start()
	defer func() {
		_ = coordinator.Shutdown(5 * time.Second)
	}()

	// Setup HTTP server for coordination & healthchecks
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"service": "syncd",
			"time":    time.Now().UTC(),
		})
	})

	mux.HandleFunc("GET /sync/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(coordinator.Status())
	})

	mux.HandleFunc("POST /sync/issues", func(w http.ResponseWriter, r *http.Request) {
		count, err := coordinator.TriggerRoutineSync(r.Context())
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

	mux.HandleFunc("POST /sync/priority/{id}", func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		repoID, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, `{"error":"invalid repository id"}`, http.StatusBadRequest)
			return
		}

		if err := coordinator.TriggerPrioritySync(r.Context(), repoID); err != nil {
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
		slog.Info("syncd HTTP server listening", "port", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("syncd HTTP server failed", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("syncd shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("syncd server forced shutdown", "error", err)
	}

	slog.Info("syncd shutdown complete")
}
