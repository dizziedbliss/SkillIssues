package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dizziedbliss/skillissues/internal/config"
)

// The desktop app reads the Login app's client id from the server; without
// one configured, sign-in reports itself off rather than half-working.
func TestGetGitHubDeviceConfig(t *testing.T) {
	for _, tc := range []struct {
		clientID string
		enabled  bool
	}{
		{"Iv1.abc123", true},
		{"", false},
	} {
		h := &handlers{cfg: &config.Config{GitHubLoginClientID: tc.clientID}}
		rec := httptest.NewRecorder()
		h.getGitHubDeviceConfig(rec, httptest.NewRequest(http.MethodGet, "/auth/github/device", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		var got struct {
			Enabled  bool   `json:"enabled"`
			ClientID string `json:"client_id"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.Enabled != tc.enabled || got.ClientID != tc.clientID {
			t.Errorf("client id %q: got %+v", tc.clientID, got)
		}
	}
}
