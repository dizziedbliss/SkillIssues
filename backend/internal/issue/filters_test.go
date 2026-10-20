package issue

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

func TestCursorFormsPerSort(t *testing.T) {
	for _, c := range []Cursor{
		{Sort: SortNewest, Updated: 1760000000123456, ID: 9},
		{Sort: SortShuffle, Key: 123456789, ID: 77},
	} {
		got, err := DecodeCursor(c.Encode())
		if err != nil {
			t.Fatalf("%s: %v", c.Sort, err)
		}
		if got.Sort != c.Sort || got.Updated != c.Updated || got.Key != c.Key || got.ID != c.ID {
			t.Errorf("%s round trip: got %+v, want %+v", c.Sort, got, c)
		}
	}
	// An easiest cursor keeps its old form, so tokens clients hold still work.
	five := 5
	old := Cursor{Bucket: 1, Score: &five, Updated: 1760000000000001, ID: 3}
	if got, err := DecodeCursor(old.Encode()); err != nil || got.Sort != SortEasiest {
		t.Errorf("easiest cursor decoded as %+v, %v", got, err)
	}
}

func TestParseFilters(t *testing.T) {
	if got, err := ParseList("labels", " bug, docs ,bug,,", 5); err != nil || !slices.Equal(got, []string{"bug", "docs"}) {
		t.Errorf("ParseList = %v, %v", got, err)
	}
	if _, err := ParseList("labels", "a,b,c", 2); !errors.Is(err, ErrBadFilter) {
		t.Errorf("too many values: %v, want ErrBadFilter", err)
	}
	if got, err := ParseDifficulties("beginner,Unclassified", 5); err != nil || !slices.Equal(got, []string{"BEGINNER", "UNCLASSIFIED"}) {
		t.Errorf("ParseDifficulties = %v, %v", got, err)
	}
	if _, err := ParseDifficulties("trivial", 5); !errors.Is(err, ErrBadFilter) {
		t.Errorf("unknown difficulty: %v, want ErrBadFilter", err)
	}
	for _, s := range []string{"", "Shuffle", "easiest", "NEWEST"} {
		if _, err := ParseSort(s); err != nil {
			t.Errorf("ParseSort(%q): %v", s, err)
		}
	}
	if _, err := ParseSort("stars"); err == nil {
		t.Error("ParseSort(stars): want an error")
	}
}

// filterFixture is a handful of repositories sharing a topic no other
// repository has, so a filter on it sees only them.
type filterFixture struct {
	pool  *db.Pool
	svc   *Service
	topic string
	repos []int // in insertion order
}

func newFilterFixture(t *testing.T) *filterFixture {
	t.Helper()
	pool := testPool(t)
	f := &filterFixture{pool: pool, svc: New(pool, nil), topic: fmt.Sprintf("filtertest%d", time.Now().UnixNano())}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE $1 = ANY(topics)`, f.topic); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return f
}

// repo inserts a repository with the fixture topic, a shuffle position and
// GitHub's languages list.
func (f *filterFixture) repo(t *testing.T, shuffleKey int, language string, languages ...string) int {
	t.Helper()
	langs := "["
	for i, l := range languages {
		if i > 0 {
			langs += ","
		}
		langs += fmt.Sprintf(`{"name": %q, "size": 1}`, l)
	}
	langs += "]"
	var id int
	if err := f.pool.QueryRow(context.Background(),
		`INSERT INTO repositories (full_name, url, language, topics, languages, shuffle_key)
		 VALUES ($1, 'u', $2, ARRAY[$3], $4::jsonb, $5) RETURNING id`,
		[]any{fmt.Sprintf("%s/r%d", f.topic, len(f.repos)), language, f.topic, langs, shuffleKey},
		func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
		t.Fatal(err)
	}
	f.repos = append(f.repos, id)
	return id
}

func (f *filterFixture) issue(t *testing.T, repo, number int, difficulty string, score any, comments int, created time.Time, labels ...string) int {
	t.Helper()
	if labels == nil {
		labels = []string{}
	}
	var id int
	if err := f.pool.QueryRow(context.Background(),
		`INSERT INTO issues (repository_id, number, title, url, difficulty, difficulty_score, comments, created_at, updated_at, labels)
		 VALUES ($1, $2, 't', 'u', $3, $4, $5, $6, $6, $7) RETURNING id`,
		[]any{repo, number, difficulty, score, comments, created, labels},
		func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f *filterFixture) walk(t *testing.T, fl Filters) []int {
	t.Helper()
	fl.Topics = append([]string{f.topic}, fl.Topics...)
	var got []int
	var after *Cursor
	for pages := 0; ; pages++ {
		if pages > 100 {
			t.Fatal("cursor never ran out")
		}
		page, next, err := f.svc.BrowseIssues(context.Background(), fl, after, nil)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, idsOf(page)...)
		if next == nil {
			return got
		}
		decoded, err := DecodeCursor(next.Encode())
		if err != nil {
			t.Fatal(err)
		}
		after = &decoded
	}
}

// The shuffle shows each repository once, its best open issue, in shuffle
// order — the same through the repository walk and the rare-filter path.
func TestShuffleFeedOneIssuePerRepository(t *testing.T) {
	f := newFilterFixture(t)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	big := f.repo(t, 30, "Go")
	small := f.repo(t, 10, "Go")
	empty := f.repo(t, 20, "Go") // no open issues: no slot
	_ = empty
	other := f.repo(t, 40, "Rust")

	bigBest := f.issue(t, big, 1, "BEGINNER", 2, 0, t0)
	f.issue(t, big, 2, "ADVANCED", 8, 0, t0)
	f.issue(t, big, 3, "UNCLASSIFIED", nil, 0, t0)
	smallOnly := f.issue(t, small, 1, "UNCLASSIFIED", nil, 0, t0)
	otherBest := f.issue(t, other, 1, "INTERMEDIATE", 5, 0, t0)

	want := []int{smallOnly, bigBest, otherBest} // shuffle keys 10, 30, 40
	for _, rareBelow := range []int{0, 1000} {   // 0 walks repositories; 1000 makes them rare
		for _, limit := range []int{1, 2, 10} {
			got := f.walk(t, Filters{Sort: SortShuffle, Limit: limit, RareLanguageBelow: rareBelow})
			if !slices.Equal(got, want) {
				t.Errorf("rare below %d, limit %d: %v, want %v", rareBelow, limit, got, want)
			}
		}
	}

	// A filter moves each repository's slot to its best matching issue.
	got := f.walk(t, Filters{Sort: SortShuffle, Limit: 10, Difficulties: []string{"ADVANCED", "INTERMEDIATE"}})
	if len(got) != 2 || got[1] != otherBest {
		t.Errorf("filtered shuffle = %v, want big's ADVANCED issue then %d", got, otherBest)
	}
}

// Newest walks by created_at, newest first, across pages.
func TestNewestSort(t *testing.T) {
	f := newFilterFixture(t)
	repo := f.repo(t, 1, "Go")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var want []int
	for n := 5; n >= 1; n-- {
		// Inserted oldest-last so insertion order is not the answer.
		want = append(want, f.issue(t, repo, n, "UNCLASSIFIED", nil, 0, t0.Add(time.Duration(n)*time.Hour)))
	}
	for _, rareBelow := range []int{0, 1000} {
		if got := f.walk(t, Filters{Sort: SortNewest, Limit: 2, RareLanguageBelow: rareBelow}); !slices.Equal(got, want) {
			t.Errorf("rare below %d: %v, want %v", rareBelow, got, want)
		}
	}
}

// Every filter narrows the catalog; values within one filter are alternatives.
func TestFilters(t *testing.T) {
	f := newFilterFixture(t)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	goRepo := f.repo(t, 1, "Go", "Go", "Shell")
	tsRepo := f.repo(t, 2, "TypeScript", "TypeScript", "JavaScript")

	bug := f.issue(t, goRepo, 1, "BEGINNER", 2, 0, t0, "Bug")
	docs := f.issue(t, goRepo, 2, "INTERMEDIATE", 5, 7, t0, "docs")
	busy := f.issue(t, tsRepo, 1, "UNCLASSIFIED", nil, 25, t0, "enhancement")
	none := f.issue(t, tsRepo, 2, "UNCLASSIFIED", nil, 3, t0)

	five, ten := 5, 10
	for name, tc := range map[string]struct {
		f    Filters
		want []int
	}{
		"label, case-insensitive":  {Filters{Labels: []string{"bug"}}, []int{bug}},
		"labels, any of":           {Filters{Labels: []string{"BUG", "docs"}}, []int{bug, docs}},
		"language":                 {Filters{Languages: []string{"typescript"}}, []int{busy, none}},
		"tech, any language used":  {Filters{Tech: []string{"Shell"}}, []int{bug, docs}},
		"tech, any of":             {Filters{Tech: []string{"javascript", "shell"}}, []int{bug, docs, busy, none}},
		"difficulty, any of":       {Filters{Difficulties: []string{"BEGINNER", "UNCLASSIFIED"}}, []int{bug, busy, none}},
		"comments range":           {Filters{CommentsMin: &five, CommentsMax: &ten}, []int{docs}},
		"no comments":              {Filters{CommentsMax: new(int)}, []int{bug}},
		"across filters, all of":   {Filters{Tech: []string{"go"}, Difficulties: []string{"INTERMEDIATE"}}, []int{docs}},
		"tech nobody has, nothing": {Filters{Tech: []string{"no-such-language-anywhere"}}, nil},
	} {
		for _, rareBelow := range []int{0, 1000} {
			fl := tc.f
			fl.Sort, fl.Limit, fl.RareLanguageBelow = SortEasiest, 2, rareBelow
			got := f.walk(t, fl)
			slices.Sort(got)
			want := slices.Clone(tc.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("%s (rare below %d): %v, want %v", name, rareBelow, got, want)
			}
		}
	}
}

// A cursor from one sort is refused by another.
func TestCursorSortMismatch(t *testing.T) {
	f := newFilterFixture(t)
	_, _, err := f.svc.BrowseIssues(context.Background(), Filters{Sort: SortNewest}, &Cursor{Sort: SortShuffle, Key: 1, ID: 1}, nil)
	if !errors.Is(err, ErrBadCursor) {
		t.Errorf("err = %v, want ErrBadCursor", err)
	}
}

// Recommendation candidates come from the caller's languages and level first,
// and widen only when those hold fewer than wanted.
func TestRecommendationCandidates(t *testing.T) {
	f := newFilterFixture(t)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tech := f.topic + "lang"
	a := f.repo(t, 1, "Go", tech)
	b := f.repo(t, 2, "Go", tech)
	aEasy := f.issue(t, a, 1, "BEGINNER", 2, 0, t0)
	aHard := f.issue(t, a, 2, "ADVANCED", 8, 0, t0)
	bEasy := f.issue(t, b, 1, "BEGINNER", 2, 0, t0)
	// Repositories outside the language, so widening to the whole catalog
	// finds something even in an otherwise empty database.
	for k := 0; k < 6; k++ {
		f.issue(t, f.repo(t, 10+k, "Rust"), 1, "BEGINNER", 2, 0, t0)
	}

	ctx := context.Background()
	for _, rareBelow := range []int{0, 1000} {
		base := Filters{RareLanguageBelow: rareBelow}

		got, err := f.svc.RecommendationCandidates(ctx, base, []string{" " + tech + " "}, "beginner", 1, 10)
		if err != nil {
			t.Fatal(err)
		}
		if ids := idsOf(got); !slices.Equal(ids, []int{aEasy, bEasy}) {
			t.Errorf("rare below %d, beginner: %v, want %v", rareBelow, ids, []int{aEasy, bEasy})
		}

		got, err = f.svc.RecommendationCandidates(ctx, base, []string{tech}, "ADVANCED", 1, 10)
		if err != nil {
			t.Fatal(err)
		}
		if ids := idsOf(got); !slices.Equal(ids, []int{aHard}) {
			t.Errorf("rare below %d, advanced: %v, want %v", rareBelow, ids, []int{aHard})
		}

		// Wanting more than the languages hold widens to the whole catalog,
		// the matching issues first, without repeating one.
		got, err = f.svc.RecommendationCandidates(ctx, base, []string{tech}, "ADVANCED", 5, 8)
		if err != nil {
			t.Fatal(err)
		}
		ids := idsOf(got)
		if len(ids) != 8 || ids[0] != aHard || !slices.Contains(ids[1:3], aEasy) || !slices.Contains(ids[1:3], bEasy) {
			t.Errorf("rare below %d, widened: %v, want %d, then %d and %d, then 5 more", rareBelow, ids, aHard, aEasy, bEasy)
		}
		slices.Sort(ids)
		if len(slices.Compact(ids)) != 8 {
			t.Errorf("rare below %d, widened: an issue listed twice", rareBelow)
		}
	}
}
