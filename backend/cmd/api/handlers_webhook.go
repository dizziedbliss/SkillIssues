package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/auth"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (h *handlers) webhookGitHub(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			middleware.RespondError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
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
	// ErrNotFound means ON CONFLICT DO NOTHING: an already-seen delivery.
	if err := h.db.QueryRow(r.Context(),
		`INSERT INTO webhook_events (delivery_id, event_name) VALUES ($1, $2)
		 ON CONFLICT (delivery_id) DO NOTHING RETURNING delivery_id`,
		[]any{delivery, eventName},
		func(row pgx.Row) error { return row.Scan(&insertedDelivery) },
	); err != nil && !errors.Is(err, db.ErrNotFound) {
		slog.Error("webhook: failed to record delivery", "delivery_id", delivery, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to record delivery")
		return
	}

	// D2a: free closed/reopened signal for repos where the App is
	// installed — no API call needed, unlike the on-demand freshness check.
	if insertedDelivery != "" && eventName == "issues" {
		var payload struct {
			Action string `json:"action"`
			Issue  struct {
				Number    int    `json:"number"`
				State     string `json:"state"`
				UpdatedAt string `json:"updated_at"`
			} `json:"issue"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		}
		// A payload GitHub signed but we can't read won't read better on
		// redelivery: it is counted and accepted.
		if err := json.Unmarshal(body, &payload); err != nil {
			metrics.WebhookErrors.WithLabelValues("parse").Inc()
			slog.Warn("webhook: failed to parse issues event", "delivery_id", delivery, "error", err)
		} else if payload.Action == "closed" || payload.Action == "reopened" {
			updatedAt, err := time.Parse(time.RFC3339, payload.Issue.UpdatedAt)
			if err != nil {
				metrics.WebhookErrors.WithLabelValues("timestamp").Inc()
				slog.Warn("webhook: failed to parse issue updated_at", "delivery_id", delivery, "error", err)
			} else {
				state := strings.ToUpper(payload.Issue.State)
				if err := h.issueSvc.SetStateByRepoAndNumber(r.Context(), payload.Repository.FullName, payload.Issue.Number, state, updatedAt); err != nil {
					metrics.WebhookErrors.WithLabelValues("write").Inc()
					slog.Error("webhook: failed to persist issue state", "delivery_id", delivery, "repository", payload.Repository.FullName, "number", payload.Issue.Number, "error", err)
					// Forget the delivery so a redelivery isn't skipped as seen.
					if derr := h.db.Exec(context.WithoutCancel(r.Context()), `DELETE FROM webhook_events WHERE delivery_id = $1`, delivery); derr != nil {
						slog.Error("webhook: failed to forget the unapplied delivery; a redelivery will be skipped", "delivery_id", delivery, "error", derr)
					}
					middleware.RespondError(w, http.StatusInternalServerError, "failed to apply the event")
					return
				}
			}
		}
	}

	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"accepted":    true,
		"delivery_id": delivery,
		"event":       eventName,
	})
}
