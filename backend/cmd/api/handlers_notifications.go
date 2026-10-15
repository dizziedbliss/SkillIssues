package main

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// getNotifications lists the caller's notifications, newest first.
// ?unread=true limits it to unread ones; unread_count always counts all.
func (h *handlers) getNotifications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	unreadOnly := false
	if v := r.URL.Query().Get("unread"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			middleware.RespondError(w, http.StatusUnprocessableEntity, "unread must be true or false")
			return
		}
		unreadOnly = b
	}
	limit := h.live.Current().Int(settings.NotificationsPageSize)
	items, unread, err := h.notificationSvc.List(r.Context(), userID, unreadOnly, limit)
	if err != nil {
		slog.Error("list notifications", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to fetch notifications")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"notifications": items,
		"unread_count":  unread,
	})
}

// markNotificationRead marks one of the caller's notifications read.
func (h *handlers) markNotificationRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		middleware.RespondError(w, http.StatusNotFound, "notification not found")
		return
	}
	found, err := h.notificationSvc.MarkRead(r.Context(), userID, id)
	if err != nil {
		slog.Error("mark notification read", "user_id", userID, "notification_id", id, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to update notification")
		return
	}
	if !found {
		middleware.RespondError(w, http.StatusNotFound, "notification not found")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]any{"id": id, "read": true})
}

// markAllNotificationsRead marks every unread notification of the caller read.
func (h *handlers) markAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	n, err := h.notificationSvc.MarkAllRead(r.Context(), userID)
	if err != nil {
		slog.Error("mark all notifications read", "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to update notifications")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, map[string]any{"marked": n})
}
