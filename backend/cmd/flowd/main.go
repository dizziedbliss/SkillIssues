// flowd relays the flow events go-api and ingestd publish to Redis to the
// public live visualizer, over GET /stream.
//
// It holds no credential beyond its own: it reads its live settings as
// skillissues_flow (SELECT on settings, nothing else), subscribes to Redis,
// and, when PROMETHEUS_URL is set, reads cAdvisor network rates for the
// links nothing is instrumented on. Only /stream is reachable from outside;
// /health and /metrics are internal.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dizziedbliss/skillissues/internal/config"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/flow"
	"github.com/dizziedbliss/skillissues/internal/flowd"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/redis/go-redis/v9"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// flowd reads only its settings: a pool of two covers the reload query
	// and /metrics' pool gauges. The LISTEN connection is separate.
	dbCfg := db.DefaultConfig(cfg.DatabaseURL)
	dbCfg.MaxConns, dbCfg.MinConns = 2, 1
	if cfg.DBMaxConns > 0 {
		dbCfg.MaxConns = int32(cfg.DBMaxConns)
	}
	var pool *db.Pool
	for attempt := 1; attempt <= 15; attempt++ {
		pool, err = db.New(ctx, dbCfg)
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
	defer pool.Close()

	metrics.RegisterFlowd()
	metrics.RegisterDBPool(pool.Raw())

	live := settings.NewLive(ctx, pool.Raw(), cfg.DatabaseURL)
	live.Start(ctx)

	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("invalid REDIS_URL", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(opts)
	defer rdb.Close()

	relay := flowd.New(live.Current)
	go relay.RunDensity(ctx)
	go subscribe(ctx, rdb, relay)

	if cfg.PrometheusURL != "" {
		ambient := flowd.NewAmbient(cfg.PrometheusURL, settings.OutboundClient(live, nil), live.Current)
		go ambient.Run(ctx, relay)
	} else {
		slog.Warn("PROMETHEUS_URL is not set — ambient link rates are off")
	}
	if len(cfg.FlowAllowedOrigins) == 0 {
		slog.Warn("FLOW_ALLOWED_ORIGINS is not set — no browser origin can read /stream")
	}

	mux := http.NewServeMux()
	mux.Handle("GET /stream", relay.Stream(cfg.FlowAllowedOrigins))
	mux.Handle("GET /metrics", metrics.Handler())
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		// Without Redis there is nothing to relay, so flowd is not healthy.
		callCtx, cancel := context.WithTimeout(r.Context(), live.Current().Millis(settings.FlowPublishTimeoutMs))
		defer cancel()
		status, code := "ok", http.StatusOK
		redisStatus := "connected"
		if err := rdb.Ping(callCtx).Err(); err != nil {
			status, code, redisStatus = "degraded", http.StatusServiceUnavailable, "unreachable"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"status": status, "service": "flowd", "redis": redisStatus, "time": time.Now().UTC(),
		}); err != nil {
			slog.Warn("flowd: write health response", "error", err)
		}
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		// No WriteTimeout: a stream stays open. Each write has its own
		// deadline (internal/flowd), and streams end with ctx on shutdown.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	go func() {
		slog.Info("flowd listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("flowd HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("flowd shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("flowd forced shutdown", "error", err)
	}
}

// subscribe feeds every published batch to the relay until ctx is done.
// go-redis reconnects and resubscribes on its own after Redis drops.
func subscribe(ctx context.Context, rdb *redis.Client, relay *flowd.Relay) {
	ps := rdb.Subscribe(ctx, flow.Channel)
	defer ps.Close()
	ch := ps.Channel()
	slog.Info("flowd subscribed", "channel", flow.Channel)
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				if ctx.Err() == nil {
					// Nothing would be relayed again; exit so the container restarts.
					slog.Error("flowd: redis subscription closed")
					os.Exit(1)
				}
				return
			}
			relay.Ingest([]byte(msg.Payload))
		}
	}
}
