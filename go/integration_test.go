package main_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"
)

const baseURL = "http://localhost:8000"

func TestE2EAPIContracts(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}

	// 1. Healthcheck
	t.Run("GET /health", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/health")
		if err != nil {
			t.Fatalf("failed to GET /health: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		if body["status"] != "ok" {
			t.Errorf("expected status ok, got %v", body["status"])
		}
	})

	// 2. Metrics
	t.Run("GET /metrics", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/metrics")
		if err != nil {
			t.Fatalf("failed to GET /metrics: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "http_requests_total") {
			t.Errorf("expected Prometheus metrics, got: %s", string(body)[:min(100, len(body))])
		}
	})

	// 3. Issue catalog
	t.Run("GET /issues", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/issues?limit=5")
		if err != nil {
			t.Fatalf("failed to GET /issues: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		var issues []map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&issues)
		if len(issues) == 0 {
			t.Log("Note: issues list empty or pending sync")
		}
	})

	// 4. Preference options
	t.Run("GET /preferences/options", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/preferences/options")
		if err != nil {
			t.Fatalf("failed to GET /preferences/options: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		var options map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&options)
		if _, ok := options["options"]; !ok {
			t.Errorf("expected options list in response, got %v", options)
		}
	})

	// 5. Auth / Session Check (unauthenticated initially)
	t.Run("GET /me unauthorized", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/me")
		if err != nil {
			t.Fatalf("failed to GET /me: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
