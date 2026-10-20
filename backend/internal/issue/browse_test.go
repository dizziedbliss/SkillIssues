package issue

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

func TestCursorRoundTrip(t *testing.T) {
	five := 5
	for _, c := range []Cursor{
		{Bucket: 0, Score: &five, Updated: 1760000000123456, ID: 42},
		{Bucket: 1, Score: nil, Updated: 1760000000000001, ID: 7},
	} {
		got, err := DecodeCursor(c.Encode())
		if err != nil {
			t.Fatalf("decode %+v: %v", c, err)
		}
		if got.Bucket != c.Bucket || got.Updated != c.Updated || got.ID != c.ID ||
			(got.Score == nil) != (c.Score == nil) || (c.Score != nil && *got.Score != *c.Score) {
			t.Errorf("round trip: got %+v, want %+v", got, c)
		}
	}
}

func TestDecodeCursorRejectsForeignTokens(t *testing.T) {
	enc := func(raw string) string { return base64.RawURLEncoding.EncodeToString([]byte(raw)) }
	for _, token := range []string{
		"",
		"not base64!",
		enc("0..1"),          // a field missing
		enc("2.5.1760.42"),   // no bucket 2
		enc("0.x.1760.42"),   // score not a number
		enc("0.5.soon.42"),   // updated not a number
		enc("0.5.1760.0"),    // id 0
		enc("0.5.1760.42.9"), // a field too many
	} {
		if _, err := DecodeCursor(token); err != ErrBadCursor {
			t.Errorf("DecodeCursor(%q) = %v, want ErrBadCursor", token, err)
		}
	}
}

func testPool(t *testing.T) *db.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := db.New(context.Background(), db.DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedBrowseRepo inserts a repository in a language no other repository has,
// so a language filter on it sees only the issues a test writes, and returns
// its id and that language.
func seedBrowseRepo(t *testing.T, pool *db.Pool, tag string) (int, string) {
	t.Helper()
	ctx := context.Background()
	lang := fmt.Sprintf("browse-%s-%d", tag, time.Now().UnixNano())
	var repoID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO repositories (full_name, url, language) VALUES ($1, 'u', $2) RETURNING id`,
		[]any{"browse-test/" + lang, lang}, func(row pgx.Row) error { return row.Scan(&repoID) }); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, repoID); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return repoID, lang
}

// seedBrowseIssues writes n open issues to the repository: classified and
// unclassified, with scores and updated_at values that repeat so the cursor
// has to break ties on id, and one closed issue that must never be listed.
func seedBrowseIssues(t *testing.T, pool *db.Pool, repoID, n int) {
	t.Helper()
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 123456000, time.UTC)
	for i := 1; i <= n; i++ {
		var score any
		difficulty := "UNCLASSIFIED"
		switch i % 4 {
		case 0:
			score, difficulty = 2, "BEGINNER"
		case 1:
			score, difficulty = 5, "INTERMEDIATE"
		case 2:
			score, difficulty = 8, "ADVANCED"
		}
		updated := base.Add(time.Duration(i%3) * time.Hour)
		if err := pool.Exec(ctx,
			`INSERT INTO issues (repository_id, number, title, url, difficulty_score, difficulty, updated_at)
			 VALUES ($1, $2, 't', 'u', $3, $4, $5)`,
			repoID, i, score, difficulty, updated); err != nil {
			t.Fatalf("insert issue %d: %v", i, err)
		}
	}
	if err := pool.Exec(ctx,
		`INSERT INTO issues (repository_id, number, title, url, difficulty, state)
		 VALUES ($1, $2, 't', 'u', 'UNCLASSIFIED', 'CLOSED')`, repoID, n+1); err != nil {
		t.Fatalf("insert closed issue: %v", err)
	}
}

// catalogIDs is the reference order: one sorted query over the same filter.
// An empty difficulty is no filter.
func catalogIDs(t *testing.T, pool *db.Pool, lang, difficulty string) []int {
	t.Helper()
	var ids []int
	err := pool.QueryRows(context.Background(),
		`SELECT i.id FROM issues i JOIN repositories r ON r.id = i.repository_id
		 WHERE i.state = 'OPEN' AND r.language = $1 AND ($2 = '' OR i.difficulty = $2)
		 ORDER BY `+catalogOrder,
		[]any{lang, difficulty}, func(rows pgx.Rows) error {
			var id int
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
			return nil
		})
	if err != nil {
		t.Fatalf("reference order: %v", err)
	}
	return ids
}

func idsOf(issues []Issue) []int {
	out := make([]int, len(issues))
	for i, it := range issues {
		out[i] = it.ID
	}
	return out
}

// walkCatalog pages through BrowseIssues until the cursor runs out, passing
// each cursor through its token as a client would, and returns every id
// listed. stop, if not nil, ends the walk early once it returns true.
func walkCatalog(t *testing.T, svc *Service, f Filters, prefLangs []string, stop func([]int) bool) []int {
	t.Helper()
	var got []int
	var after *Cursor
	for pages := 0; ; pages++ {
		if pages > 1000 {
			t.Fatal("cursor never ran out")
		}
		page, next, err := svc.BrowseIssues(context.Background(), f, after, prefLangs)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, idsOf(page)...)
		if next == nil || (stop != nil && stop(got)) {
			return got
		}
		decoded, err := DecodeCursor(next.Encode())
		if err != nil {
			t.Fatalf("own cursor did not decode: %v", err)
		}
		after = &decoded
	}
}

// Both ways of reading a language: 1 makes a test repository's issues common,
// so the catalog index is walked; 1000 makes them rare, so they are sorted.
var rareThresholds = []int{1, 1000}

// Paging by cursor visits every open issue exactly once, in catalog order,
// across score boundaries, ties on updated_at and the unclassified tail, and
// says when it has run out.
func TestBrowseIssuesCursorWalksCatalogOrder(t *testing.T) {
	pool := testPool(t)
	svc := New(pool, nil)
	repoID, lang := seedBrowseRepo(t, pool, "walk")
	seedBrowseIssues(t, pool, repoID, 23)
	want := catalogIDs(t, pool, lang, "")
	if len(want) != 23 {
		t.Fatalf("reference order has %d issues, want 23", len(want))
	}

	for _, rareBelow := range rareThresholds {
		for _, limit := range []int{1, 4, 5, 23, 50} {
			got := walkCatalog(t, svc, Filters{Languages: []string{lang}, Limit: limit, RareLanguageBelow: rareBelow}, nil, nil)
			if !slices.Equal(got, want) {
				t.Errorf("rare below %d, limit %d: walked %v, want %v", rareBelow, limit, got, want)
			}
		}
	}
}

// A difficulty filter walks only that difficulty, in the same order. The
// filter is upper-cased, as a client may send it either way.
func TestBrowseIssuesDifficultyFilter(t *testing.T) {
	pool := testPool(t)
	svc := New(pool, nil)
	repoID, lang := seedBrowseRepo(t, pool, "difficulty")
	seedBrowseIssues(t, pool, repoID, 23)

	for _, rareBelow := range rareThresholds {
		for _, difficulty := range []string{"UNCLASSIFIED", "intermediate"} {
			want := catalogIDs(t, pool, lang, strings.ToUpper(difficulty))
			if len(want) == 0 {
				t.Fatalf("%s: seed has no such issues", difficulty)
			}
			got := walkCatalog(t, svc,
				Filters{Languages: []string{lang}, Difficulties: []string{strings.ToUpper(difficulty)}, Limit: 2, RareLanguageBelow: rareBelow}, nil, nil)
			if !slices.Equal(got, want) {
				t.Errorf("rare below %d, %s: walked %v, want %v", rareBelow, difficulty, got, want)
			}
		}
	}
}

// With preferred languages, every issue in them comes before any other, and
// the cursor carries the walk on from the first bucket into the second.
func TestBrowseIssuesPreferredLanguagesFirst(t *testing.T) {
	pool := testPool(t)
	svc := New(pool, nil)
	repoID, lang := seedBrowseRepo(t, pool, "preferred")
	seedBrowseIssues(t, pool, repoID, 7)
	want := catalogIDs(t, pool, lang, "")

	for _, rareBelow := range rareThresholds {
		// Walk until two pages past the preferred issues: the rest of the
		// catalog is every other issue in the database.
		got := walkCatalog(t, svc, Filters{Limit: 3, RareLanguageBelow: rareBelow},
			[]string{"  " + strings.ToUpper(lang) + " "},
			func(got []int) bool { return len(got) >= len(want)+6 })
		if len(got) < len(want) || !slices.Equal(got[:len(want)], want) {
			t.Fatalf("rare below %d: walk began %v, want the preferred issues %v", rareBelow, got, want)
		}
		for _, id := range got[len(want):] {
			if slices.Contains(want, id) {
				t.Fatalf("rare below %d: preferred issue %d listed again after its bucket", rareBelow, id)
			}
		}
	}
}

// Under a language filter the preferred languages change nothing: the walk is
// the filter's catalog order whether or not the language is preferred.
func TestBrowseIssuesLanguageFilterIgnoresPreference(t *testing.T) {
	pool := testPool(t)
	svc := New(pool, nil)
	repoID, lang := seedBrowseRepo(t, pool, "filter-preferred")
	seedBrowseIssues(t, pool, repoID, 9)
	want := catalogIDs(t, pool, lang, "")

	for _, prefs := range [][]string{{lang}, {"some-other-language"}} {
		got := walkCatalog(t, svc, Filters{Languages: []string{lang}, Limit: 4, RareLanguageBelow: 1000}, prefs, nil)
		if !slices.Equal(got, want) {
			t.Errorf("preferring %v: walked %v, want %v", prefs, got, want)
		}
	}
}

// The OFFSET query and the cursor walk agree on the order, so a client moving
// from ?page to ?cursor sees the same catalog.
func TestSearchIssuesPageMatchesBrowseOrder(t *testing.T) {
	pool := testPool(t)
	svc := New(pool, nil)
	repoID, lang := seedBrowseRepo(t, pool, "offset")
	seedBrowseIssues(t, pool, repoID, 12)
	want := catalogIDs(t, pool, lang, "")

	var got []int
	for page := 1; page <= 3; page++ {
		issues, err := svc.SearchIssues(context.Background(), Filters{Languages: []string{lang}, Limit: 4, Page: page}, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, idsOf(issues)...)
	}
	if !slices.Equal(got, want) {
		t.Errorf("offset pages %v, want %v", got, want)
	}
}
