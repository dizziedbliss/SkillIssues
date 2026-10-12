package main

import (
	"strings"
	"testing"
	"time"
)

func lang(s string) *string { return &s }

func TestEligible(t *testing.T) {
	now := time.Now()
	recent := now.Add(-30 * 24 * time.Hour)
	stale := now.Add(-1000 * 24 * time.Hour)

	cases := []struct {
		name    string
		rec     repoRecord
		wantOK  bool
		wantHas string // substring expected in the skip reason, if !wantOK
	}{
		{
			name: "fully eligible",
			rec: repoRecord{
				Stars: 100, Issues: 5, PrimaryLanguage: lang("Go"), PushedAt: &recent,
			},
			wantOK: true,
		},
		{
			name:    "archived",
			rec:     repoRecord{Stars: 100, Issues: 5, PrimaryLanguage: lang("Go"), PushedAt: &recent, IsArchived: true},
			wantHas: "archived",
		},
		{
			name:    "fork",
			rec:     repoRecord{Stars: 100, Issues: 5, PrimaryLanguage: lang("Go"), PushedAt: &recent, IsFork: true},
			wantHas: "is_fork",
		},
		{
			name:    "no issues",
			rec:     repoRecord{Stars: 100, Issues: 0, PrimaryLanguage: lang("Go"), PushedAt: &recent},
			wantHas: "no_issues",
		},
		{
			name:    "no primary language",
			rec:     repoRecord{Stars: 100, Issues: 5, PrimaryLanguage: nil, PushedAt: &recent},
			wantHas: "no_primary_language",
		},
		{
			name:    "stale",
			rec:     repoRecord{Stars: 100, Issues: 5, PrimaryLanguage: lang("Go"), PushedAt: &stale},
			wantHas: "stale",
		},
		{
			name:    "no pushed_at at all",
			rec:     repoRecord{Stars: 100, Issues: 5, PrimaryLanguage: lang("Go"), PushedAt: nil},
			wantHas: "stale",
		},
		{
			name:    "below star floor",
			rec:     repoRecord{Stars: 1, Issues: 5, PrimaryLanguage: lang("Go"), PushedAt: &recent},
			wantHas: "below_star_floor",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, reason := eligible(c.rec, 25, 730)
			if ok != c.wantOK {
				t.Fatalf("eligible() = %v, reason=%q; want ok=%v", ok, reason, c.wantOK)
			}
			if !ok && !strings.Contains(reason, c.wantHas) {
				t.Fatalf("reason %q does not contain %q", reason, c.wantHas)
			}
		})
	}
}
