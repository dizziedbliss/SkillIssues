package ingest

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

func TestIsResourceLimitError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{&github.GraphQLError{Type: "RESOURCE_LIMITS_EXCEEDED", Message: "Resource limits for this query exceeded."}, true},
		{fmt.Errorf("fetch: %w", context.DeadlineExceeded), true},
		{&github.HTTPError{Status: 502}, true},
		// The text alone proves nothing: only the type counts.
		{errors.New("github graphql: RESOURCE_LIMITS_EXCEEDED"), false},
		{&github.HTTPError{Status: 401}, false},
		{github.ErrRepositoryNotFound, false},
		{fmt.Errorf("fetch: %w", context.Canceled), false},
	}
	for _, c := range cases {
		if got := isResourceLimitError(c.err); got != c.want {
			t.Errorf("isResourceLimitError(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

func TestRateLimitPauseHonoursRetryAfter(t *testing.T) {
	snap := settings.Defaults()
	base := snap.Seconds(settings.IngestRateLimitPauseSeconds)
	if got := rateLimitPause(snap, 0); got != base {
		t.Errorf("no Retry-After: pause %v, want the setting %v", got, base)
	}
	if got := rateLimitPause(snap, base+time.Hour); got != base+time.Hour {
		t.Errorf("longer Retry-After: pause %v, want %v", got, base+time.Hour)
	}
	if got := rateLimitPause(snap, time.Second); got != base {
		t.Errorf("shorter Retry-After: pause %v, want the setting %v", got, base)
	}
}
