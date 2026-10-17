package ingest

import (
	"testing"
	"time"
)

func TestEligible(t *testing.T) {
	recent := time.Now().Add(-24 * time.Hour)
	old := time.Now().Add(-1000 * 24 * time.Hour)
	goLang, empty := "Go", ""
	ok := RepoFacts{IssueCount: 3, PrimaryLanguage: &goLang, PushedAt: &recent, Stars: 25}

	cases := []struct {
		name   string
		edit   func(*RepoFacts)
		reason string
	}{
		{"passes", func(*RepoFacts) {}, ""},
		{"fork", func(f *RepoFacts) { f.IsFork = true }, "is_fork"},
		{"archived", func(f *RepoFacts) { f.IsArchived = true }, "archived"},
		{"no issues", func(f *RepoFacts) { f.IssueCount = 0 }, "no_issues"},
		{"no language", func(f *RepoFacts) { f.PrimaryLanguage = nil }, "no_primary_language"},
		{"empty language", func(f *RepoFacts) { f.PrimaryLanguage = &empty }, "no_primary_language"},
		{"never pushed", func(f *RepoFacts) { f.PushedAt = nil }, "stale"},
		{"stale", func(f *RepoFacts) { f.PushedAt = &old }, "stale"},
		{"below floor", func(f *RepoFacts) { f.Stars = 24 }, "below_star_floor"},
		{"several", func(f *RepoFacts) { f.IsArchived = true; f.Stars = 1 }, "archived;below_star_floor"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := ok
			c.edit(&f)
			got, reason := Eligible(f, 25, 730)
			if got != (c.reason == "") || reason != c.reason {
				t.Fatalf("Eligible = %v, %q; want %v, %q", got, reason, c.reason == "", c.reason)
			}
		})
	}
}
