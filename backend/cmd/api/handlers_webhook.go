package main

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/dizziedbliss/skillissues/internal/auth"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (h *handlers) webhookGitHub(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	signature := r.Header.Get("X-Hub-Signature-256")
	delivery := r.Header.Get("X-GitHub-Delivery")
	if delivery == "" {
		delivery = uuid.New().String()
	}
	eventName := r.Header.Get("X-GitHub-Event")
	if eventName == "" {
		eventName = "unknown"
	}

	if h.cfg.GitHubWebhookSecret != "" {
		if !auth.VerifyWebhookSignature([]byte(h.cfg.GitHubWebhookSecret), body, signature) {
			middleware.RespondError(w, http.StatusUnauthorized, "invalid webhook signature")
			return
		}
	} else {
		middleware.RespondError(w, http.StatusServiceUnavailable, "webhook verification is not configured")
		return
	}

	var insertedDelivery string
	_ = h.db.QueryRow(r.Context(),
		`INSERT INTO webhook_events (delivery_id, event_name) VALUES ($1, $2)
		 ON CONFLICT (delivery_id) DO NOTHING RETURNING delivery_id`,
		[]any{delivery, eventName},
		func(row pgx.Row) error { return row.Scan(&insertedDelivery) },
	)

	if insertedDelivery != "" && eventName == "pull_request" {
		var payload struct {
			PullRequest struct {
				Merged  bool   `json:"merged"`
				HTMLURL string `json:"html_url"`
			} `json:"pull_request"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			if payload.PullRequest.Merged && payload.PullRequest.HTMLURL != "" {
				_, _ = h.contribSvc.ApplyMergedPRByUrl(r.Context(), payload.PullRequest.HTMLURL, h.cfg.GitHubToken)
			}
		}
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"accepted":    true,
		"delivery_id": delivery,
		"event":       eventName,
	})
}
