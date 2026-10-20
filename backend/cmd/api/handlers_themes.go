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
	t, err := h.themeSvc.Active(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		respondThemeError(w, r, err)
		return
	}
	respondTheme(w, http.StatusOK, t)
}

// setActiveTheme applies {"id": "..."} (own or public), or {"id": null} for
// the default.
func (h *handlers) setActiveTheme(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID *string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "body must be {\"id\": \"…\"} or {\"id\": null}")
		return
	}
	id := ""
	if body.ID != nil {
		id = *body.ID
	}
	userID := middleware.GetUserID(r.Context())
	if err := h.themeSvc.SetActive(r.Context(), userID, id); err != nil {
		respondThemeError(w, r, err)
		return
	}
	h.getActiveTheme(w, r)
}
