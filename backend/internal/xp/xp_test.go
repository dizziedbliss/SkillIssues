package xp

import (
	"testing"
	"time"
)

var rules = Rules{
	BaseBeginner: 75, BaseIntermediate: 140, BaseAdvanced: 250, BaseUnclassified: 100, BaseUnlinked: 50,
	FirstMergeRepoBonus: 15, BacklogBonus: 50, BacklogDays: 180,
	CIBonus: 15, ApprovalBonus: 10, ApprovalBonusNoCI: 20, CleanMergeBonus: 5, CleanMergeBonusNoCI: 10,
	BackfillPercent: 50, RepoDailyCap: 400, Opened: 5, OpenedWeeklyCap: 3, LevelStep: 100,
}

var joined = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

func at(day int) *time.Time {
	t := joined.AddDate(0, 0, day)
	return &t
}

// merged builds a merged pull request with changes requested (no clean
// bonus) and no checks or approvals, so only what a test sets adds up.
func merged(id int64, repo int, day int, issues ...Issue) PR {
	return PR{ID: id, RepositoryID: repo, Language: "Go", State: "MERGED", CreatedAt: *at(day - 1), MergedAt: at(day),
		ChangesRequested: 1, Issues: issues}
}

func amounts(awards []Award) map[int64]int {
	m := map[int64]int{}
	for _, a := range awards {
		if a.Source == SourceMerged || a.Source == SourceBackfill {
			m[a.PRID] = a.Amount
		}
	}
	return m
}

func TestBaseByDifficulty(t *testing.T) {
	prs := []PR{
		merged(1, 1, 1, Issue{ID: 10, Difficulty: "BEGINNER"}),
		merged(2, 1, 2, Issue{ID: 11, Difficulty: "INTERMEDIATE"}),
		merged(3, 1, 3, Issue{ID: 12, Difficulty: "ADVANCED"}),
		merged(4, 1, 4, Issue{ID: 13, Difficulty: "UNCLASSIFIED"}),
		merged(5, 1, 5),
		// Two linked issues: the harder one pays.
		merged(6, 1, 6, Issue{ID: 14, Difficulty: "BEGINNER"}, Issue{ID: 15, Difficulty: "ADVANCED"}),
	}
	got := amounts(Plan(rules, joined, prs, nil))
	want := map[int64]int{1: 75 + 15, 2: 140, 3: 250, 4: 100, 5: 50, 6: 250}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("pr %d: %d XP, want %d", id, got[id], w)
		}
	}
}

// The first merge in a repository earns the bonus however the pull requests
// arrived: a backfill reads newest first.
func TestFirstMergeInRepositoryIsTheOldest(t *testing.T) {
	prs := []PR{merged(2, 7, 9), merged(1, 7, 3), merged(3, 8, 5)}
	got := amounts(Plan(rules, joined, prs, nil))
	if got[1] != 65 || got[2] != 50 || got[3] != 65 {
		t.Errorf("got %v, want the oldest per repository to carry the bonus", got)
	}
	// Already paid for an earlier merge in that repository: no bonus now.
	paid := []Paid{{Source: SourceMerged, PRID: 99, Amount: 50, Detail: Detail{RepositoryID: 7}, EarnedAt: *at(1)}}
	if got := amounts(Plan(rules, joined, []PR{merged(4, 7, 20)}, paid)); got[4] != 50 {
		t.Errorf("second repository merge: %d, want 50", got[4])
	}
}

func TestBacklogBonusByIssueAge(t *testing.T) {
	old := joined.AddDate(0, 0, -200)
	recent := joined.AddDate(0, 0, -10)
	prs := []PR{
		merged(1, 1, 1, Issue{ID: 1, Difficulty: "BEGINNER", CreatedAt: &old}),
		merged(2, 2, 1, Issue{ID: 2, Difficulty: "BEGINNER", CreatedAt: &recent}),
	}
	a := Plan(rules, joined, prs, nil)
	got := amounts(a)
	if got[1] != 75+15+50 || got[2] != 75+15 {
		t.Errorf("got %v", got)
	}
	for _, x := range a {
		if x.Source == SourceMerged && x.PRID == 1 && (x.Detail.Backlog != 50 || x.Detail.IssueID != 1) {
			t.Errorf("detail = %+v", x.Detail)
		}
	}
}

func TestQualityBonusesWithAndWithoutChecks(t *testing.T) {
	ci := merged(1, 1, 1)
	ci.Checks, ci.Approvals, ci.ChangesRequested = new("SUCCESS"), 1, 0
	failing := merged(2, 2, 1)
	failing.Checks, failing.ChangesRequested = new("FAILURE"), 0
	noCI := merged(3, 3, 1)
	noCI.Approvals, noCI.ChangesRequested = 2, 0
	got := amounts(Plan(rules, joined, []PR{ci, failing, noCI}, nil))
	// 50 base + 15 first merge, then: CI 15 + approval 10 + clean 5;
	// failing checks: clean 5 only; no checks: approval 20 + clean 10.
	if got[1] != 65+30 || got[2] != 65+5 || got[3] != 65+30 {
		t.Errorf("got %v", got)
	}
}

func TestBackfillPaysHalf(t *testing.T) {
	p := merged(1, 1, -30, Issue{ID: 1, Difficulty: "INTERMEDIATE"})
	a := Plan(rules, joined, []PR{p}, nil)
	if len(a) != 1 || a[0].Source != SourceBackfill || a[0].Amount != (140+15)/2 || a[0].Detail.Percent != 50 {
		t.Fatalf("got %+v", a)
	}
}

func TestDailyCapPerRepository(t *testing.T) {
	r := rules
	r.RepoDailyCap = 300
	advanced := Issue{Difficulty: "ADVANCED"}
	a, b := advanced, advanced
	a.ID, b.ID = 1, 2
	prs := []PR{merged(1, 1, 1, a), merged(2, 1, 1, b), merged(3, 1, 2)}
	got := amounts(Plan(r, joined, prs, nil))
	// 265 the first, the cap's 35 left the second, and a new day is uncapped.
	if got[1] != 265 || got[2] != 35 || got[3] != 50 {
		t.Errorf("got %v", got)
	}
}

func TestNothingForOwnRepositoriesOrWhatIsPaid(t *testing.T) {
	own := merged(1, 1, 1)
	own.NoXPReason = new("own_repository")
	paid := []Paid{{Source: SourceMerged, PRID: 2, Amount: 65, Detail: Detail{RepositoryID: 2}, EarnedAt: *at(1)}}
	if a := Plan(rules, joined, []PR{own, merged(2, 2, 1)}, paid); len(a) != 0 {
		t.Errorf("got %+v, want nothing", a)
	}
}

func TestOpenedOncePerIssueCappedWeeklyAndReversed(t *testing.T) {
	open := func(id int64, day int, issue int) PR {
		return PR{ID: id, RepositoryID: 1, State: "OPEN", CreatedAt: *at(day), Issues: []Issue{{ID: issue}}}
	}
	// 2026-06-01 is a Monday: days 0-6 are one week.
	prs := []PR{open(1, 0, 1), open(2, 1, 1), open(3, 2, 2), open(4, 3, 3), open(5, 4, 4), open(6, 8, 5)}
	before := open(7, -3, 6) // opened before signing up
	prs = append(prs, before)
	var got []int64
	for _, a := range Plan(rules, joined, prs, nil) {
		if a.Source == SourceOpened {
			got = append(got, a.PRID)
		}
	}
	// #2 repeats issue 1; #5 is the week's fourth; #7 predates signup.
	if want := []int64{1, 3, 4, 6}; len(got) != len(want) || got[0] != 1 || got[1] != 3 || got[2] != 4 || got[3] != 6 {
		t.Errorf("opened awards for %v, want %v", got, want)
	}

	closed := open(1, 0, 1)
	closed.State = "CLOSED"
	paid := []Paid{{Source: SourceOpened, PRID: 1, Amount: 5, Detail: Detail{IssueID: 1}, EarnedAt: *at(0)}}
	a := Plan(rules, joined, []PR{closed}, paid)
	if len(a) != 1 || a[0].Source != SourceOpenedReversed || a[0].Amount != -5 {
		t.Errorf("reversal = %+v", a)
	}
	paid = append(paid, Paid{Source: SourceOpenedReversed, PRID: 1, Amount: -5})
	if a := Plan(rules, joined, []PR{closed}, paid); len(a) != 0 {
		t.Errorf("reversed twice: %+v", a)
	}
}

func TestLevels(t *testing.T) {
	for _, c := range []struct{ total, level, start, next int }{
		{0, 1, 0, 100}, {99, 1, 0, 100}, {100, 2, 100, 300}, {299, 2, 100, 300},
		{300, 3, 300, 600}, {1000, 5, 1000, 1500}, {4500, 10, 4500, 5500},
	} {
		if got := LevelOf(c.total, 100); got != (Level{c.level, c.start, c.next}) {
			t.Errorf("LevelOf(%d) = %+v, want level %d [%d, %d)", c.total, got, c.level, c.start, c.next)
		}
	}
}

func TestSummariseStreaksBadgesAndLanguages(t *testing.T) {
	week := func(n int) time.Time { return joined.AddDate(0, 0, 7*n+2) } // a Wednesday
	ev := func(n int, lang string, repo int, d Detail) Event {
		d.Language, d.RepositoryID = lang, repo
		return Event{Source: SourceMerged, Amount: 100, Detail: d, EarnedAt: week(n)}
	}
	events := []Event{
		ev(0, "Go", 1, Detail{}), ev(1, "Go", 2, Detail{Docs: true}), ev(2, "Rust", 3, Detail{}),
		ev(3, "TypeScript", 3, Detail{Backlog: 50}),    // four weeks in a row
		ev(5, "Go", 1, Detail{Difficulty: "ADVANCED"}), // a gap, then one more
		{Source: SourceOpened, Amount: 5, EarnedAt: week(5)},
	}
	s := Summarise(events, 100, week(6))
	if s.Total != 505 || s.Merges != 5 || s.Level.Level != 3 {
		t.Errorf("total %d merges %d level %d", s.Total, s.Merges, s.Level.Level)
	}
	if s.Streak.Best != 4 || s.Streak.Current != 1 || s.Streak.ThisWeek {
		t.Errorf("streak = %+v, want best 4, current 1 (last week), not this week", s.Streak)
	}
	if s2 := Summarise(events, 100, week(8)); s2.Streak.Current != 0 {
		t.Errorf("a missed week still counts: %+v", s2.Streak)
	}
	if len(s.Languages) != 3 || s.Languages[0].Language != "Go" || s.Languages[0].XP != 300 {
		t.Errorf("languages = %+v", s.Languages)
	}
	earned := map[string]time.Time{}
	for _, b := range s.Badges {
		earned[b.Key] = b.EarnedAt
	}
	for key, when := range map[string]time.Time{
		"first_merge": week(0), "merges_5": week(5), "repos_3": week(2), "languages_3": week(3),
		"docs_fix": week(1), "backlog_hero": week(3), "advanced_fix": week(5), "streak_4": week(3),
	} {
		if !earned[key].Equal(when) {
			t.Errorf("badge %s earned %v, want %v", key, earned[key], when)
		}
	}
	if _, ok := earned["merges_10"]; ok {
		t.Error("merges_10 with five merges")
	}
}

func TestQuests(t *testing.T) {
	at := func(day int) time.Time { return joined.AddDate(0, 0, day) }
	merge := func(day int, lang string, repo int, d Detail) Event {
		d.Language, d.RepositoryID = lang, repo
		return Event{Source: SourceMerged, Amount: 50, Detail: d, EarnedAt: at(day)}
	}
	events := []Event{
		{Source: SourceBackfill, Amount: 25, Detail: Detail{Language: "C", RepositoryID: 9, Docs: true}, EarnedAt: at(-90)}, // never counts
		merge(3, "Go", 1, Detail{}),
		merge(10, "Go", 2, Detail{Claimed: true}),
		merge(12, "Rust", 3, Detail{Backlog: 50}),
	}
	got := map[string]QuestProgress{}
	for _, q := range Quests(events) {
		got[q.Key] = q
	}
	done := func(key string, day int) {
		t.Helper()
		if q := got[key]; q.DoneAt == nil || !q.DoneAt.Equal(at(day)) {
			t.Errorf("%s = %+v, want done on day %d", key, q, day)
		}
	}
	done("first_merge_here", 3)
	done("claimed_and_done", 10)
	done("two_languages", 12)
	done("three_repos", 12)
	done("three_in_a_month", 12)
	done("backlog_dive", 12)
	if q := got["docs_fix"]; q.DoneAt != nil || q.Have != 0 {
		t.Errorf("a backfilled docs fix completed docs_fix: %+v", q)
	}

	r := rules
	r.QuestReward = 50
	paid := map[string]bool{QuestKey("first_merge_here"): true}
	awards := QuestAwards(r, events, paid)
	if len(awards) != 5 {
		t.Fatalf("awards = %+v, want the 5 done and unpaid", awards)
	}
	for _, a := range awards {
		if a.Source != SourceQuest || a.Amount != 50 || a.Key == QuestKey("first_merge_here") {
			t.Errorf("award %+v", a)
		}
	}
}
