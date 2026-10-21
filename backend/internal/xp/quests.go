package xp

import (
	"sort"
	"time"
)

// Quests are one-time goals, each paid xp.quest_reward once (source quest,
// key "quest:<key>"). They count only merges since joining: they are about
// what you do here, so backfilled history never completes one.

// QuestProgress is how far a person is with one quest.
type QuestProgress struct {
	Key    string     `json:"key"`
	Have   int        `json:"have"`
	Need   int        `json:"need"`
	DoneAt *time.Time `json:"done_at"`
}

type questWalk struct {
	merges    int
	languages map[string]bool
	repos     map[int]bool
	docs      int
	backlog   int
	claimed   int
	months    map[string]int
	bestMonth int
}

type questDef struct {
	key  string
	need int
	have func(w *questWalk) int
}

var questDefs = []questDef{
	{"first_merge_here", 1, func(w *questWalk) int { return w.merges }},
	{"claimed_and_done", 1, func(w *questWalk) int { return w.claimed }},
	{"docs_fix", 1, func(w *questWalk) int { return w.docs }},
	{"two_languages", 2, func(w *questWalk) int { return len(w.languages) }},
	{"three_repos", 3, func(w *questWalk) int { return len(w.repos) }},
	{"three_in_a_month", 3, func(w *questWalk) int { return w.bestMonth }},
	{"backlog_dive", 1, func(w *questWalk) int { return w.backlog }},
}

// Quests is every quest's progress over a person's ledger, in display order.
func Quests(events []Event) []QuestProgress {
	sorted := append([]Event(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].EarnedAt.Before(sorted[j].EarnedAt) })
	w := &questWalk{languages: map[string]bool{}, repos: map[int]bool{}, months: map[string]int{}}
	out := make([]QuestProgress, len(questDefs))
	for i, q := range questDefs {
		out[i] = QuestProgress{Key: q.key, Need: q.need}
	}
	for _, e := range sorted {
		if e.Source != SourceMerged {
			continue
		}
		w.merges++
		if e.Detail.Language != "" {
			w.languages[e.Detail.Language] = true
		}
		w.repos[e.Detail.RepositoryID] = true
		if e.Detail.Docs {
			w.docs++
		}
		if e.Detail.Backlog > 0 {
			w.backlog++
		}
		if e.Detail.Claimed {
			w.claimed++
		}
		m := e.EarnedAt.UTC().Format("2006-01")
		w.months[m]++
		w.bestMonth = max(w.bestMonth, w.months[m])
		for i, q := range questDefs {
			if out[i].DoneAt == nil && q.have(w) >= q.need {
				at := e.EarnedAt
				out[i].DoneAt = &at
			}
		}
	}
	for i, q := range questDefs {
		out[i].Have = min(q.have(w), q.need)
	}
	return out
}

// QuestKey is the ledger key a quest is paid under.
func QuestKey(quest string) string { return "quest:" + quest }

// QuestAwards are the quests done and not yet paid (paid holds the ledger
// keys already written).
func QuestAwards(r Rules, events []Event, paid map[string]bool) []Award {
	var out []Award
	for _, q := range Quests(events) {
		if q.DoneAt != nil && !paid[QuestKey(q.Key)] && r.QuestReward > 0 {
			out = append(out, Award{Source: SourceQuest, Key: QuestKey(q.Key), Amount: r.QuestReward, EarnedAt: *q.DoneAt})
		}
	}
	return out
}
