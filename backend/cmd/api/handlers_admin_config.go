package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/dizziedbliss/skillissues/internal/auth"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// adminGetConfig lists every live-reloadable setting with its current value,
// default, type and bounds.
//
// Only keys in the settings registry are reachable here. Secrets live in the
// environment and are deliberately absent: this endpoint sits behind an admin
// token, and a token that could read credentials would be a much bigger
// liability than one that can retune a page size.
func (h *handlers) adminGetConfig(w http.ResponseWriter, r *http.Request) {
	entries := settings.Describe(h.live.Current())
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })

	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"settings": entries,
		"count":    len(entries),
	})
}

// adminPutConfig updates one setting and returns the value now in effect.
//
// Validation happens twice on purpose. Here it produces a useful message, but
// the database trigger is the real boundary: it also guards direct psql writes,
// which is the escape hatch when a bad value has made the API itself unhealthy.
func (h *handlers) adminPutConfig(w http.ResponseWriter, r *http.Request) {
	key := settings.Key(r.PathValue("key"))
	if key == "" {
		middleware.RespondError(w, http.StatusBadRequest, "setting key is required")
		return
	}

	var body struct {
		Value any `json:"value"`
	}
	dec := json.NewDecoder(r.Body)
	dec.UseNumber() // keep integers exact rather than routing them through float64
	if err := dec.Decode(&body); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "body must be JSON of the form {\"value\": ...}")
		return
	}
	if body.Value == nil {
		middleware.RespondError(w, http.StatusBadRequest, "\"value\" is required")
		return
	}

	if err := settings.Set(r.Context(), h.db.Raw(), key, body.Value, adminActor(r)); err != nil {
		if errors.Is(err, settings.ErrUnknownKey) {
			middleware.RespondError(w, http.StatusNotFound, err.Error())
			return
		}
		// A trigger rejection is the caller's fault, not the server's.
		middleware.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	slog.Info("setting updated", "key", key, "actor", adminActor(r))

	// The NOTIFY that this write triggered is handled asynchronously, so report
	// the value that was accepted rather than re-reading a snapshot that may not
	// have been swapped in yet.
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"key":     key,
		"value":   body.Value,
		"applied": true,
	})
}

// sessionTTL resolves the configured session lifetime for cookie expiry.
func (h *handlers) sessionTTL() time.Duration {
	if h.live == nil {
		return auth.SessionDuration
	}
	if d := h.live.Current().Hours(settings.SessionTTLHours); d > 0 {
		return d
	}
	return auth.SessionDuration
}

// adminActor labels the audit row. The admin token is shared, so this records
// where a change came from, not who made it.
func adminActor(r *http.Request) string {
	if id := middleware.GetRequestID(r.Context()); id != "" {
		return "admin-api:" + id
	}
	return "admin-api"
}
