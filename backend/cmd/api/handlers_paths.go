package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/learnpath"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// pathBase carries the catalog's walk settings into a path's issue picks.
func (h *handlers) pathBase() issue.Filters {
	cfg := h.live.Current()
	return issue.Filters{
		RareLanguageBelow:   cfg.Int(settings.IssuesRareLanguageThreshold),
		LanguageCountTTL:    cfg.Seconds(settings.IssuesLanguageCountTTLSeconds),
		RareProbe:           cfg.Millis(settings.IssuesRareProbeMs),
		ShuffleWalkMaxRepos: cfg.Int(settings.IssuesShuffleWalkMaxRepos),
		ShuffleWalkBudget:   cfg.Millis(settings.IssuesShuffleWalkBudgetMs),
	}
}

// getPathRoles lists the roles a path can be made of.
func (h *handlers) getPathRoles(w http.ResponseWriter, _ *http.Request) {
	middleware.RespondJSON(w, http.StatusOK, map[string]any{"roles": learnpath.Roles})
}

// getMyPath is the caller's path in progress, or null.
func (h *handlers) getMyPath(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	p, err := h.pathSvc.Current(r.Context(), userID)
	w.Header().Set("Cache-Control", "private, no-store")
	switch {
	case errors.Is(err, learnpath.ErrNoPath):
		middleware.RespondJSON(w, http.StatusOK, map[string]any{"path": nil})
	case err != nil:
		slog.Error("read learning path", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to read your path")
	default:
		middleware.RespondJSON(w, http.StatusOK, map[string]any{"path": p})
	}
}

// startPath begins a path of {"role"} for the caller.
func (h *handlers) startPath(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var in struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "send {\"role\": ...}")
		return
	}
	cfg := h.live.Current()
	p, err := h.pathSvc.Start(r.Context(), userID, in.Role, learnpath.Sizes{
		Beginner:     cfg.Int(settings.PathsBeginnerSteps),
		Intermediate: cfg.Int(settings.PathsIntermediateSteps),
		PickPages:    cfg.Int(settings.PathsPickPages),
	}, h.pathBase())
	h.respondPath(w, r, userID, p, err, http.StatusCreated)
}

// swapPathStep replaces a step that isn't done with another issue.
func (h *handlers) swapPathStep(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	pos, err := strconv.Atoi(r.PathValue("position"))
	if err != nil || pos <= 0 {
		middleware.RespondError(w, http.StatusNotFound, "no such step")
		return
	}
	p, err := h.pathSvc.Swap(r.Context(), userID, pos, h.live.Current().Int(settings.PathsPickPages), h.pathBase())
	h.respondPath(w, r, userID, p, err, http.StatusOK)
}

// abandonPath gives up the caller's path in progress.
func (h *handlers) abandonPath(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	err := h.pathSvc.Abandon(r.Context(), userID)
	switch {
	case errors.Is(err, learnpath.ErrNoPath):
		middleware.RespondError(w, http.StatusNotFound, "you have no path in progress")
	case err != nil:
		slog.Error("abandon learning path", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to give up the path")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *handlers) respondPath(w http.ResponseWriter, r *http.Request, userID int, p learnpath.Path, err error, ok int) {
	switch {
	case errors.Is(err, learnpath.ErrUnknownRole):
		middleware.RespondError(w, http.StatusUnprocessableEntity, "no such role")
	case errors.Is(err, learnpath.ErrHasPath):
		middleware.RespondError(w, http.StatusConflict, "you already have a path in progress: finish or give it up first")
	case errors.Is(err, learnpath.ErrNoPath):
		middleware.RespondError(w, http.StatusNotFound, "you have no path in progress")
	case errors.Is(err, learnpath.ErrNoStep):
		middleware.RespondError(w, http.StatusNotFound, "no such step")
	case errors.Is(err, learnpath.ErrStepDone):
		middleware.RespondError(w, http.StatusConflict, "that step is done")
	case errors.Is(err, learnpath.ErrNoIssues):
		middleware.RespondError(w, http.StatusUnprocessableEntity, "no open, unclaimed issues fit right now: try another role later")
	case err != nil:
		respondIssueQueryError(w, r, err)
		slog.Error("learning path", "user_id", userID, "error", err)
	default:
		w.Header().Set("Cache-Control", "private, no-store")
		middleware.RespondJSON(w, ok, map[string]any{"path": p})
	}
}
