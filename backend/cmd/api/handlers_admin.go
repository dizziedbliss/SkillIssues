package main

import (
	"net/http"

	"github.com/dizziedbliss/skillissues/internal/middleware"
)

func (h *handlers) adminSyncRepository(w http.ResponseWriter, r *http.Request) {
	rawID := r.PathValue("id")
	if rawID == "" {
		middleware.RespondError(w, http.StatusBadRequest, "repository id is required")
		return
	}
	path, err := syncPath(rawID)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.proxyToSyncd(w, r, path)
}
