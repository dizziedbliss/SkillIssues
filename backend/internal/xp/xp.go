// Package xp holds the rules for what a pull request earns, and what levels,
// streaks and badges follow from the ledger. Everything here is a pure
// function over its inputs: internal/ingest feeds it what is stored and writes
// what it returns, and go-api reads the ledger back through Summarise.
//
// Every number is an xp.* setting (Rules), so the formula can be tuned live;
// a change applies to what is paid from then on, and levels are recomputed
// from the ledger on every read.
package xp

import (
	"math"
	"sort"
	"time"

	"github.com/dizziedbliss/skillissues/internal/settings"
)

// Ledger sources (xp_events.source).
const (
	SourceMerged         = "merged"
	SourceBackfill       = "backfill"
	SourceOpened         = "opened"
	SourceOpenedReversed = "opened_reversed"
	SourceQuest          = "quest"
	SourcePath           = "path"
)

// Rules is one reading of the xp.* settings.
type Rules struct {
	BaseBeginner, BaseIntermediate, BaseAdvanced, BaseUnclassified, BaseUnlinked int

	FirstMergeRepoBonus int
	BacklogBonus        int
	BacklogDays         int
	CIBonus             int
	ApprovalBonus       int
	ApprovalBonusNoCI   int
	CleanMergeBonus     int
	CleanMergeBonusNoCI int
	BackfillPercent     int
	RepoDailyCap        int
	Opened              int
	OpenedWeeklyCap     int
	LevelStep           int
	QuestReward         int
	PathReward          int
}

// RulesFrom reads Rules from a settings snapshot.
func RulesFrom(s *settings.Snapshot) Rules {
	return Rules{
		BaseBeginner:        s.Int(settings.XPBaseBeginner),
		BaseIntermediate:    s.Int(settings.XPBaseIntermediate),
		BaseAdvanced:        s.Int(settings.XPBaseAdvanced),
		BaseUnclassified:    s.Int(settings.XPBaseUnclassified),
		BaseUnlinked:        s.Int(settings.XPBaseUnlinked),
		FirstMergeRepoBonus: s.Int(settings.XPFirstMergeRepoBonus),
		BacklogBonus:        s.Int(settings.XPBacklogBonus),
		BacklogDays:         s.Int(settings.XPBacklogDays),
		CIBonus:             s.Int(settings.XPCIBonus),
		ApprovalBonus:       s.Int(settings.XPApprovalBonus),
		ApprovalBonusNoCI:   s.Int(settings.XPApprovalBonusNoCI),
		CleanMergeBonus:     s.Int(settings.XPCleanMergeBonus),
		CleanMergeBonusNoCI: s.Int(settings.XPCleanMergeBonusNoCI),
		BackfillPercent:     s.Int(settings.XPBackfillPercent),
		RepoDailyCap:        s.Int(settings.XPRepoDailyCap),
		Opened:              s.Int(settings.XPOpened),
		OpenedWeeklyCap:     s.Int(settings.XPOpenedWeeklyCap),
		LevelStep:           s.Int(settings.XPLevelStep),
		QuestReward:         s.Int(settings.XPQuestReward),
		PathReward:          s.Int(settings.XPPathReward),
	}
}

// Issue is a catalog issue a pull request says it closes.
type Issue struct {
	ID         int
	Difficulty string // BEGINNER, INTERMEDIATE, ADVANCED or UNCLASSIFIED
	CreatedAt  *time.Time
	Labels     []string
	// ClaimedByAuthor: the pull request's author had claimed this issue.
	ClaimedByAuthor bool
}

// PR is a stored pull request, as the rules need it.
type PR struct {
	ID               int64
	RepositoryID     int
	Language         string // the repository's main language; "" when unknown
	State            string // OPEN, CLOSED or MERGED
	CreatedAt        time.Time
	MergedAt         *time.Time
	Checks           *string // nil: the repository runs no checks on it
	Approvals        int
	ChangesRequested int
	NoXPReason       *string
	Issues           []Issue
}

// Detail is how an amount was reached, stored with it (xp_events.detail).
type Detail struct {
	Base         int    `json:"base,omitempty"`
	Difficulty   string `json:"difficulty,omitempty"`
	IssueID      int    `json:"issue_id,omitempty"`
	FirstInRepo  int    `json:"first_in_repo,omitempty"`
	Backlog      int    `json:"backlog,omitempty"`
	CI           int    `json:"ci,omitempty"`
	Approval     int    `json:"approval,omitempty"`
	CleanMerge   int    `json:"clean_merge,omitempty"`
	Percent      int    `json:"percent,omitempty"` // backfill: the share paid
	Capped       int    `json:"capped,omitempty"`  // taken off by xp.repo_daily_cap
	RepositoryID int    `json:"repository_id,omitempty"`
	Language     string `json:"language,omitempty"`
	Docs         bool   `json:"docs,omitempty"`    // the closed issue is labelled documentation
	Claimed      bool   `json:"claimed,omitempty"` // it closes an issue its author had claimed
}

// Award is one ledger row to write.
type Award struct {
	Source   string
	PRID     int64
	Key      string // quests and paths; "" for pull request awards
	Amount   int
	Detail   Detail
	EarnedAt time.Time
}

// Paid is a pull request award already in the ledger.
type Paid struct {
	Source   string
	PRID     int64
	Amount   int
	Detail   Detail
	EarnedAt time.Time
}

// Plan returns the awards a person's pull requests earn that are not in the
// ledger yet. joined is when they signed up: merges before it are backfill.
// It is deterministic over its inputs, so running it again after a failed
// write plans the same awards, and the ledger's unique keys make a repeat
// harmless.
func Plan(r Rules, joined time.Time, prs []PR, paid []Paid) []Award {
	type pk struct {
		source string
		pr     int64
	}
	type dayKey struct {
		repo int
		day  string
	}
	done := map[pk]Paid{}
	mergedRepos := map[int]bool{}
	repoDay := map[dayKey]int{}
	openedWeek := map[string]int{}
	openedIssues := map[int]bool{}
	for _, p := range paid {
		done[pk{p.Source, p.PRID}] = p
		switch p.Source {
		case SourceMerged, SourceBackfill:
			mergedRepos[p.Detail.RepositoryID] = true
			repoDay[dayKey{p.Detail.RepositoryID, p.EarnedAt.UTC().Format(time.DateOnly)}] += p.Amount
		case SourceOpened:
			openedWeek[Week(p.EarnedAt)]++
			if p.Detail.IssueID != 0 {
				openedIssues[p.Detail.IssueID] = true
			}
		}
	}
	isPaid := func(source string, id int64) bool { _, ok := done[pk{source, id}]; return ok }

	var out []Award

	// Merges, oldest first, so "first merge in a repository" and the daily
	// cap are decided over the whole history, however it arrived.
	merged := make([]PR, 0, len(prs))
	for _, p := range prs {
		if p.State == "MERGED" && p.MergedAt != nil && p.NoXPReason == nil &&
			!isPaid(SourceMerged, p.ID) && !isPaid(SourceBackfill, p.ID) {
			merged = append(merged, p)
		}
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if !merged[i].MergedAt.Equal(*merged[j].MergedAt) {
			return merged[i].MergedAt.Before(*merged[j].MergedAt)
		}
		return merged[i].ID < merged[j].ID
	})
	for _, p := range merged {
		at := *p.MergedAt
		d := Detail{RepositoryID: p.RepositoryID, Language: p.Language}
		issue, base := hardest(r, p.Issues)
		d.Base = base
		if issue != nil {
			d.Difficulty, d.IssueID, d.Docs = issue.Difficulty, issue.ID, isDocs(issue.Labels)
			for _, i := range p.Issues {
				d.Claimed = d.Claimed || i.ClaimedByAuthor
			}
			if issue.CreatedAt != nil && r.BacklogDays > 0 && at.Sub(*issue.CreatedAt) >= time.Duration(r.BacklogDays)*24*time.Hour {
				d.Backlog = r.BacklogBonus
			}
		}
		if !mergedRepos[p.RepositoryID] {
			d.FirstInRepo = r.FirstMergeRepoBonus
		}
		mergedRepos[p.RepositoryID] = true
		ci := p.Checks != nil
		if ci && *p.Checks == "SUCCESS" {
			d.CI = r.CIBonus
		}
		if p.Approvals > 0 {
			d.Approval = pick(ci, r.ApprovalBonus, r.ApprovalBonusNoCI)
		}
		if p.ChangesRequested == 0 {
			d.CleanMerge = pick(ci, r.CleanMergeBonus, r.CleanMergeBonusNoCI)
		}
		amount := d.Base + d.FirstInRepo + d.Backlog + d.CI + d.Approval + d.CleanMerge
		source := SourceMerged
		if at.Before(joined) {
			source = SourceBackfill
			d.Percent = r.BackfillPercent
			amount = amount * r.BackfillPercent / 100
		}
		k := dayKey{p.RepositoryID, at.UTC().Format(time.DateOnly)}
		if r.RepoDailyCap > 0 {
			if left := max(r.RepoDailyCap-repoDay[k], 0); amount > left {
				d.Capped = amount - left
				amount = left
			}
		}
		repoDay[k] += amount
		out = append(out, Award{Source: source, PRID: p.ID, Amount: amount, Detail: d, EarnedAt: at})
	}

	// Openings, oldest first: a small nudge for starting on a catalog issue,
	// once per issue, a few a week, and only after signing up.
	opened := make([]PR, 0, len(prs))
	for _, p := range prs {
		if len(p.Issues) > 0 && p.NoXPReason == nil && !p.CreatedAt.Before(joined) && !isPaid(SourceOpened, p.ID) {
			opened = append(opened, p)
		}
	}
	sort.SliceStable(opened, func(i, j int) bool {
		if !opened[i].CreatedAt.Equal(opened[j].CreatedAt) {
			return opened[i].CreatedAt.Before(opened[j].CreatedAt)
		}
		return opened[i].ID < opened[j].ID
	})
	for _, p := range opened {
		if r.Opened == 0 || p.State == "CLOSED" {
			continue // nothing to pay, or already closed unmerged: it would be reversed at once
		}
		var issue *Issue
		for i := range p.Issues {
			if !openedIssues[p.Issues[i].ID] {
				issue = &p.Issues[i]
				break
			}
		}
		w := Week(p.CreatedAt)
		if issue == nil || openedWeek[w] >= r.OpenedWeeklyCap {
			continue
		}
		openedWeek[w]++
		openedIssues[issue.ID] = true
		out = append(out, Award{Source: SourceOpened, PRID: p.ID, Amount: r.Opened, EarnedAt: p.CreatedAt,
			Detail: Detail{IssueID: issue.ID, RepositoryID: p.RepositoryID, Language: p.Language}})
	}

	// An opening paid for a pull request that then closed unmerged is taken back.
	for _, p := range prs {
		if p.State != "CLOSED" {
			continue
		}
		if o, ok := done[pk{SourceOpened, p.ID}]; ok && !isPaid(SourceOpenedReversed, p.ID) {
			out = append(out, Award{Source: SourceOpenedReversed, PRID: p.ID, Amount: -o.Amount, EarnedAt: o.EarnedAt,
				Detail: Detail{IssueID: o.Detail.IssueID, RepositoryID: p.RepositoryID, Language: p.Language}})
		}
	}
	return out
}

// hardest is the linked issue paying the most, and that base; with none
// linked, the unlinked base.
func hardest(r Rules, issues []Issue) (*Issue, int) {
	var best *Issue
	base := r.BaseUnlinked
	for i := range issues {
		b := r.BaseUnclassified
		switch issues[i].Difficulty {
		case "BEGINNER":
			b = r.BaseBeginner
		case "INTERMEDIATE":
			b = r.BaseIntermediate
		case "ADVANCED":
			b = r.BaseAdvanced
		}
		if best == nil || b > base {
			best, base = &issues[i], b
		}
	}
	return best, base
}

func pick(ci bool, withCI, withoutCI int) int {
	if ci {
		return withCI
	}
	return withoutCI
}

// isDocs reports whether an issue's labels say it is about documentation.
func isDocs(labels []string) bool {
	for _, l := range labels {
		switch normal(l) {
		case "docs", "documentation", "doc", "type: docs", "type: documentation", "area: docs", "kind/documentation":
			return true
		}
	}
	return false
}

func normal(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// Level is where an XP total stands: level L starts at step × L(L−1)/2.
type Level struct {
	Level int `json:"level"`
	// Start and Next are the totals at which this level began and the next
	// one begins.
	Start int `json:"start"`
	Next  int `json:"next"`
}

// LevelOf places an XP total on the curve.
func LevelOf(total, step int) Level {
	if step <= 0 {
		step = 1
	}
	if total < 0 {
		total = 0
	}
	l := int((1 + math.Sqrt(1+8*float64(total)/float64(step))) / 2)
	for threshold(l+1, step) <= total {
		l++
	}
	for l > 1 && threshold(l, step) > total {
		l--
	}
	return Level{Level: l, Start: threshold(l, step), Next: threshold(l+1, step)}
}

func threshold(l, step int) int { return step * l * (l - 1) / 2 }
