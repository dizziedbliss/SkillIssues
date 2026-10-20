package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/dizziedbliss/skillissues/internal/theme"
)

// Themes restyle the signed-in app (internal/theme). Every response is
// no-store: a theme is per account, and a shared one changes when its author
// saves it.

func (h *handlers) themeLimits() theme.Limits {
	cfg := h.live.Current()
	return theme.Limits{MaxPerUser: cfg.Int(settings.ThemesMaxPerUser), MaxBytes: cfg.Int(settings.ThemesMaxBytes)}
}

func respondTheme(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Cache-Control", "no-store")
	middleware.RespondJSON(w, status, v)
}

// respondThemeError maps the store's errors; an invalid theme answers with
// every problem found, for the editor to show.
func respondThemeError(w http.ResponseWriter, r *http.Request, err error) {
	var problems theme.Problems
	switch {
	case errors.As(err, &problems):
		w.Header().Set("Cache-Control", "no-store")
		middleware.RespondJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "the theme is not valid", "problems": problems})
	case errors.Is(err, theme.ErrNotFound):
		middleware.RespondError(w, http.StatusNotFound, "theme not found")
	case errors.Is(err, theme.ErrTooMany):
		middleware.RespondError(w, http.StatusConflict, "you have reached the most themes an account may keep; delete one first")
	default:
		slog.Error("theme request failed", "route", r.Pattern, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "theme request failed")
	}
}

func decodeThemeInput(w http.ResponseWriter, r *http.Request) (theme.Input, bool) {
	var in theme.Input
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "body must be {\"toml\", \"css\", \"public\"}")
		return in, false
	}
	return in, true
}

func (h *handlers) listThemes(w http.ResponseWriter, r *http.Request) {
	themes, err := h.themeSvc.List(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		respondThemeError(w, r, err)
		return
	}
	respondTheme(w, http.StatusOK, themes)
}

// getTheme serves a theme its owner, or anyone when it is public; signed out
// is fine for a public one.
func (h *handlers) getTheme(w http.ResponseWriter, r *http.Request) {
	t, err := h.themeSvc.Get(r.Context(), middleware.GetUserID(r.Context()), r.PathValue("id"))
	if err != nil {
		respondThemeError(w, r, err)
		return
	}
	respondTheme(w, http.StatusOK, t)
}

// validateTheme checks {"toml", "css"} as a save would, storing nothing:
// 200 {"valid": true, "name"}, or 422 with every problem and its line.
// Open to anyone, so a theme can be written and checked before signing in.
func (h *handlers) validateTheme(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeThemeInput(w, r)
	if !ok {
		return
	}
	name, err := theme.Check(in, h.themeLimits())
	if err != nil {
		respondThemeError(w, r, err)
		return
	}
	respondTheme(w, http.StatusOK, map[string]any{"valid": true, "name": name})
}

func (h *handlers) createTheme(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeThemeInput(w, r)
	if !ok {
		return
	}
	t, err := h.themeSvc.Create(r.Context(), middleware.GetUserID(r.Context()), in, h.themeLimits())
	if err != nil {
		respondThemeError(w, r, err)
		return
	}
	respondTheme(w, http.StatusCreated, t)
}

func (h *handlers) updateTheme(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeThemeInput(w, r)
	if !ok {
		return
	}
	t, err := h.themeSvc.Update(r.Context(), middleware.GetUserID(r.Context()), r.PathValue("id"), in, h.themeLimits())
	if err != nil {
		respondThemeError(w, r, err)
		return
	}
	respondTheme(w, http.StatusOK, t)
}

func (h *handlers) deleteTheme(w http.ResponseWriter, r *http.Request) {
	if err := h.themeSvc.Delete(r.Context(), middleware.GetUserID(r.Context()), r.PathValue("id")); err != nil {
		respondThemeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getActiveTheme returns the theme applied to the caller's account, or null.
func (h *handlers) getActiveTheme(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	t, err := h.themeSvc.Active(r.Context(), userID)
	if err != nil {
		respondThemeError(w, r, err)
		return
	}
	if t != nil {
		respondTheme(w, http.StatusOK, t)
		return
	}
	// No stored theme: a built-in applied as it is, or the default (null).
	preset, err := h.themeSvc.ActivePreset(r.Context(), userID)
	if err != nil {
		respondThemeError(w, r, err)
		return
	}
	if preset != "" {
		respondTheme(w, http.StatusOK, map[string]string{"preset": preset})
		return
	}
	respondTheme(w, http.StatusOK, nil)
}

// setActiveTheme applies {"id": "..."} (own or public), {"preset": "nord"}
// (a built-in as it is), or {"id": null} for the default.
func (h *handlers) setActiveTheme(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID     *string `json:"id"`
		Preset *string `json:"preset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (body.ID != nil && body.Preset != nil) {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "body must be {\"id\": \"…\"}, {\"preset\": \"…\"} or {\"id\": null}")
		return
	}
	userID := middleware.GetUserID(r.Context())
	var err error
	if body.Preset != nil {
		err = h.themeSvc.SetActivePreset(r.Context(), userID, *body.Preset)
	} else {
		id := ""
		if body.ID != nil {
			id = *body.ID
		}
		err = h.themeSvc.SetActive(r.Context(), userID, id)
	}
	if err != nil {
		respondThemeError(w, r, err)
		return
	}
	h.getActiveTheme(w, r)
}
