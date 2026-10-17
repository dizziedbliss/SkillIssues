package ingest

import (
	"strings"
	"time"
)

// RepoFacts is what eligibility is decided from, whichever path saw the
// repository: a dataset record (cmd/repoimport) or a search result (the
// Discovery Sweeper).
type RepoFacts struct {
	IsFork          bool
	IsArchived      bool
	IssueCount      int // open + closed, ever
	PrimaryLanguage *string
	PushedAt        *time.Time
	Stars           int
}

// Eligible applies D1's rule. The star floor is abuse control (anyone can
// open a fake issue on a trivially-created repo to burn crawl budget), not a
// quality ranking — there is deliberately no scoring here.
//
// RecomputeEligibility applies the same rule in SQL to stored rows — if one
// changes, both change. The exclusion list is checked by the callers, so this
// stays a pure function of the facts.
func Eligible(f RepoFacts, minStars, pushedWithinDays int) (bool, string) {
	var reasons []string
	if f.IsFork {
		reasons = append(reasons, "is_fork")
	}
	if f.IsArchived {
		reasons = append(reasons, "archived")
	}
	if f.IssueCount <= 0 {
		reasons = append(reasons, "no_issues")
	}
	if f.PrimaryLanguage == nil || *f.PrimaryLanguage == "" {
		reasons = append(reasons, "no_primary_language")
	}
	if f.PushedAt == nil || time.Since(*f.PushedAt) > time.Duration(pushedWithinDays)*24*time.Hour {
		reasons = append(reasons, "stale")
	}
	if f.Stars < minStars {
		reasons = append(reasons, "below_star_floor")
	}
	if len(reasons) > 0 {
		return false, strings.Join(reasons, ";")
	}
	return true, ""
}
