package xp

import (
	"fmt"
	"sort"
	"time"
)

// Event is one ledger row as Summarise reads it.
type Event struct {
	Source   string
	Key      string
	Amount   int
	Detail   Detail
	EarnedAt time.Time
}

// Summary is everything a profile shows that the ledger decides.
type Summary struct {
	Total     int             `json:"total"`
	Level     Level           `json:"level"`
	Merges    int             `json:"merges"`
	Languages []LanguageLevel `json:"languages"`
	Streak    Streak          `json:"streak"`
	Badges    []Badge         `json:"badges"`
}

// LanguageLevel is the XP earned in repositories of one main language.
type LanguageLevel struct {
	Language string `json:"language"`
	XP       int    `json:"xp"`
	Level    Level  `json:"level"`
}

// Streak counts UTC ISO weeks in a row with at least one merge. Current
// still counts while this week has none yet but last week had one.
type Streak struct {
	Current  int  `json:"current"`
	Best     int  `json:"best"`
	ThisWeek bool `json:"this_week"`
}

// Badge is a milestone and when it was reached.
type Badge struct {
	Key      string    `json:"key"`
	EarnedAt time.Time `json:"earned_at"`
}

// badge definitions, in display order. Each says when, walking the merges
// oldest first, it is reached; the walk records the first moment.
type badgeDef struct {
	key   string
	reach func(s *walk) bool
}

type walk struct {
	merges     int
	repos      map[int]bool
	languages  map[string]bool
	backlog    bool
	advanced   bool
	docs       bool
	streak     int
	bestStreak int
	total      int
	step       int
}

var badgeDefs = []badgeDef{
	{"first_merge", func(s *walk) bool { return s.merges >= 1 }},
	{"merges_5", func(s *walk) bool { return s.merges >= 5 }},
	{"merges_10", func(s *walk) bool { return s.merges >= 10 }},
	{"merges_25", func(s *walk) bool { return s.merges >= 25 }},
	{"merges_50", func(s *walk) bool { return s.merges >= 50 }},
	{"merges_100", func(s *walk) bool { return s.merges >= 100 }},
	{"repos_3", func(s *walk) bool { return len(s.repos) >= 3 }},
	{"repos_10", func(s *walk) bool { return len(s.repos) >= 10 }},
	{"languages_3", func(s *walk) bool { return len(s.languages) >= 3 }},
	{"languages_5", func(s *walk) bool { return len(s.languages) >= 5 }},
	{"docs_fix", func(s *walk) bool { return s.docs }},
	{"backlog_hero", func(s *walk) bool { return s.backlog }},
	{"advanced_fix", func(s *walk) bool { return s.advanced }},
	{"streak_4", func(s *walk) bool { return s.bestStreak >= 4 }},
	{"streak_12", func(s *walk) bool { return s.bestStreak >= 12 }},
	{"level_5", func(s *walk) bool { return LevelOf(s.total, s.step).Level >= 5 }},
	{"level_10", func(s *walk) bool { return LevelOf(s.total, s.step).Level >= 10 }},
	{"level_20", func(s *walk) bool { return LevelOf(s.total, s.step).Level >= 20 }},
}

// BadgeKeys lists every badge, in display order.
func BadgeKeys() []string {
	keys := make([]string, len(badgeDefs))
	for i, b := range badgeDefs {
		keys[i] = b.key
	}
	return keys
}

// Summarise totals a person's ledger at now.
func Summarise(events []Event, step int, now time.Time) Summary {
	sorted := append([]Event(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].EarnedAt.Before(sorted[j].EarnedAt) })

	s := Summary{Languages: []LanguageLevel{}, Badges: []Badge{}}
	byLanguage := map[string]int{}
	w := &walk{repos: map[int]bool{}, languages: map[string]bool{}, step: step}
	reached := map[string]bool{}
	var lastWeek time.Time // Monday of the last week with a merge
	weeks := map[time.Time]bool{}

	for _, e := range sorted {
		s.Total += e.Amount
		w.total = s.Total
		if e.Detail.Language != "" {
			byLanguage[e.Detail.Language] += e.Amount
		}
		if e.Source == SourceMerged || e.Source == SourceBackfill {
			s.Merges++
			w.merges++
			w.repos[e.Detail.RepositoryID] = true
			if e.Detail.Language != "" {
				w.languages[e.Detail.Language] = true
			}
			w.backlog = w.backlog || e.Detail.Backlog > 0
			w.advanced = w.advanced || e.Detail.Difficulty == "ADVANCED"
			w.docs = w.docs || e.Detail.Docs
			wk := monday(e.EarnedAt)
			if !weeks[wk] {
				weeks[wk] = true
				if !lastWeek.IsZero() && wk.Equal(lastWeek.AddDate(0, 0, 7)) {
					w.streak++
				} else {
					w.streak = 1
				}
				lastWeek = wk
				w.bestStreak = max(w.bestStreak, w.streak)
			}
		}
		for _, b := range badgeDefs {
			if !reached[b.key] && b.reach(w) {
				reached[b.key] = true
				s.Badges = append(s.Badges, Badge{Key: b.key, EarnedAt: e.EarnedAt})
			}
		}
	}

	s.Level = LevelOf(s.Total, step)
	for lang, x := range byLanguage {
		if x > 0 {
			s.Languages = append(s.Languages, LanguageLevel{Language: lang, XP: x, Level: LevelOf(x, step)})
		}
	}
	sort.Slice(s.Languages, func(i, j int) bool {
		if s.Languages[i].XP != s.Languages[j].XP {
			return s.Languages[i].XP > s.Languages[j].XP
		}
		return s.Languages[i].Language < s.Languages[j].Language
	})

	this := monday(now)
	s.Streak.Best = w.bestStreak
	s.Streak.ThisWeek = weeks[this]
	if !lastWeek.IsZero() && (lastWeek.Equal(this) || lastWeek.Equal(this.AddDate(0, 0, -7))) {
		s.Streak.Current = w.streak
	}
	return s
}

// monday is the start of t's UTC ISO week.
func monday(t time.Time) time.Time {
	t = t.UTC()
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	offset := (int(d.Weekday()) + 6) % 7 // Monday = 0
	return d.AddDate(0, 0, -offset)
}

// Week names t's UTC ISO week, e.g. "2026-W43".
func Week(t time.Time) string {
	y, w := t.UTC().ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}
