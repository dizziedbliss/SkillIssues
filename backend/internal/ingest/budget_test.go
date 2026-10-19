package ingest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
)

// budgetIdentity is an identity of the test's own, so it never reads or
// overwrites the running installation's budget rows.
func budgetIdentity(t *testing.T, pool *db.Pool) string {
	t.Helper()
	id := fmt.Sprintf("budget-test:%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM github_budget WHERE identity = $1`, id); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return id
}

func mustUpdateBudget(t *testing.T, pool *db.Pool, lane Lane, identity string, limit, remaining int, resetAt time.Time) {
	t.Helper()
	rl := github.GraphQLRateLimit{Limit: limit, Remaining: remaining, ResetAt: resetAt}
	if err := UpdateBudget(context.Background(), pool, lane, identity, rl); err != nil {
		t.Fatalf("update budget: %v", err)
	}
}

func mustProceed(t *testing.T, pool *db.Pool, lane Lane, identity string, floor, reserve int) bool {
	t.Helper()
	ok, err := CanProceed(context.Background(), pool, lane, identity, floor, reserve)
	if err != nil {
		t.Fatalf("CanProceed(%s): %v", lane, err)
	}
	return ok
}

// The crawler lane stops while the reserve is still left; the interactive
// lane spends into it, down to the floor.
func TestCanProceedCrawlerLeavesReserve(t *testing.T) {
	pool := testPool(t)
	id := budgetIdentity(t, pool)
	reset := time.Now().Add(30 * time.Minute)

	// 30% of 12,500 is 3,750; the crawler's floor of 100 sits above it.
	for _, tc := range []struct {
		remaining   int
		crawler     bool
		interactive bool
	}{
		{remaining: 5000, crawler: true, interactive: true},
		{remaining: 3850, crawler: true, interactive: true},
		{remaining: 3849, crawler: false, interactive: true},
		{remaining: 100, crawler: false, interactive: true},
		{remaining: 99, crawler: false, interactive: false},
	} {
		mustUpdateBudget(t, pool, LaneCrawler, id, 12500, tc.remaining, reset)
		if got := mustProceed(t, pool, LaneCrawler, id, 100, 30); got != tc.crawler {
			t.Errorf("remaining %d: crawler proceeds = %v, want %v", tc.remaining, got, tc.crawler)
		}
		if got := mustProceed(t, pool, LaneInteractive, id, 100, 30); got != tc.interactive {
			t.Errorf("remaining %d: interactive proceeds = %v, want %v", tc.remaining, got, tc.interactive)
		}
	}

	// A reserve of 0 is the old behaviour: the crawler spends to its floor.
	mustUpdateBudget(t, pool, LaneCrawler, id, 12500, 150, reset)
	if !mustProceed(t, pool, LaneCrawler, id, 100, 0) {
		t.Error("reserve 0: crawler stopped above its floor")
	}
}

// Each lane reads the counter the other lane last saw: it is one counter.
func TestCanProceedReadsEveryLane(t *testing.T) {
	pool := testPool(t)
	id := budgetIdentity(t, pool)
	reset := time.Now().Add(30 * time.Minute)

	// The crawler last saw plenty; the interactive lane has since seen the
	// budget nearly spent. The lower value is the newer one.
	mustUpdateBudget(t, pool, LaneCrawler, id, 12500, 9000, reset)
	mustUpdateBudget(t, pool, LaneInteractive, id, 12500, 2000, reset)
	if mustProceed(t, pool, LaneCrawler, id, 100, 30) {
		t.Error("crawler proceeded on its own stale reading of 9,000")
	}

	// A reading from the previous window describes a budget that has been
	// refilled since, and no longer counts.
	mustUpdateBudget(t, pool, LaneInteractive, id, 12500, 50, reset.Add(-time.Hour))
	if !mustProceed(t, pool, LaneCrawler, id, 100, 30) {
		t.Error("crawler stopped on a reading from an earlier window")
	}
}

// Past the reset time the budget is full again, whatever was last seen.
func TestCanProceedAfterReset(t *testing.T) {
	pool := testPool(t)
	id := budgetIdentity(t, pool)

	mustUpdateBudget(t, pool, LaneCrawler, id, 12500, 0, time.Now().Add(-time.Second))
	if !mustProceed(t, pool, LaneCrawler, id, 100, 30) {
		t.Error("crawler stopped although the window has reset")
	}
}

// With no telemetry at all, the first call goes ahead so there can be some.
func TestCanProceedWithoutTelemetry(t *testing.T) {
	pool := testPool(t)
	id := budgetIdentity(t, pool)

	for _, lane := range []Lane{LaneCrawler, LaneInteractive} {
		if !mustProceed(t, pool, lane, id, 100, 30) {
			t.Errorf("%s stopped with no telemetry", lane)
		}
	}
}

// A secondary limit is GitHub's verdict on the credential, so it pauses every
// lane, not only the one that tripped it.
func TestCanProceedSecondaryLimitPausesEveryLane(t *testing.T) {
	pool := testPool(t)
	id := budgetIdentity(t, pool)
	ctx := context.Background()

	mustUpdateBudget(t, pool, LaneInteractive, id, 12500, 12000, time.Now().Add(30*time.Minute))
	if err := MarkSecondaryLimit(ctx, pool, LaneCrawler, id, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	for _, lane := range []Lane{LaneCrawler, LaneInteractive} {
		if mustProceed(t, pool, lane, id, 100, 30) {
			t.Errorf("%s proceeded during a secondary-limit pause", lane)
		}
	}

	if err := MarkSecondaryLimit(ctx, pool, LaneCrawler, id, time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if !mustProceed(t, pool, LaneInteractive, id, 100, 30) {
		t.Error("interactive still paused after the pause ended")
	}
}
