package report

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

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

func insertID(t *testing.T, pool *db.Pool, cleanup, q string, args ...any) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), q, args, func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), cleanup, id); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return id
}

func TestReports(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	stamp := time.Now().UnixNano()
	user := func(n int) int {
		id, _ := strconv.Atoi(insertID(t, pool, `DELETE FROM users WHERE id = $1::int`,
			`INSERT INTO users (github_id, username) VALUES ($1, $1) RETURNING id::text`, fmt.Sprintf("report-%d-%d", stamp, n)))
		return id
	}
	a, b, c := user(1), user(2), user(3)
	name := fmt.Sprintf("report-test/repo-%d", stamp)
	repo := insertID(t, pool, `DELETE FROM repositories WHERE id = $1::int`,
		`INSERT INTO repositories (full_name, url) VALUES ($1, 'u') RETURNING id::text`, name)
	t.Cleanup(func() {
		_ = pool.Exec(context.Background(), `DELETE FROM excluded_repositories WHERE lower(full_name) = lower($1)`, name)
	})
	issue := insertID(t, pool, `DELETE FROM issues WHERE id = $1::int`,
		`INSERT INTO issues (repository_id, number, title, url, difficulty) VALUES ($1::int, 1, 't', 'u', 'UNCLASSIFIED') RETURNING id::text`, repo)
	public := insertID(t, pool, `DELETE FROM user_themes WHERE id = $1::uuid`,
		`INSERT INTO user_themes (user_id, name, toml, css, public) VALUES ($1, 'Loud', '', '', true) RETURNING id::text`, c)
	private := insertID(t, pool, `DELETE FROM user_themes WHERE id = $1::uuid`,
		`INSERT INTO user_themes (user_id, name, toml, css, public) VALUES ($1, 'Mine', '', '', false) RETURNING id::text`, c)
	if err := pool.Exec(ctx, `UPDATE users SET active_theme_id = $1::uuid WHERE id = $2`, public, a); err != nil {
		t.Fatal(err)
	}

	svc := NewService(pool)
	issueT := Target{Type: Issue, ID: issue}

	// Refusals.
	for name, err := range map[string]error{
		"unknown reason":  svc.Create(ctx, a, issueT, "boring", "", 20),
		"unknown type":    svc.Create(ctx, a, Target{Type: "user", ID: "1"}, "spam", "", 20),
		"no such issue":   svc.Create(ctx, a, Target{Type: Issue, ID: "2147483000"}, "spam", "", 20),
		"private theme":   svc.Create(ctx, a, Target{Type: Theme, ID: private}, "abuse", "", 20),
		"malformed theme": svc.Create(ctx, a, Target{Type: Theme, ID: "x'; --"}, "abuse", "", 20),
	} {
		if err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	// One open report per person per target; the daily limit holds.
	if err := svc.Create(ctx, a, issueT, "spam", "  selling things ", 20); err != nil {
		t.Fatal(err)
	}
	if err := svc.Create(ctx, a, issueT, "abuse", "", 20); !errors.Is(err, ErrDuplicate) {
		t.Errorf("second report: %v, want ErrDuplicate", err)
	}
	if err := svc.Create(ctx, a, Target{Type: Repository, ID: repo}, "spam", "", 1); !errors.Is(err, ErrTooMany) {
		t.Errorf("past the daily limit: %v, want ErrTooMany", err)
	}

	hidden := func() Hidden {
		h, err := svc.Hidden(ctx, 2)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	if hidden().Issues[mustInt(issue)] {
		t.Error("one report hid the issue; the threshold is two")
	}
	if err := svc.Create(ctx, b, issueT, "already-fixed", "", 20); err != nil {
		t.Fatal(err)
	}
	if !hidden().Issues[mustInt(issue)] {
		t.Error("two reports did not hide the issue")
	}

	// Dismissed: back in lists.
	if err := svc.Resolve(ctx, issueT, false); err != nil {
		t.Fatal(err)
	}
	if hidden().Issues[mustInt(issue)] {
		t.Error("a dismissed issue is still hidden")
	}
	if err := svc.Resolve(ctx, issueT, false); !errors.Is(err, ErrNotFound) {
		t.Errorf("resolving with nothing open: %v, want ErrNotFound", err)
	}

	// Upheld: the repository is excluded, the theme made private and taken off accounts.
	for _, tg := range []Target{{Type: Repository, ID: repo}, {Type: Theme, ID: public}} {
		reason := "spam"
		if tg.Type == Theme {
			reason = "abuse"
		}
		if err := svc.Create(ctx, b, tg, reason, "", 20); err != nil {
			t.Fatal(err)
		}
		if err := svc.Resolve(ctx, tg, true); err != nil {
			t.Fatal(err)
		}
	}
	var excluded, isPublic bool
	var active *string
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM excluded_repositories WHERE lower(full_name) = lower($1)),
		(SELECT public FROM user_themes WHERE id = $2::uuid), (SELECT active_theme_id::text FROM users WHERE id = $3)`,
		[]any{name, public, a}, func(row pgx.Row) error { return row.Scan(&excluded, &isPublic, &active) }); err != nil {
		t.Fatal(err)
	}
	if !excluded || isPublic || active != nil {
		t.Errorf("upheld: excluded %v, theme public %v, still applied %v", excluded, isPublic, active)
	}
	if h := hidden(); !h.Repositories[mustInt(repo)] || !h.Themes[public] {
		t.Errorf("upheld targets are not hidden: %+v", h)
	}

	open, err := svc.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range open {
		if s.ID == issue || s.ID == repo || s.ID == public {
			t.Errorf("a resolved target is still open: %+v", s)
		}
	}
}

func mustInt(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return n
}
