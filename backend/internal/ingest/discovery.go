package ingest

// The Discovery Sweeper finds repositories the dataset does not contain:
// new ones, ones that crossed the star floor since the snapshot, and ones the
// dataset never had. It runs GitHub repository Search with the eligibility
// thresholds (stars:>=min_stars fork:false archived:false) over pushed-date
// windows, and stores what passes Eligible. A new eligible row is picked up
// by EnqueueInitialJobs exactly like an imported one, so nothing downstream
// knows how a repository arrived.
//
// A sweep covers one pushed-date range, split into windows. GitHub search
// returns at most 1,000 results per query, so a window reporting more is
// replaced by its two halves, down to ingest.discovery.min_window_seconds.
// Windows are a leased queue like ingest_jobs: claimed with SKIP LOCKED,
// one page per claim, every write fenced on the lease, and a page's
// repository writes committed with its cursor.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/flow"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// SweepKind is a discovery_sweeps.kind.
type SweepKind string

const (
	// SweepFull covers the whole ingest.eligibility.pushed_within_days range.
	SweepFull SweepKind = "full"
	// SweepIncremental covers from the last completed sweep's pushed_to.
	SweepIncremental SweepKind = "incremental"
)

// ErrSweepRunning means a sweep is already running; only one runs at a time.
var ErrSweepRunning = errors.New("ingest: a discovery sweep is already running")

// SweepParams is what starting a sweep reads from settings.
type SweepParams struct {
	MinStars         int
	PushedWithinDays int
	Window           time.Duration
}

func sweepParams(snap *settings.Snapshot) SweepParams {
	return SweepParams{
		MinStars:         snap.Int(settings.IngestEligibilityMinStars),
		PushedWithinDays: snap.Int(settings.IngestEligibilityPushedWithinDays),
		Window:           snap.Hours(settings.IngestDiscoveryWindowHours),
	}
}

// StartSweep starts a sweep of the given kind covering [from, now), split
// into windows of p.Window. An incremental sweep with no completed sweep to
// start from becomes a full one. It returns ErrSweepRunning if one is
// already running.
func StartSweep(ctx context.Context, pool *db.Pool, kind SweepKind, p SweepParams) (int64, SweepKind, error) {
	if p.Window <= 0 {
		return 0, kind, fmt.Errorf("ingest: discovery window must be positive, got %s", p.Window)
	}
	var sweepID int64
	err := pool.InTx(ctx, func(tx pgx.Tx) error {
		var now time.Time
		if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
			return err
		}
		from := now.Add(-time.Duration(p.PushedWithinDays) * 24 * time.Hour)
		if kind == SweepIncremental {
			var watermark *time.Time
			if err := tx.QueryRow(ctx,
				`SELECT max(pushed_to) FROM discovery_sweeps WHERE status = 'done'`,
			).Scan(&watermark); err != nil {
				return fmt.Errorf("read discovery watermark: %w", err)
			}
			if watermark == nil {
				kind = SweepFull
			} else if watermark.After(from) {
				// Older than pushed_within_days is outside eligibility anyway.
				from = *watermark
			}
		}
		if !from.Before(now) {
			return fmt.Errorf("ingest: empty discovery range %s..%s", from, now)
		}

		err := tx.QueryRow(ctx,
			`INSERT INTO discovery_sweeps (kind, pushed_from, pushed_to, min_stars)
			 VALUES ($1, $2, $3, $4) RETURNING id`,
			string(kind), from, now, p.MinStars,
		).Scan(&sweepID)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrSweepRunning
		}
		if err != nil {
			return fmt.Errorf("insert discovery sweep: %w", err)
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO discovery_windows (sweep_id, pushed_from, pushed_to, next_retry_at)
			 SELECT $1, g, LEAST(g + ($4 * interval '1 second'), $3::timestamptz), now()
			 FROM generate_series($2::timestamptz, $3::timestamptz, $4 * interval '1 second') AS g
			 WHERE g < $3::timestamptz`,
			sweepID, from, now, p.Window.Seconds(),
		); err != nil {
			return fmt.Errorf("insert discovery windows: %w", err)
		}
		return nil
	})
	return sweepID, kind, err
}

// DueSweep reports which sweep, if any, should start now: none while one is
// running; a full one once full_interval has passed since the last full one
// started (or none ever did); otherwise an incremental one once
// incremental_interval has passed since any sweep started. A failed sweep
// counts as started, so a persistently failing range is retried on the
// incremental cadence rather than every cycle. An incrementalEvery of 0 runs
// no incremental sweep: the archive reader's candidates stand in for it.
func DueSweep(ctx context.Context, pool *db.Pool, fullEvery, incrementalEvery time.Duration) (SweepKind, bool, error) {
	var running bool
	var lastFull, lastAny *time.Time
	var now time.Time
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM discovery_sweeps WHERE status = 'running'),
		        (SELECT max(started_at) FROM discovery_sweeps WHERE kind = 'full'),
		        (SELECT max(started_at) FROM discovery_sweeps),
		        now()`, nil,
		func(row pgx.Row) error { return row.Scan(&running, &lastFull, &lastAny, &now) },
	); err != nil {
		return "", false, fmt.Errorf("ingest: read discovery schedule: %w", err)
	}
	switch {
	case running:
		return "", false, nil
	case lastFull == nil || now.Sub(*lastFull) >= fullEvery:
		return SweepFull, true, nil
	case incrementalEvery > 0 && (lastAny == nil || now.Sub(*lastAny) >= incrementalEvery):
		return SweepIncremental, true, nil
	}
	return "", false, nil
}

// RetryFailedSweep reopens the latest sweep when it ended 'failed' at least
// retryAfter ago and no sweep is running: its failed windows go back to
// pending with their attempts reset, and the sweep to running, so only the
// ranges that failed are searched again and a done one then advances the
// watermark. Without it a failed full sweep waited out the whole full
// interval, because the incremental sweep is off while the archive is
// current. It returns the reopened sweep's id, or 0 when there was none.
func RetryFailedSweep(ctx context.Context, pool *db.Pool, retryAfter time.Duration) (int64, error) {
	var sweepID int64
	err := pool.InTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx,
			`UPDATE discovery_sweeps SET status = 'running', completed_at = NULL
			 WHERE id = (SELECT max(id) FROM discovery_sweeps)
			   AND status = 'failed'
			   AND completed_at < now() - ($1 * interval '1 second')
			   AND NOT EXISTS (SELECT 1 FROM discovery_sweeps WHERE status = 'running')
			 RETURNING id`,
			retryAfter.Seconds(),
		).Scan(&sweepID)
		if errors.Is(err, pgx.ErrNoRows) {
			sweepID = 0
			return nil
		}
		if err != nil {
			return fmt.Errorf("reopen discovery sweep: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE discovery_windows SET status = 'pending', attempt_count = 0,
			    lease_owner = NULL, lease_expires_at = NULL, next_retry_at = now(), updated_at = now()
			 WHERE sweep_id = $1 AND status = 'failed'`,
			sweepID,
		); err != nil {
			return fmt.Errorf("reopen failed discovery windows: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("ingest: retry failed sweep: %w", err)
	}
	return sweepID, nil
}

// CompletedSweep is a sweep CompleteSweeps just closed.
type CompletedSweep struct {
	ID     int64
	Kind   SweepKind
	Status string
}

// CompleteSweeps closes every running sweep with no window left pending or
// running: 'done' if every window finished, 'failed' if any failed. Only a
// 'done' sweep advances the discovery watermark.
func CompleteSweeps(ctx context.Context, pool *db.Pool) ([]CompletedSweep, error) {
	var out []CompletedSweep
	err := pool.QueryRows(ctx,
		`UPDATE discovery_sweeps s
		 SET status = CASE WHEN EXISTS (
		       SELECT 1 FROM discovery_windows w WHERE w.sweep_id = s.id AND w.status = 'failed'
		     ) THEN 'failed' ELSE 'done' END,
		     completed_at = now()
		 WHERE s.status = 'running'
		   AND NOT EXISTS (
		       SELECT 1 FROM discovery_windows w
		       WHERE w.sweep_id = s.id AND w.status IN ('pending', 'running'))
		 RETURNING s.id, s.kind, s.status`, nil,
		func(rows pgx.Rows) error {
			var c CompletedSweep
			var kind string
			if err := rows.Scan(&c.ID, &kind, &c.Status); err != nil {
				return err
			}
			c.Kind = SweepKind(kind)
			out = append(out, c)
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("ingest: complete discovery sweeps: %w", err)
	}
	return out, nil
}

// DiscoveryWindow is one claimed row of discovery_windows.
type DiscoveryWindow struct {
	ID           int64
	SweepID      int64
	From, To     time.Time
	Cursor       *string
	PagesDone    int
	AttemptCount int
	MinStars     int
	LeaseOwner   string
}

// fenceWindow is the WHERE clause every window-finishing write shares.
const fenceWindow = `id = $1 AND status = 'running' AND lease_owner = $2`

// ClaimDiscoveryWindows claims up to limit due windows of running sweeps,
// FIFO on next_retry_at, with a lease.
func ClaimDiscoveryWindows(ctx context.Context, pool *db.Pool, limit int, owner string, lease time.Duration) ([]DiscoveryWindow, error) {
	if limit <= 0 {
		return nil, nil
	}
	var out []DiscoveryWindow
	err := pool.QueryRows(ctx,
		`UPDATE discovery_windows w
		 SET status = 'running', lease_owner = $1,
		     lease_expires_at = now() + ($2 * interval '1 second'), updated_at = now()
		 FROM (
			SELECT dw.id, s.min_stars FROM discovery_windows dw
			JOIN discovery_sweeps s ON s.id = dw.sweep_id
			WHERE dw.status = 'pending'
			  AND (dw.next_retry_at IS NULL OR dw.next_retry_at <= now())
			  AND s.status = 'running'
			ORDER BY dw.next_retry_at NULLS FIRST, dw.id
			LIMIT $3
			FOR UPDATE OF dw SKIP LOCKED
		 ) claimed
		 WHERE w.id = claimed.id
		 RETURNING w.id, w.sweep_id, w.pushed_from, w.pushed_to, w.cursor, w.pages_done,
		           w.attempt_count, claimed.min_stars`,
		[]any{owner, lease.Seconds(), limit},
		func(rows pgx.Rows) error {
			win := DiscoveryWindow{LeaseOwner: owner}
			if err := rows.Scan(&win.ID, &win.SweepID, &win.From, &win.To, &win.Cursor,
				&win.PagesDone, &win.AttemptCount, &win.MinStars); err != nil {
				return err
			}
			out = append(out, win)
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("ingest: claim discovery windows: %w", err)
	}
	return out, nil
}

// ReleaseDiscoveryWindow returns a claimed window to pending without counting
// an attempt — the worker never called GitHub.
func ReleaseDiscoveryWindow(ctx context.Context, pool *db.Pool, win DiscoveryWindow, retryAfter time.Duration) error {
	return execFenced(ctx, pool.Raw().Exec,
		`UPDATE discovery_windows SET status = 'pending', lease_owner = NULL, lease_expires_at = NULL,
		    next_retry_at = now() + ($3 * interval '1 second'), updated_at = now()
		 WHERE `+fenceWindow,
		win.ID, win.LeaseOwner, retryAfter.Seconds(),
	)
}

// FailDiscoveryWindow records an error and retries with exponential backoff,
// or leaves the window 'failed' once attempts reach maxAttempts. A failed
// window makes its sweep end 'failed', which does not advance the watermark.
func FailDiscoveryWindow(ctx context.Context, pool *db.Pool, win DiscoveryWindow, errClass, errDetail string, backoffBase time.Duration, maxAttempts int) error {
	attempt := win.AttemptCount + 1
	if attempt >= maxAttempts {
		return execFenced(ctx, pool.Raw().Exec,
			`UPDATE discovery_windows SET status = 'failed', attempt_count = $3,
			    last_error_class = $4, last_error_detail = $5,
			    lease_owner = NULL, lease_expires_at = NULL, updated_at = now()
			 WHERE `+fenceWindow,
			win.ID, win.LeaseOwner, attempt, errClass, errDetail,
		)
	}
	backoff := backoffBase * time.Duration(1<<uint(attempt))
	return execFenced(ctx, pool.Raw().Exec,
		`UPDATE discovery_windows SET status = 'pending', attempt_count = $3,
		    last_error_class = $4, last_error_detail = $5,
		    lease_owner = NULL, lease_expires_at = NULL,
		    next_retry_at = now() + ($6 * interval '1 second'), updated_at = now()
		 WHERE `+fenceWindow,
		win.ID, win.LeaseOwner, attempt, errClass, errDetail, backoff.Seconds(),
	)
}

// ReclaimExpiredDiscoveryLeases returns running windows whose lease lapsed to
// pending.
func ReclaimExpiredDiscoveryLeases(ctx context.Context, pool *db.Pool) (int, error) {
	tag, err := pool.Raw().Exec(ctx,
		`UPDATE discovery_windows SET status = 'pending', lease_owner = NULL, lease_expires_at = NULL
		 WHERE status = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at < now()`,
	)
	if err != nil {
		return 0, fmt.Errorf("ingest: reclaim expired discovery leases: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// splitWindowTx replaces a window GitHub reports too many matches for with
// its two halves, in one transaction: the range is never lost or doubled.
func splitWindowTx(ctx context.Context, tx pgx.Tx, win DiscoveryWindow, mid time.Time, count int) error {
	if err := execFenced(ctx, tx.Exec,
		`UPDATE discovery_windows SET status = 'done', split = true, repository_count = $3,
		    lease_owner = NULL, lease_expires_at = NULL, next_retry_at = NULL, updated_at = now()
		 WHERE `+fenceWindow,
		win.ID, win.LeaseOwner, count,
	); err != nil {
		return err
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO discovery_windows (sweep_id, pushed_from, pushed_to, next_retry_at)
		 VALUES ($1, $2, $3, now()), ($1, $3, $4, now())`,
		win.SweepID, win.From, mid, win.To,
	)
	if err != nil {
		return fmt.Errorf("ingest: insert split windows: %w", err)
	}
	return nil
}

// splitPoint is the midpoint a window is halved at, or false if it is already
// at the minimum span (or too short to halve on whole seconds).
func splitPoint(win DiscoveryWindow, minSpan time.Duration) (time.Time, bool) {
	span := win.To.Sub(win.From)
	if span <= minSpan {
		return time.Time{}, false
	}
	mid := win.From.Add(span / 2).Truncate(time.Second)
	if !mid.After(win.From) || !mid.Before(win.To) {
		return time.Time{}, false
	}
	return mid, true
}

// searchQuery is the GitHub search for a window. Range bounds are inclusive,
// so adjacent windows share one boundary second; upserts are idempotent.
func searchQuery(win DiscoveryWindow) string {
	const layout = "2006-01-02T15:04:05Z"
	return fmt.Sprintf("stars:>=%d fork:false archived:false pushed:%s..%s",
		win.MinStars, win.From.UTC().Format(layout), win.To.UTC().Format(layout))
}

// Outcomes of upserting one searched repository, the label values of
// discovery_repositories_total.
const (
	discoveredNew        = "new"
	discoveredUpdated    = "updated"
	discoveredRenamed    = "renamed"
	discoveredIneligible = "ineligible"
	discoveredExcluded   = "excluded"
	discoveredConflict   = "conflict"
)

// upsertDiscoveredTx records one searched repository. Rows are matched by
// GitHub id first, then by name:
//
//   - known by id under another name: renamed, unless another row holds the
//     new name — an id-less one is the duplicate twin and is marked so; one
//     with its own id is a different repository, and nothing is renamed.
//   - known by name with no id: the id is stored.
//   - known by name under another id: the name now belongs to a different
//     repository; reported, nothing changed.
//   - unknown: inserted, if Eligible and not excluded.
//
// A known row gets GitHub's live stats (as the crawl's first page writes
// them), and a 'not_found' flag is cleared — search just showed the
// repository exists — so the next eligibility recheck judges it again.
func upsertDiscoveredTx(ctx context.Context, tx pgx.Tx, r github.SearchedRepository, minStars, pushedWithinDays int) (outcome, conflict string, err error) {
	name := SanitizeText(r.NameWithOwner)
	stats := github.RepositoryStats{
		DatabaseID: r.DatabaseID, Stars: r.Stars, Forks: r.Forks, Watchers: r.Watchers,
		OpenIssueCount: r.OpenIssueCount, IsArchived: r.IsArchived, PushedAt: r.PushedAt,
	}

	var byID int
	var byIDName string
	err = tx.QueryRow(ctx,
		`SELECT id, full_name FROM repositories WHERE github_database_id = $1`, r.DatabaseID,
	).Scan(&byID, &byIDName)
	switch {
	case err == nil:
		outcome = discoveredUpdated
		if !strings.EqualFold(byIDName, name) {
			outcome, conflict, err = renameTx(ctx, tx, byID, byIDName, name)
			if err != nil {
				return "", "", err
			}
		}
		return outcome, conflict, refreshKnownTx(ctx, tx, byID, stats)
	case !errors.Is(err, pgx.ErrNoRows):
		return "", "", fmt.Errorf("ingest: find repository by id: %w", err)
	}

	var byName int
	var byNameID *int64
	err = tx.QueryRow(ctx,
		`SELECT id, github_database_id FROM repositories WHERE lower(full_name) = lower($1)`, name,
	).Scan(&byName, &byNameID)
	switch {
	case err == nil:
		if byNameID != nil {
			slog.Warn("repository name now resolves to a different GitHub repository; row left as it is",
				"repository_id", byName, "stored_github_id", *byNameID, "found_github_id", r.DatabaseID, "repo", name)
			return discoveredConflict, ConflictIDMismatch, nil
		}
		if _, err := tx.Exec(ctx,
			`UPDATE repositories SET github_database_id = $2 WHERE id = $1`, byName, r.DatabaseID,
		); err != nil {
			return "", "", fmt.Errorf("ingest: store repository id: %w", err)
		}
		return discoveredUpdated, "", refreshKnownTx(ctx, tx, byName, stats)
	case !errors.Is(err, pgx.ErrNoRows):
		return "", "", fmt.Errorf("ingest: find repository by name: %w", err)
	}

	var excluded bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM excluded_repositories WHERE lower(full_name) = lower($1))`, name,
	).Scan(&excluded); err != nil {
		return "", "", fmt.Errorf("ingest: check excluded repositories: %w", err)
	}
	if excluded {
		return discoveredExcluded, "", nil
	}
	language := SanitizeOptional(r.PrimaryLanguage)
	if ok, _ := Eligible(RepoFacts{
		IsFork: r.IsFork, IsArchived: r.IsArchived, IssueCount: r.IssueCount,
		PrimaryLanguage: language, PushedAt: r.PushedAt, Stars: r.Stars,
	}, minStars, pushedWithinDays); !ok {
		// Like the importer, an ineligible repository is not stored: the next
		// sweep sees it again if it ever qualifies.
		return discoveredIneligible, "", nil
	}
	return discoveredNew, "", insertDiscoveredTx(ctx, tx, name, language, r)
}

// renameTx moves a row known by GitHub id to the name GitHub reports now.
func renameTx(ctx context.Context, tx pgx.Tx, repoID int, oldName, newName string) (outcome, conflict string, err error) {
	var holder int
	var holderID *int64
	err = tx.QueryRow(ctx,
		`SELECT id, github_database_id FROM repositories WHERE lower(full_name) = lower($1)`, newName,
	).Scan(&holder, &holderID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if _, err := tx.Exec(ctx,
			`UPDATE repositories SET full_name = $2, url = $3 WHERE id = $1`,
			repoID, newName, "https://github.com/"+newName,
		); err != nil {
			return "", "", fmt.Errorf("ingest: rename repository: %w", err)
		}
		slog.Info("repository renamed on GitHub", "repository_id", repoID, "from", oldName, "to", newName)
		return discoveredRenamed, "", nil
	case err != nil:
		return "", "", fmt.Errorf("ingest: find rename target: %w", err)
	}

	if holderID != nil {
		// The name belongs to another repository's row. Keep both as they are.
		slog.Warn("repository renamed onto a name another repository's row holds; not renamed",
			"repository_id", repoID, "from", oldName, "to", newName, "holder_repository_id", holder)
		return discoveredConflict, ConflictRenameBlocked, nil
	}
	// An id-less row under the new name is the same repository imported
	// again after the rename. The row with the id wins, under its old name
	// (GitHub redirects it); the twin stops being crawled. Nothing is deleted.
	marked, err := markDuplicateTx(ctx, tx, holder)
	if err != nil {
		return "", "", err
	}
	if !marked {
		return discoveredUpdated, "", nil
	}
	slog.Warn("id-less row under a renamed repository's new name marked duplicate",
		"repository_id", repoID, "name", oldName, "duplicate_repository_id", holder, "duplicate_name", newName)
	return discoveredConflict, ConflictDuplicate, nil
}

// refreshKnownTx writes live stats over a known row and clears 'not_found'.
func refreshKnownTx(ctx context.Context, tx pgx.Tx, repoID int, stats github.RepositoryStats) error {
	if err := UpdateRepositoryStatsTx(ctx, tx, repoID, stats); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE repositories SET eligibility_reason = NULL
		 WHERE id = $1 AND eligibility_reason = 'not_found'`, repoID,
	); err != nil {
		return fmt.Errorf("ingest: clear not_found: %w", err)
	}
	return nil
}

// insertDiscoveredTx stores a new eligible repository with the columns
// cmd/repoimport fills, plus its GitHub id and live open-issue count.
func insertDiscoveredTx(ctx context.Context, tx pgx.Tx, name string, language *string, r github.SearchedRepository) error {
	langs := make([]github.RepositoryLanguage, len(r.Languages))
	for i, l := range r.Languages {
		langs[i] = github.RepositoryLanguage{Name: SanitizeText(l.Name), Size: l.Size}
	}
	languagesJSON, err := json.Marshal(langs)
	if err != nil {
		return fmt.Errorf("ingest: marshal languages: %w", err)
	}
	// ON CONFLICT: another worker inserted the same repository between the
	// lookups above and here. Its row is as good as this one.
	_, err = tx.Exec(ctx,
		`INSERT INTO repositories (
			full_name, url, description, language, stars, license,
			forks, watchers, is_archived, created_at, pushed_at,
			topics, languages, forking_allowed,
			issue_count, open_issue_count, github_database_id, metadata_refreshed_at,
			is_eligible, eligibility_reason
		 ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17, now(), true, '')
		 ON CONFLICT DO NOTHING`,
		name, "https://github.com/"+name, SanitizeOptional(r.Description), language, r.Stars,
		SanitizeOptional(r.License), r.Forks, r.Watchers, r.IsArchived, r.CreatedAt, r.PushedAt,
		SanitizeSlice(r.Topics), languagesJSON, r.ForkingAllowed,
		r.IssueCount, r.OpenIssueCount, r.DatabaseID,
	)
	if err != nil {
		return fmt.Errorf("ingest: insert discovered repository: %w", err)
	}
	return nil
}

// ProcessDiscoveryWindow fetches one search page for a claimed window and
// persists it atomically.
func (w *Worker) ProcessDiscoveryWindow(ctx context.Context, win DiscoveryWindow) error {
	outcome, err := w.processDiscoveryWindow(ctx, win)
	if errors.Is(err, ErrLeaseLost) {
		outcome = "lease_lost"
	}
	metrics.DiscoveryWindowOutcomes.WithLabelValues(outcome).Inc()
	return err
}

func (w *Worker) processDiscoveryWindow(ctx context.Context, win DiscoveryWindow) (string, error) {
	token := w.token()
	if token == "" {
		return "no_token", fmt.Errorf("no github token configured")
	}
	snap := w.live()

	proceed, err := CanProceed(ctx, w.pool, LaneCrawler, installationIdentity,
		snap.Int(settings.IngestDiscoveryRateLimitFloor), snap.Int(settings.IngestInteractiveReservePercent))
	if err != nil {
		metrics.BudgetErrors.WithLabelValues("read", string(LaneCrawler)).Inc()
		slog.Warn("budget check failed, searching anyway", "error", err)
	} else if !proceed {
		if err := ReleaseDiscoveryWindow(ctx, w.pool, win, snap.Seconds(settings.IngestRateLimitPauseSeconds)); err != nil {
			return "persist_error", fmt.Errorf("release discovery window: %w", err)
		}
		return "budget_deferred", nil
	}

	query := searchQuery(win)
	page, err := w.searchWithBatchLadder(ctx, token, query, snap.Int(settings.IngestDiscoveryPageSize), win.Cursor)
	if err != nil {
		return w.handleDiscoveryError(ctx, win, err)
	}
	if err := UpdateBudget(ctx, w.pool, LaneCrawler, installationIdentity, page.RateLimit); err != nil {
		metrics.BudgetErrors.WithLabelValues("write", string(LaneCrawler)).Inc()
		slog.Warn("failed to persist budget", "error", err)
	}

	tx, err := w.pool.Raw().Begin(ctx)
	if err != nil {
		return "persist_error", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op if already committed

	firstPage := win.Cursor == nil || *win.Cursor == ""
	truncated := false
	if firstPage && page.RepositoryCount > github.SearchResultCap {
		if mid, ok := splitPoint(win, snap.Seconds(settings.IngestDiscoveryMinWindowSeconds)); ok {
			if err := splitWindowTx(ctx, tx, win, mid, page.RepositoryCount); err != nil {
				return "persist_error", fmt.Errorf("split window: %w", err)
			}
			if err := tx.Commit(ctx); err != nil {
				return "persist_error", fmt.Errorf("commit split: %w", err)
			}
			slog.Info("discovery window too dense, split",
				"window_id", win.ID, "from", win.From, "to", win.To, "matches", page.RepositoryCount)
			return "split", nil
		}
		truncated = true
	}

	minStars := snap.Int(settings.IngestEligibilityMinStars)
	pushedWithinDays := snap.Int(settings.IngestEligibilityPushedWithinDays)
	outcomes := map[string]int{}
	conflicts := map[string]int{}
	firstNew := ""
	for _, r := range page.Repositories {
		if r.DatabaseID == 0 {
			continue
		}
		outcome, conflict, err := upsertDiscoveredTx(ctx, tx, r, minStars, pushedWithinDays)
		if err != nil {
			return "persist_error", fmt.Errorf("upsert %s: %w", r.NameWithOwner, err)
		}
		if outcome == discoveredNew && firstNew == "" {
			firstNew = r.NameWithOwner
		}
		outcomes[outcome]++
		if conflict != "" {
			conflicts[conflict]++
		}
	}

	// GitHub stops paging at the 1,000-result cap, so hasNextPage ends a
	// truncated window too.
	var countArg *int
	if firstPage {
		countArg = &page.RepositoryCount
	}
	status, cursor := "done", (*string)(nil)
	if page.HasNextPage {
		status, cursor = "pending", &page.EndCursor
	}
	if err := execFenced(ctx, tx.Exec,
		`UPDATE discovery_windows SET status = $3, cursor = coalesce($4, cursor),
		    pages_done = pages_done + 1, repos_seen = repos_seen + $5,
		    repository_count = coalesce($6, repository_count), truncated = truncated OR $7,
		    attempt_count = 0, lease_owner = NULL, lease_expires_at = NULL,
		    next_retry_at = CASE WHEN $3 = 'pending' THEN now() END, updated_at = now()
		 WHERE `+fenceWindow,
		win.ID, win.LeaseOwner, status, cursor, len(page.Repositories), countArg, truncated,
	); err != nil {
		return "persist_error", fmt.Errorf("advance window: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "persist_error", fmt.Errorf("commit page: %w", err)
	}

	for o, n := range outcomes {
		metrics.DiscoveryRepositories.WithLabelValues(o).Add(float64(n))
	}
	for c, n := range conflicts {
		metrics.RepositoryIdentityConflicts.WithLabelValues(c).Add(float64(n))
	}
	if truncated {
		metrics.DiscoveryWindowsTruncated.Inc()
		slog.Warn("discovery window at the minimum span still exceeds the search cap; reading only the first results",
			"window_id", win.ID, "from", win.From, "to", win.To, "matches", page.RepositoryCount, "cap", github.SearchResultCap)
	}
	flow.Emit(ctx, flow.Event{
		From: flow.GitHub, To: flow.Ingestd, Kind: flow.KindDiscovery,
		Label:   fmt.Sprintf("discovery %d repos +%d new", len(page.Repositories), outcomes[discoveredNew]),
		Payload: firstNew,
	})
	slog.Info("discovery page ingested",
		"window_id", win.ID, "sweep_id", win.SweepID, "repositories", len(page.Repositories),
		"new", outcomes[discoveredNew], "has_next_page", page.HasNextPage, "remaining_budget", page.RateLimit.Remaining)
	return "page", nil
}

// searchWithBatchLadder halves the page size on RESOURCE_LIMITS_EXCEEDED or a
// timeout, down to 1, like the crawl's fetchWithBatchLadder.
func (w *Worker) searchWithBatchLadder(ctx context.Context, token, query string, pageSize int, cursor *string) (*github.RepositorySearchPage, error) {
	for {
		page, err := w.github.SearchRepositories(ctx, token, query, pageSize, cursor)
		if err == nil {
			return page, nil
		}
		if !isResourceLimitError(err) || pageSize <= 1 {
			return nil, err
		}
		pageSize = max(pageSize/2, 1)
		slog.Warn("resource limit hit on search, shrinking page size and retrying",
			"query", query, "new_page_size", pageSize, "error", err)
	}
}

// handleDiscoveryError classifies a search failure: rate limits pause the
// shared crawler lane; everything else backs off and, past
// ingest.max_attempts, fails the window.
func (w *Worker) handleDiscoveryError(ctx context.Context, win DiscoveryWindow, err error) (string, error) {
	msg := err.Error()
	snap := w.live()
	backoff, attempts := snap.Seconds(settings.IngestBackoffBaseSeconds), snap.Int(settings.IngestMaxAttempts)

	if limited, retryAfter := github.IsRateLimited(err); limited {
		until := time.Now().Add(rateLimitPause(snap, retryAfter))
		if err := MarkSecondaryLimit(ctx, w.pool, LaneCrawler, installationIdentity, until); err != nil {
			metrics.BudgetErrors.WithLabelValues("write", string(LaneCrawler)).Inc()
			slog.Warn("failed to record secondary rate limit pause", "error", err)
		}
		if err := FailDiscoveryWindow(ctx, w.pool, win, "rate_limit", msg, backoff, attempts); err != nil {
			return "persist_error", fmt.Errorf("fail discovery window: %w", err)
		}
		return "rate_limited", nil
	}

	slog.Warn("discovery search failed", "window_id", win.ID, "attempt", win.AttemptCount+1, "error", err)
	if err := FailDiscoveryWindow(ctx, w.pool, win, "fetch_error", msg, backoff, attempts); err != nil {
		return "persist_error", fmt.Errorf("fail discovery window: %w", err)
	}
	return "fetch_error", nil
}
