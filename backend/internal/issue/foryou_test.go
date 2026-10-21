package issue

import (
	"context"
	"encoding/base64"
	"math"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestForYouCursorRoundTrip(t *testing.T) {
	for _, c := range []Cursor{
		{Sort: SortForYou, Seed: 123456789, Start: 42, Key: 77, ID: 9, Offset: 3},
		{Sort: SortForYou, Seed: 1, Start: 2147483646, Key: -1, ID: 0, Offset: 0, Wrapped: true},
	} {
		got, err := DecodeCursor(c.Encode())
		if err != nil || got != c {
			t.Errorf("round trip of %+v = %+v, %v", c, got, err)
		}
	}
	for _, bad := range []string{"y.1.2.3.4.5", "y.-1.2.3.4.5.0", "y.1.2.-2.4.5.0", "y.1.2.3.4.5.2", "y.a.2.3.4.5.0"} {
		if _, err := DecodeCursor(base64.RawURLEncoding.EncodeToString([]byte(bad))); err == nil {
			t.Errorf("%q decoded", bad)
		}
	}
}

func TestForYouScoreParts(t *testing.T) {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	r := &ForYouRules{WeightFit: 1, WeightDoable: 1, WeightNeed: 1, ExposureHalf: 20, Now: now}
	two := 2
	base := func() Issue {
		return Issue{ID: 1, Language: "Go", Tech: []string{"Go", "Shell"}, Difficulty: "BEGINNER", DifficultyScore: &two,
			Comments: 2, RepositoryID: 7, updated: now.AddDate(0, 0, -3), created: now.AddDate(0, 0, -10)}
	}
	me := &ForYouViewer{ID: 5, Languages: []string{"go"}, Level: "BEGINNER", Target: "INTERMEDIATE"}
	score := func(it Issue, v *ForYouViewer, shown int) float64 { s, _ := forYouScore(&it, v, r, shown); return s }

	inLang := score(base(), me, 0)
	techOnly := base()
	techOnly.Language = "Rust"
	techOnly.Tech = []string{"Rust", "Go"}
	other := base()
	other.Language, other.Tech = "Rust", []string{"Rust"}
	if !(inLang > score(techOnly, me, 0) && score(techOnly, me, 0) > score(other, me, 0)) {
		t.Errorf("language %v, tech %v, neither %v: want falling", inLang, score(techOnly, me, 0), score(other, me, 0))
	}

	hard := base()
	hard.Difficulty = "ADVANCED"
	if score(hard, me, 0) >= inLang {
		t.Errorf("an issue two levels up scored %v, not below %v", score(hard, me, 0), inLang)
	}

	returning := &ForYouViewer{ID: 5, Languages: []string{"go"}, Level: "BEGINNER", MergedRepos: map[int]bool{7: true}}
	if score(base(), returning, 0) <= score(base(), &ForYouViewer{ID: 5, Languages: []string{"go"}, Level: "BEGINNER"}, 0) {
		t.Error("a repository merged in before doesn't score higher")
	}

	busy := base()
	busy.Comments = 80
	if score(busy, me, 0) >= inLang {
		t.Error("a long discussion doesn't lower Doable")
	}
	welcoming := base()
	welcoming.Health = &Health{Welcoming: true}
	if score(welcoming, me, 0) <= inLang {
		t.Error("a welcoming repository doesn't raise Doable")
	}

	waiting := base()
	waiting.created = now.AddDate(0, -8, 0)
	if score(waiting, me, 0) <= inLang {
		t.Error("an issue waiting months doesn't raise Need")
	}
	if got, want := score(base(), me, 20), inLang/2; math.Abs(got-want) > 1e-9 {
		t.Errorf("shown exposure_half times: %v, want half of %v", got, inLang)
	}

	// Signed out, Fit doesn't apply: language makes no difference.
	if score(base(), &ForYouViewer{}, 0) != score(other, &ForYouViewer{}, 0) {
		t.Error("signed out, the language changed the score")
	}

	_, reasons := forYouScore(func() *Issue { it := waiting; it.Health = &Health{Welcoming: true}; return &it }(), me, r, 0)
	if len(reasons) != 2 || reasons[0] != "language" {
		t.Errorf("reasons = %v, want two, language first", reasons)
	}
}

func TestDrawForYou(t *testing.T) {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	var items []Issue
	for i := 1; i <= 40; i++ {
		items = append(items, Issue{ID: i, Language: "Go", Comments: i * 2, updated: now, created: now})
	}
	v := &ForYouViewer{}
	ids := func(list []Issue) []int {
		out := make([]int, len(list))
		for i, it := range list {
			out[i] = it.ID
		}
		return out
	}
	plain := &ForYouRules{WeightFit: 1, WeightDoable: 1, WeightNeed: 1, Now: now}
	ranked := ids(drawForYou(items, v, plain, 1, nil))
	for i := 1; i < len(ranked); i++ {
		a, _ := forYouScore(&items[ranked[i-1]-1], v, plain, 0)
		b, _ := forYouScore(&items[ranked[i]-1], v, plain, 0)
		if a < b {
			t.Fatalf("randomness 0 isn't a ranking: %v", ranked)
		}
	}
	random := &ForYouRules{WeightFit: 1, WeightDoable: 1, WeightNeed: 1, Now: now, Randomness: 1}
	a, b := ids(drawForYou(items, v, random, 1, nil)), ids(drawForYou(items, v, random, 1, nil))
	if !slices.Equal(a, b) {
		t.Error("the same seed drew two orders")
	}
	if c := ids(drawForYou(items, v, random, 2, nil)); slices.Equal(a, c) {
		t.Error("two seeds drew the same order")
	}
	if !slices.Equal(slices.Sorted(slices.Values(a)), ids(items)) {
		t.Error("the draw lost or repeated an issue")
	}
}

// memStore is a ForYouStore in memory.
type memStore struct {
	mu     sync.Mutex
	chunks map[string][]byte
	shown  map[int]int
}

func (m *memStore) GetChunk(_ context.Context, key string) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.chunks[key]
}

func (m *memStore) SetChunk(_ context.Context, key string, data []byte, _ time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chunks[key] = data
}

func (m *memStore) ShownCounts(_ context.Context, ids []int) map[int]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[int]int{}
	for _, id := range ids {
		out[id] = m.shown[id]
	}
	return out
}

func (m *memStore) RecordShown(_ context.Context, ids []int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range ids {
		m.shown[id]++
	}
}

// Paged to the end, For you shows each matching repository's issue exactly
// once, from the seeded start round past the end of the order and back to it,
// whatever the chunk and page sizes, with or without a store.
func TestForYouWalksEverythingOnce(t *testing.T) {
	f := newFilterFixture(t)
	now := time.Now()
	viewer := ForYouViewer{ID: 31337, Languages: []string{"Go"}, Level: "BEGINNER"}
	start := int(forYouSeed(viewer.ID, now) % math.MaxInt32)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	want := map[int]bool{}
	// Repositories either side of the start, so the walk has to wrap.
	for i, key := range []int{start - 30, start - 20, start - 10, start, start + 10, start + 20, start + 30} {
		if key < 0 {
			key = i
		}
		repo := f.repo(t, key, "Go")
		want[f.issue(t, repo, 1, "BEGINNER", 2, 0, t0)] = true
		f.issue(t, repo, 2, "ADVANCED", 8, 0, t0) // not the repository's slot
	}

	for _, store := range []ForYouStore{nil, &memStore{chunks: map[string][]byte{}, shown: map[int]int{}}} {
		f.svc.SetForYouStore(store)
		for _, pool := range []int{1, 2, 3, 50} {
			for _, limit := range []int{1, 2, 5, 20} {
				rules := ForYouRules{Pool: pool, Randomness: 0.6, WeightFit: 1, WeightDoable: 1, WeightNeed: 1, ExposureHalf: 20, Now: now}
				seen := map[int]bool{}
				var after *Cursor
				for pages := 0; ; pages++ {
					if pages > 100 {
						t.Fatalf("pool %d, limit %d: no end after 100 pages", pool, limit)
					}
					page, next, err := f.svc.ForYouPage(context.Background(), Filters{Topics: []string{f.topic}, Limit: limit}, after, viewer, rules)
					if err != nil {
						t.Fatal(err)
					}
					for _, it := range page {
						if seen[it.ID] {
							t.Fatalf("pool %d, limit %d: #%d shown twice", pool, limit, it.ID)
						}
						seen[it.ID] = true
					}
					if next == nil {
						break
					}
					// Through the cursor's token, as a client would send it.
					c, err := DecodeCursor(next.Encode())
					if err != nil {
						t.Fatal(err)
					}
					after = &c
				}
				for id := range want {
					if !seen[id] {
						t.Errorf("pool %d, limit %d: #%d never shown (saw %v)", pool, limit, id, seen)
					}
				}
				if len(seen) != len(want) {
					t.Errorf("pool %d, limit %d: saw %d issues, want %d", pool, limit, len(seen), len(want))
				}
			}
		}
	}
	f.svc.SetForYouStore(nil)
}
