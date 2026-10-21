package migrate

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func rolesTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// asRole runs fn in a transaction as role (SET LOCAL ROLE), then rolls back,
// so a test can try what a service could do without leaving anything behind.
// The connection is the owner's, so a missing role fails here, not silently.
func asRole(t *testing.T, pool *pgxpool.Pool, role string, setup func(pgx.Tx), fn func(tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if setup != nil {
		setup(tx)
	}
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+role); err != nil {
		t.Fatalf("set role %s: %v", role, err)
	}
	fn(tx)
}

// try runs one statement in a savepoint so a denied statement does not abort
// the surrounding transaction.
func try(tx pgx.Tx, sql string, args ...any) error {
	ctx := context.Background()
	sp, err := tx.Begin(ctx)
	if err != nil {
		return err
	}
	if _, err := sp.Exec(ctx, sql, args...); err != nil {
		_ = sp.Rollback(ctx)
		return err
	}
	return sp.Commit(ctx)
}

func mustDo(t *testing.T, tx pgx.Tx, sql string, args ...any) {
	t.Helper()
	if err := try(tx, sql, args...); err != nil {
		t.Errorf("%s: %v", sql, err)
	}
}

func mustFail(t *testing.T, tx pgx.Tx, sql string, args ...any) {
	t.Helper()
	if err := try(tx, sql, args...); err == nil {
		t.Errorf("expected permission failure, statement succeeded: %s", sql)
	}
}

// Every table is either granted to one of the service roles or listed here as
// deliberately granted to neither. A new table that is in neither fails this
// test, which is what makes "forgot to grant it" loud.
var ungranted = map[string]bool{
	"goose_db_version": true, // the migration ledger; only the owner touches it
}

func TestEveryTableIsClassified(t *testing.T) {
	pool := rolesTestPool(t)
	ctx := context.Background()
	rows, err := pool.Query(ctx, `
		SELECT c.relname,
		       has_table_privilege('skillissues_api',    c.oid, 'SELECT,INSERT,UPDATE,DELETE'),
		       has_table_privilege('skillissues_ingest', c.oid, 'SELECT,INSERT,UPDATE,DELETE')
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r'
		ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var api, ingest bool
		if err := rows.Scan(&name, &api, &ingest); err != nil {
			t.Fatal(err)
		}
		// has_table_privilege with a list is true if ANY listed privilege is held.
		if !api && !ingest && !ungranted[name] {
			t.Errorf("table %s is granted to neither skillissues_api nor skillissues_ingest and is not in the ungranted list: grant it in its migration, or list it here", name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

// A role that can INSERT into a table with a serial column must be able to use
// its sequence, or every insert fails at runtime.
func TestInsertGrantsComeWithSequenceUsage(t *testing.T) {
	pool := rolesTestPool(t)
	ctx := context.Background()
	for _, role := range []string{"skillissues_api", "skillissues_ingest"} {
		rows, err := pool.Query(ctx, `
			SELECT c.relname, a.attname, pg_get_serial_sequence(format('%I.%I', n.nspname, c.relname), a.attname)
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
			WHERE n.nspname = 'public' AND c.relkind = 'r'
			  AND pg_get_serial_sequence(format('%I.%I', n.nspname, c.relname), a.attname) IS NOT NULL
			  AND has_table_privilege($1, c.oid, 'INSERT')`, role)
		if err != nil {
			t.Fatal(err)
		}
		type seq struct{ table, col, name string }
		var seqs []seq
		for rows.Next() {
			var s seq
			if err := rows.Scan(&s.table, &s.col, &s.name); err != nil {
				t.Fatal(err)
			}
			seqs = append(seqs, s)
		}
		rows.Close()
		for _, s := range seqs {
			var ok bool
			if err := pool.QueryRow(ctx, `SELECT has_sequence_privilege($1, $2, 'USAGE')`, role, s.name).Scan(&ok); err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Errorf("%s can INSERT into %s but has no USAGE on %s", role, s.table, s.name)
			}
		}
	}
}

func TestAPIRoleCanDoItsJobAndNothingMore(t *testing.T) {
	pool := rolesTestPool(t)
	setup := func(tx pgx.Tx) {
		ctx := context.Background()
		for _, q := range []string{
			`INSERT INTO repositories (id, full_name, url) VALUES (910001, 'roles-test/repo', 'u')`,
			`INSERT INTO issues (id, repository_id, number, title, url, difficulty, state) VALUES (910001, 910001, 1, 't', 'u', 'UNCLASSIFIED', 'OPEN')`,
		} {
			if _, err := tx.Exec(ctx, q); err != nil {
				t.Fatalf("setup %s: %v", q, err)
			}
		}
	}
	asRole(t, pool, "skillissues_api", setup, func(tx pgx.Tx) {
		// What the API does.
		mustDo(t, tx, `INSERT INTO users (github_id, username) VALUES ('roles-test-1', 'roles-test-1')`)
		mustDo(t, tx, `INSERT INTO auth_sessions (token, user_id) SELECT 'h', id FROM users WHERE github_id = 'roles-test-1'`)
		mustDo(t, tx, `INSERT INTO oauth_states (state, code_verifier, redirect_uri) VALUES ('s', 'v', 'r')`)
		mustDo(t, tx, `DELETE FROM oauth_states WHERE state = 's' RETURNING code_verifier`)
		mustDo(t, tx, `INSERT INTO developer_profiles (user_id) SELECT id FROM users WHERE github_id = 'roles-test-1'`)
		mustDo(t, tx, `INSERT INTO saved_issues (user_id, issue_id) SELECT id, 910001 FROM users WHERE github_id = 'roles-test-1'`)
		mustDo(t, tx, `INSERT INTO webhook_events (delivery_id, event_name) VALUES ('d', 'e') ON CONFLICT (delivery_id) DO NOTHING RETURNING delivery_id`)
		// Closing a saved issue fires the notification trigger as this role.
		mustDo(t, tx, `UPDATE issues SET state = 'CLOSED' WHERE id = 910001`)
		mustDo(t, tx, `UPDATE notifications SET read_at = now()`)
		// An admin settings edit fires the audit trigger as this role.
		mustDo(t, tx, `UPDATE settings SET value = value, updated_by = 'roles-test' WHERE key = 'api.session.ttl_hours'`)
		mustDo(t, tx, `DELETE FROM users WHERE github_id = 'roles-test-1'`)

		// What it must not be able to do.
		mustFail(t, tx, `DROP TABLE users`)
		mustFail(t, tx, `TRUNCATE issues`)
		mustFail(t, tx, `CREATE TABLE roles_test_t (id int)`)
		mustFail(t, tx, `ALTER TABLE users ADD COLUMN x int`)
		mustFail(t, tx, `COPY (SELECT 1) TO PROGRAM 'true'`)
		mustFail(t, tx, `SELECT 1 FROM ingest_jobs`)
		mustFail(t, tx, `SELECT 1 FROM github_budget`)
		mustFail(t, tx, `UPDATE repositories SET stars = 0`)
		mustFail(t, tx, `DELETE FROM issues`)
		mustFail(t, tx, `CREATE ROLE roles_test_r`)
		mustFail(t, tx, `SELECT 1 FROM goose_db_version`)
	})
}

func TestIngestRoleCanDoItsJobAndNothingMore(t *testing.T) {
	pool := rolesTestPool(t)
	setup := func(tx pgx.Tx) {
		ctx := context.Background()
		for _, q := range []string{
			`INSERT INTO users (id, github_id, username) VALUES (910002, 'roles-test-2', 'roles-test-2')`,
			`INSERT INTO repositories (id, full_name, url) VALUES (910002, 'roles-test/repo2', 'u')`,
			`INSERT INTO issues (id, repository_id, number, title, url, difficulty, state) VALUES (910002, 910002, 1, 't', 'u', 'UNCLASSIFIED', 'OPEN')`,
			`INSERT INTO saved_issues (user_id, issue_id) VALUES (910002, 910002)`,
		} {
			if _, err := tx.Exec(ctx, q); err != nil {
				t.Fatalf("setup %s: %v", q, err)
			}
		}
	}
	asRole(t, pool, "skillissues_ingest", setup, func(tx pgx.Tx) {
		// What the crawl does.
		mustDo(t, tx, `INSERT INTO ingest_jobs (repository_id, job_type, status, next_retry_at) VALUES (910002, 'initial', 'pending', now())`)
		mustDo(t, tx, `UPDATE ingest_jobs SET status = 'running' WHERE repository_id = 910002`)
		mustDo(t, tx, `UPDATE repositories SET stars = 5, is_eligible = true WHERE id = 910002`)
		mustDo(t, tx, `INSERT INTO issues (repository_id, number, title, url, difficulty) VALUES (910002, 2, 't', 'u', 'UNCLASSIFIED')`)
		// Closing a saved issue fires the notification trigger as this role.
		mustDo(t, tx, `UPDATE issues SET state = 'CLOSED' WHERE id = 910002`)
		mustDo(t, tx, `DELETE FROM issues WHERE id = 910002`)
		mustDo(t, tx, `SELECT value FROM settings`)
		mustDo(t, tx, `SELECT full_name FROM excluded_repositories`)
		mustDo(t, tx, `INSERT INTO import_skipped (full_name, reason) VALUES ('x/y', 'r')`)
		// Pull request tracking: whom to read, and the ledger.
		mustDo(t, tx, `SELECT id, github_id, username, created_at FROM users`)
		mustDo(t, tx, `INSERT INTO pr_sync (user_id) VALUES (910002)`)
		mustDo(t, tx, `UPDATE pr_sync SET next_check_at = now() WHERE user_id = 910002`)
		mustDo(t, tx, `INSERT INTO pull_requests (github_id, user_id, repository_id, number, title, url, state, created_at, updated_at, merged_at, author_association)
			VALUES (910002, 910002, 910002, 1, 't', 'u', 'MERGED', now(), now(), now(), 'CONTRIBUTOR')`)
		mustDo(t, tx, `INSERT INTO pull_request_issues (pull_request_id, issue_id) SELECT p.id, i.id FROM pull_requests p, issues i WHERE p.github_id = 910002 AND i.repository_id = 910002 AND i.number = 2`)
		// A merge's XP fires the notification trigger as this role.
		mustDo(t, tx, `INSERT INTO xp_events (user_id, source, pull_request_id, amount, earned_at) SELECT 910002, 'merged', id, 75, now() FROM pull_requests WHERE github_id = 910002`)

		// What it must not be able to do.
		mustFail(t, tx, `SELECT name FROM users`)
		mustFail(t, tx, `SELECT * FROM users`)
		mustFail(t, tx, `UPDATE users SET username = 'x'`)
		mustFail(t, tx, `UPDATE xp_events SET amount = 0`)
		mustFail(t, tx, `DELETE FROM xp_events`)
		mustFail(t, tx, `SELECT 1 FROM auth_sessions`)
		mustFail(t, tx, `UPDATE settings SET value = value`)
		mustFail(t, tx, `DELETE FROM repositories`)
		mustFail(t, tx, `DROP TABLE issues`)
		mustFail(t, tx, `TRUNCATE ingest_jobs`)
		mustFail(t, tx, `COPY (SELECT 1) TO PROGRAM 'true'`)
		mustFail(t, tx, `INSERT INTO excluded_repositories (full_name, reason) VALUES ('a/b', 'c')`)
	})
}

func TestFlowRoleReadsSettingsAndNothingElse(t *testing.T) {
	pool := rolesTestPool(t)
	asRole(t, pool, "skillissues_flow", nil, func(tx pgx.Tx) {
		mustDo(t, tx, `SELECT key, value, value_type FROM settings`)

		mustFail(t, tx, `UPDATE settings SET value = value`)
		mustFail(t, tx, `SELECT 1 FROM users`)
		mustFail(t, tx, `SELECT 1 FROM auth_sessions`)
		mustFail(t, tx, `SELECT 1 FROM saved_issues`)
		mustFail(t, tx, `SELECT 1 FROM issues`)
		mustFail(t, tx, `SELECT 1 FROM repositories`)
		mustFail(t, tx, `SELECT 1 FROM settings_audit`)
		mustFail(t, tx, `CREATE TABLE roles_test_t (id int)`)
		mustFail(t, tx, `COPY (SELECT 1) TO PROGRAM 'true'`)
	})
}
