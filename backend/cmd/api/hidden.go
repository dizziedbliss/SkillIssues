package main

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/report"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// hiddenStore holds what reports have taken out of lists, re-read every
// api.reports.hidden_refresh_seconds so a list never queries for it.
type hiddenStore struct{ v atomic.Pointer[report.Hidden] }

func (h *hiddenStore) get() report.Hidden {
	if p := h.v.Load(); p != nil {
		return *p
	}
	return report.Hidden{}
}

// issues is the catalog's view of it (issue.Service.SetHidden).
func (h *hiddenStore) issues() issue.Hidden {
	cur := h.get()
	return issue.Hidden{Issues: cur.Issues, Repositories: cur.Repositories}
}

// refreshHiddenLoop re-reads the hidden targets until ctx ends. A failed read
// is logged and counted, and the last good one stands.
func refreshHiddenLoop(ctx context.Context, svc *report.Service, current func() *settings.Snapshot, store *hiddenStore) {
	for {
		cfg := current()
		_ = store.refresh(ctx, svc, cfg)
		select {
		case <-ctx.Done():
			return
		case <-time.After(cfg.Seconds(settings.ReportsHiddenRefreshSeconds)):
		}
	}
}

// refresh re-reads the hidden targets now (a moderator's decision applies at
// once). A failed read is logged and counted, and the last good one stands.
func (h *hiddenStore) refresh(ctx context.Context, svc *report.Service, cfg *settings.Snapshot) error {
	read, err := svc.Hidden(ctx, cfg.Int(settings.ReportsHideAfter))
	if err != nil {
		metrics.ReportsHiddenRefreshErrors.Inc()
		slog.Error("reports: refreshing hidden targets failed; keeping the last read", "error", err)
		return err
	}
	h.v.Store(&read)
	return nil
}
