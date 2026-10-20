package ingest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
)

type statDelta struct{ indexed, arrived, opened, closed int }

// repoDeltas sums the unfolded deltas the issues triggers wrote for one
// repository.
func repoDeltas(t *testing.T, pool *db.Pool, repoID int) statDelta {
	t.Helper()
	var d statDelta
	if err := pool.QueryRow(context.Background(), `
		SELECT coalesce(sum(indexed), 0), coalesce(sum(arrived), 0), coalesce(sum(opened), 0), coalesce(sum(closed), 0)
		FROM catalog_stat_deltas WHERE repository_id = $1`, []any{repoID},
		func(row pgx.Row) error { return row.Scan(&d.indexed, &d.arrived, &d.opened, &d.closed) }); err != nil {
		t.Fatal(err)
	}
	return d
}

func statsRepo(t *testing.T, pool *db.Pool) int {
	t.Helper()
	ctx := context.Background()
	var id int
	if err := pool.QueryRow(ctx,
		`INSERT INTO repositories (full_name, url) VALUES ($1, 'u') RETURNING id`,
		[]any{fmt.Sprintf("stats-test/r%d", time.Now().UnixNano())},
		func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		if err := pool.Exec(ctx, `DELETE FROM repositories WHERE id = $1`, id); err != nil {
			t.Errorf("cleanup: %v", err)
		}
		if err := pool.Exec(ctx, `DELETE FROM catalog_stat_deltas WHERE repository_id = $1`, id); err != nil {
			t.Errorf("cleanup: %v", err)
		}
		if err := pool.Exec(ctx, `DELETE FROM catalog_activity WHERE repository_id = $1`, id); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return id
}

// Every way an issue changes is counted, whichever statement made it.
func TestStatsTriggersCountEveryChange(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := statsRepo(t, pool)
	exec := func(sql string, args ...any) {
		t.Helper()
		if err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	want := func(step string, w statDelta) {
		t.Helper()
		if got := repoDeltas(t, pool, repo); got != w {
			t.Errorf("after %s: deltas %+v, want %+v", step, got, w)
		}
	}

	// Three open and one closed, in one statement.
	exec(`INSERT INTO issues (repository_id, number, title, url, difficulty, state)
	      VALUES ($1, 1, 't', 'u', 'UNCLASSIFIED', 'OPEN'), ($1, 2, 't', 'u', 'UNCLASSIFIED', 'OPEN'),
	             ($1, 3, 't', 'u', 'UNCLASSIFIED', 'OPEN'), ($1, 4, 't', 'u', 'UNCLASSIFIED', 'CLOSED')`, repo)
	want("insert", statDelta{indexed: 4, arrived: 3, opened: 3})

	// An upsert that inserts one and updates one without changing its state.
	exec(`INSERT INTO issues (repository_id, number, title, url, difficulty, state)
	      VALUES ($1, 1, 'new title', 'u', 'UNCLASSIFIED', 'OPEN'), ($1, 5, 't', 'u', 'UNCLASSIFIED', 'OPEN')
	      ON CONFLICT (repository_id, number) DO UPDATE SET title = EXCLUDED.title`, repo)
	want("upsert", statDelta{indexed: 5, arrived: 4, opened: 4})

	exec(`UPDATE issues SET state = 'CLOSED' WHERE repository_id = $1 AND number IN (1, 2)`, repo)
	want("close", statDelta{indexed: 5, arrived: 4, opened: 2, closed: 2})

	exec(`UPDATE issues SET state = 'OPEN' WHERE repository_id = $1 AND number = 4`, repo)
	want("reopen", statDelta{indexed: 5, arrived: 4, opened: 3, closed: 2})

	exec(`UPDATE issues SET title = 'x' WHERE repository_id = $1`, repo)
	want("an update that changes no state", statDelta{indexed: 5, arrived: 4, opened: 3, closed: 2})

	exec(`DELETE FROM issues WHERE repository_id = $1 AND number IN (1, 3)`, repo) // one closed, one open
	want("delete", statDelta{indexed: 5, arrived: 4, opened: 2, closed: 2})
}

// A rollup folds the deltas into the totals, today's counts and the activity
// list, and leaves none behind.
func TestRollupStatsFoldsDeltas(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if _, err := RollupStats(ctx, pool, true, 50); err != nil {
		t.Fatal(err)
	}
	before := readStats(t, pool)
	repo := statsRepo(t, pool)
	if err := pool.Exec(ctx, `INSERT INTO issues (repository_id, number, title, url, difficulty, state)
	      VALUES ($1, 1, 't', 'u', 'UNCLASSIFIED', 'OPEN'), ($1, 2, 't', 'u', 'UNCLASSIFIED', 'OPEN')`, repo); err != nil {
		t.Fatal(err)
	}
	if err := pool.Exec(ctx, `UPDATE issues SET state = 'CLOSED' WHERE repository_id = $1 AND number = 1`, repo); err != nil {
		t.Fatal(err)
	}

	if _, err := RollupStats(ctx, pool, false, 50); err != nil {
		t.Fatal(err)
	}
	if d := repoDeltas(t, pool, repo); d != (statDelta{}) {
		t.Errorf("deltas left after a rollup: %+v", d)
	}
	after := readStats(t, pool)
	// Other tests write issues concurrently, so the totals moved at least by
	// this test's changes.
	if after["issues_indexed"]-before["issues_indexed"] < 2 || after["closed_since"]-before["closed_since"] < 1 {
		t.Errorf("totals moved %v -> %v, want indexed +2 and closed +1 at least", before, after)
	}
	var arrived, closed int
	if err := pool.QueryRow(ctx, `SELECT coalesce(sum(arrived), 0), coalesce(sum(closed), 0) FROM catalog_activity WHERE repository_id = $1`,
		[]any{repo}, func(row pgx.Row) error { return row.Scan(&arrived, &closed) }); err != nil {
		t.Fatal(err)
	}
	if arrived != 2 || closed != 1 {
		t.Errorf("activity for the repository: %d arrived, %d closed, want 2 and 1", arrived, closed)
	}

	// Several statements in one transaction are one activity row.
	if err := pool.InTx(ctx, func(tx pgx.Tx) error {
		for n := 3; n <= 4; n++ {
			if _, err := tx.Exec(ctx, `INSERT INTO issues (repository_id, number, title, url, difficulty, state)
			      VALUES ($1, $2, 't', 'u', 'UNCLASSIFIED', 'OPEN')`, repo, n); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RollupStats(ctx, pool, false, 50); err != nil {
		t.Fatal(err)
	}
	var rows, sum int
	if err := pool.QueryRow(ctx, `SELECT count(*), coalesce(sum(arrived), 0) FROM catalog_activity
	      WHERE repository_id = $1 AND arrived = 2`, []any{repo}, func(row pgx.Row) error { return row.Scan(&rows, &sum) }); err != nil {
		t.Fatal(err)
	}
	if rows != 2 { // the first transaction's row and this one's
		t.Errorf("one transaction of two inserts: %d activity rows with arrived=2, want 2 in all", rows)
	}
}

func readStats(t *testing.T, pool *db.Pool) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	if err := pool.QueryRows(context.Background(), `SELECT key, value FROM catalog_stats`, nil, func(rows pgx.Rows) error {
		var k string
		var v int64
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		out[k] = v
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"open_issues", "issues_indexed", "repositories_indexed", "closed_since"} {
		if _, ok := out[k]; !ok {
			t.Fatalf("catalog_stats has no %s after a recount", k)
		}
	}
	return out
}
