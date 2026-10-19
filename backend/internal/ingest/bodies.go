package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// The issue body cache (issue_bodies).
//
// Clients read issue bodies from GitHub themselves; this cache is the
// fallback for one that cannot, typically a signed-out browser past GitHub's
// 60 unauthenticated requests an hour. go-api serves hits; on a miss it asks
// ingestd, which holds the GitHub credential, to fetch the body here on the
// interactive lane. The archive reader refreshes bodies already cached; it
// never adds any. ingestd trims the cache back to ingest.bodies.cache_max_mb.

// BodyOutcome is how a fetch request ended.
type BodyOutcome string

const (
	BodyFetched      BodyOutcome = "fetched"
	BodyBudget       BodyOutcome = "budget"        // the interactive budget is spent; retry later
	BodyNoCredential BodyOutcome = "no-credential" // ingestd has no GitHub token
	BodyGone         BodyOutcome = "gone"          // GitHub no longer has the issue
	BodyNotStored    BodyOutcome = "not-stored"    // no such issue in the catalog
)

// FetchIssueBody fetches one stored issue's body from GitHub, stores it in
// issue_bodies and returns it. It spends the interactive lane: a user is
// waiting on it.
func (w *Worker) FetchIssueBody(ctx context.Context, issueID int) (string, BodyOutcome, error) {
	var fullName string
	var number int
	err := w.pool.QueryRow(ctx,
		`SELECT r.full_name, i.number FROM issues i JOIN repositories r ON r.id = i.repository_id WHERE i.id = $1`,
		[]any{issueID}, func(row pgx.Row) error { return row.Scan(&fullName, &number) })
	if errors.Is(err, db.ErrNotFound) {
		return "", BodyNotStored, nil
	}
	if err != nil {
		return "", "", fmt.Errorf("ingest: find issue %d: %w", issueID, err)
	}
	owner, name, ok := strings.Cut(fullName, "/")
	if !ok {
		return "", BodyGone, nil
	}

	token := w.token()
	if token == "" {
		return "", BodyNoCredential, nil
	}
	snap := w.live()
	proceed, err := CanProceed(ctx, w.pool, LaneInteractive, installationIdentity,
		snap.Int(settings.IngestRateLimitFloor), snap.Int(settings.IngestInteractiveReservePercent))
	if err != nil {
		metrics.BudgetErrors.WithLabelValues("read", string(LaneInteractive)).Inc()
		return "", "", err
	}
	if !proceed {
		return "", BodyBudget, nil
	}

	got, rl, err := w.github.FetchIssueBody(ctx, token, github.IssueRef{Owner: owner, Name: name, Number: number})
	if err != nil {
		if strings.Contains(err.Error(), "rate limit exceeded") {
			// As the crawl does: pause the credential rather than retry into it.
			until := time.Now().Add(snap.Seconds(settings.IngestRateLimitPauseSeconds))
			if err := MarkSecondaryLimit(ctx, w.pool, LaneInteractive, installationIdentity, until); err != nil {
				metrics.BudgetErrors.WithLabelValues("write", string(LaneInteractive)).Inc()
				slog.Warn("failed to record secondary rate limit pause", "error", err)
			}
			return "", BodyBudget, nil
		}
		return "", "", fmt.Errorf("ingest: fetch body of issue %d: %w", issueID, err)
	}
	if err := UpdateBudget(ctx, w.pool, LaneInteractive, installationIdentity, rl); err != nil {
		metrics.BudgetErrors.WithLabelValues("write", string(LaneInteractive)).Inc()
		slog.Warn("failed to persist budget", "error", err)
	}
	if !got.Found {
		return "", BodyGone, nil
	}

	body := SanitizeText(got.Body)
	if err := w.pool.Exec(ctx,
		`INSERT INTO issue_bodies (issue_id, body, source) VALUES ($1, $2, 'fetch')
		 ON CONFLICT (issue_id) DO UPDATE SET body = EXCLUDED.body, source = 'fetch', fetched_at = now(), last_read_at = now()`,
		issueID, body); err != nil {
		return "", "", fmt.Errorf("ingest: store body of issue %d: %w", issueID, err)
	}
	return body, BodyFetched, nil
}

// refreshCachedBodyTx replaces a cached body with the one an archive event
// carried, if the issue's body is cached at all. It never adds one: the cache
// holds what someone asked for.
func refreshCachedBodyTx(ctx context.Context, tx pgx.Tx, repoID, number int, body string) error {
	_, err := tx.Exec(ctx,
		`UPDATE issue_bodies b SET body = $3, source = 'archive', fetched_at = now()
		 FROM issues i
		 WHERE b.issue_id = i.id AND i.repository_id = $1 AND i.number = $2`,
		repoID, number, SanitizeText(body))
	return err
}

// PruneIssueBodies trims the cache back to maxBytes of bodies, deleting the
// least recently read first. Bodies of issues someone saved are never
// deleted, and count against maxBytes first.
func PruneIssueBodies(ctx context.Context, pool *db.Pool, maxBytes int64) (int, error) {
	var n int
	err := pool.QueryRow(ctx,
		`WITH kept AS (
		   SELECT COALESCE(sum(octet_length(b.body)), 0) AS bytes FROM issue_bodies b
		   WHERE EXISTS (SELECT 1 FROM saved_issues s WHERE s.issue_id = b.issue_id)
		 ), ranked AS (
		   SELECT b.issue_id,
		          sum(octet_length(b.body)) OVER (ORDER BY b.last_read_at DESC, b.issue_id DESC) AS running
		   FROM issue_bodies b
		   WHERE NOT EXISTS (SELECT 1 FROM saved_issues s WHERE s.issue_id = b.issue_id)
		 ), pruned AS (
		   DELETE FROM issue_bodies WHERE issue_id IN (
		     SELECT r.issue_id FROM ranked r, kept WHERE r.running + kept.bytes > $1)
		   RETURNING 1
		 )
		 SELECT count(*) FROM pruned`,
		[]any{maxBytes},
		func(row pgx.Row) error { return row.Scan(&n) })
	if err != nil {
		return 0, fmt.Errorf("ingest: prune issue bodies: %w", err)
	}
	return n, nil
}
