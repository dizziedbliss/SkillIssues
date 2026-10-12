package main

import (
	"net/http"

	"github.com/dizziedbliss/skillissues/internal/middleware"
)

// coContributionTrailer handles GET /github/co-contribution-trailer (D23).
// Returns the exact `Co-authored-by:` line desktop appends to a commit
// message when the user has opted into co-contribution credit — never
// constructed client-side, since a mis-formatted bot noreply email looks
// valid but never links on github.com.
func (h *handlers) coContributionTrailer(w http.ResponseWriter, r *http.Request) {
	if h.ghApp == nil {
		middleware.RespondError(w, http.StatusServiceUnavailable, "GitHub App is not configured")
		return
	}
	trailer, err := h.ghApp.BotCoAuthorTrailer(r.Context())
	if err != nil {
		middleware.RespondError(w, http.StatusBadGateway, "failed to resolve bot identity")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]string{"trailer": trailer})
}
