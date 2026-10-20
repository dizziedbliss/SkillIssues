package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/recommendation"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

func (h *handlers) getRecommendations(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	limit := 3
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = min(l, h.live.Current().Int(settings.RecommendationsMaxLimit))
		}
	}

	// Redis Cache lookup for default limit
	if limit == 3 && h.cache != nil {
		if cached := h.cache.GetRecommendations(r.Context(), userID); cached != nil {
			h.respondRecommendations(w, r, cached)
			return
		}
	}

	p, err := h.profileSvc.GetFullProfile(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to load developer profile")
		return
	}

	cfg := h.live.Current()
	base := issue.Filters{
		RareLanguageBelow:   cfg.Int(settings.IssuesRareLanguageThreshold),
		LanguageCountTTL:    cfg.Seconds(settings.IssuesLanguageCountTTLSeconds),
		RareProbe:           cfg.Millis(settings.IssuesRareProbeMs),
		ShuffleWalkMaxRepos: cfg.Int(settings.IssuesShuffleWalkMaxRepos),
		ShuffleWalkBudget:   cfg.Millis(settings.IssuesShuffleWalkBudgetMs),
	}
	candidates, err := h.issueSvc.RecommendationCandidates(r.Context(), base,
		p.PreferredLanguages, p.TargetLevel, limit, cfg.Int(settings.RecommendationsCandidatePool))
	if err != nil {
		respondIssueQueryError(w, r, err)
		return
	}

	scored := recommendation.ScoreCandidates(p, candidates, limit)
	if scored == nil {
		scored = []issue.Issue{}
	}

	data, err := json.Marshal(scored)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to encode recommendations")
		return
	}

	if recTTL := h.live.Current().Seconds(settings.CacheRecommendationsTTLSecond); limit == 3 && recTTL > 0 && h.cache != nil {
		h.cache.SetRecommendations(r.Context(), userID, data, recTTL)
	}

	h.respondRecommendations(w, r, data)
}

// respondRecommendations sends data with an ETag (D15), responding 304
// without a body when the client's If-None-Match already matches — the
// client's cached copy (keyed by profile version, per D15) is still valid.
func (h *handlers) respondRecommendations(w http.ResponseWriter, r *http.Request, data []byte) {
	sum := sha256.Sum256(data)
	etag := `"` + hex.EncodeToString(sum[:]) + `"`

	w.Header().Set("ETag", etag)
	if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
