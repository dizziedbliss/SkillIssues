package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("starting SkillIssues Priority Refresh Worker")

	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://api:8000"
	}

	syncdURL := os.Getenv("SYNCD_URL")
	if syncdURL == "" {
		syncdURL = os.Getenv("INGESTION_URL")
		if syncdURL == "" {
			syncdURL = "http://syncd:8002"
		}
	}

	intervalSecs := 900
	if val := os.Getenv("SYNC_INTERVAL_SECONDS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			intervalSecs = n
		}
	}
	interval := time.Duration(intervalSecs) * time.Second

	slog.Info("worker initialized", "api_url", apiURL, "syncd_url", syncdURL, "interval", interval.String())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	runCycle := func() {
		// 1. Probe main API gateway health status
		probeURL := fmt.Sprintf("%s/health", apiURL)
		req, _ := http.NewRequestWithContext(ctx, "GET", probeURL, nil)
		resp, err := httpClient.Do(req)
		if err != nil {
			slog.Warn("API health probe failed", "error", err)
		} else {
			_ = resp.Body.Close()
			slog.Info("API health probe successful", "status", resp.StatusCode)
		}

		// 2. Trigger priority open issue catalog synchronization via syncd
		syncURL := fmt.Sprintf("%s/sync/issues", syncdURL)
		syncReq, _ := http.NewRequestWithContext(ctx, "POST", syncURL, nil)
		syncResp, err := httpClient.Do(syncReq)
		if err != nil {
			slog.Warn("syncd issue refresh failed to connect", "error", err)
		} else {
			body, _ := io.ReadAll(syncResp.Body)
			_ = syncResp.Body.Close()
			slog.Info("issue refresh cycle triggered", "status", syncResp.StatusCode, "response", string(body))
		}
	}

	// Run initial cycle
	runCycle()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("priority refresh worker stopped cleanly")
			return
		case <-ticker.C:
			runCycle()
		}
	}
}
