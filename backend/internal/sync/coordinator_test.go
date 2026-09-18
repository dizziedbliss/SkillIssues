package sync

import (
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/github"
)

func TestRateLimitBudget(t *testing.T) {
	budget := &RateLimitBudget{}

	// Initial uninitialized state should allow proceed
	if !budget.CanProceed() {
		t.Errorf("expected initial budget to allow proceed")
	}

	// Update with healthy quota
	budget.Update(github.GraphQLRateLimit{
		Limit:     5000,
		Remaining: 4500,
		Used:      500,
		ResetAt:   time.Now().Add(1 * time.Hour),
		Cost:      1,
	})

	if !budget.CanProceed() {
		t.Errorf("expected budget with 4500 remaining to allow proceed")
	}

	snap := budget.Snapshot()
	if snap.Remaining != 4500 || snap.Limit != 5000 {
		t.Errorf("unexpected snapshot: %+v", snap)
	}

	// Update with low quota (< 100) and future reset time
	budget.Update(github.GraphQLRateLimit{
		Limit:     5000,
		Remaining: 50,
		Used:      4950,
		ResetAt:   time.Now().Add(30 * time.Minute),
		Cost:      1,
	})

	if budget.CanProceed() {
		t.Errorf("expected budget with 50 remaining to block proceed")
	}

	// If reset time has passed, should allow proceed
	budget.Update(github.GraphQLRateLimit{
		Limit:     5000,
		Remaining: 50,
		Used:      4950,
		ResetAt:   time.Now().Add(-1 * time.Minute),
		Cost:      1,
	})

	if !budget.CanProceed() {
		t.Errorf("expected budget with expired reset time to allow proceed")
	}
}
