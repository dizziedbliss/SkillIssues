package main

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/dizziedbliss/skillissues/internal/claim"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// claimedStore holds which issues are claimed, re-read every
// api.claims.refresh_seconds so a list never queries for it. A claim made
// or released here is applied at once, so its author sees it straight away.
type claimedStore struct{ v atomic.Pointer[map[int]bool] }

func (c *claimedStore) get() map[int]bool {
	if p := c.v.Load(); p != nil {
		return *p
	}
	return nil
}

// set marks one issue claimed or not, copying the set (readers never lock).
func (c *claimedStore) set(issueID int, claimed bool) {
	for {
		old := c.v.Load()
		next := map[int]bool{}
		if old != nil {
			for id := range *old {
				next[id] = true
			}
		}
		if claimed {
			next[issueID] = true
		} else {
			delete(next, issueID)
		}
		if c.v.CompareAndSwap(old, &next) {
			return
		}
	}
}

// refreshClaimedLoop re-reads the claimed issues until ctx ends. A failed read
// is logged and counted, and the last good one stands.
func refreshClaimedLoop(ctx context.Context, svc *claim.Service, current func() *settings.Snapshot, store *claimedStore) {
	for {
		cfg := current()
		ids, err := svc.ActiveIssues(ctx)
		if err != nil {
			metrics.ClaimsRefreshErrors.Inc()
			slog.Error("claims: refreshing claimed issues failed; keeping the last read", "error", err)
		} else {
			store.v.Store(&ids)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(cfg.Seconds(settings.ClaimsRefreshSeconds)):
		}
	}
}
