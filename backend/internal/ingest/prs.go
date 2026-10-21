package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/dizziedbliss/skillissues/internal/xp"
)

// Pull request tracking: each signed-up person's public pull requests to
// catalog repositories, read with the server's credential on the interactive
// lane, newest-updated first, and the XP their merges earn.
//
// A pass reads pages until it reaches what the last completed pass saw
// (pr_sync.synced_through) or the end; the first pass reads everything, which
// is the backfill. Each page commits with its cursor, so a pass resumes where
// it stopped. The watermark advances only when a pass completes, to the
// newest updatedAt it saw at its start (pass_high): anything updated after
// that is newer still, and the next pass reads it. XP is planned when a pass
// completes, over everything stored, so an award never depends on which page
// arrived first.

// prOverlap re-reads a little before the watermark: a pull request updated in
// the same second as the last one seen is not skipped. Re-reading is harmless.
const prOverlap = time.Minute

// PRSyncState is one person due a read.
type PRSyncState struct {
	UserID        int
	GitHubID      int64
	Login         string
	Joined        time.Time
	SyncedThrough *time.Time
	PassCursor    *string
	PassHigh      *time.Time
	Attempts      int
}

// EnsurePRSync adds a pr_sync row, due now, for everyone who has none.
func EnsurePRSync(ctx context.Context, pool *db.Pool) (int, error) {
	tag, err := pool.Raw().Exec(ctx,
		`INSERT INTO pr_sync (user_id)
		 SELECT u.id FROM users u WHERE NOT EXISTS (SELECT 1 FROM pr_sync s WHERE s.user_id = u.id)
		 ON CONFLICT (user_id) DO NOTHING`)
	if err != nil {
		return 0, fmt.Errorf("ingest: add pr_sync rows: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// DuePRSync returns up to limit people due a read, longest-waiting first.
// Someone with no username yet (never signed in fully) is skipped.
func DuePRSync(ctx context.Context, pool *db.Pool, limit int) ([]PRSyncState, error) {
	var out []PRSyncState
	err := pool.QueryRows(ctx,
		`SELECT s.user_id, u.github_id, u.username, u.created_at, s.synced_through, s.pass_cursor, s.pass_high, s.attempts
		 FROM pr_sync s JOIN users u ON u.id = s.user_id
		 WHERE s.next_check_at <= now() AND u.username IS NOT NULL AND u.username <> ''
		 ORDER BY s.next_check_at, s.user_id
		 LIMIT $1`, []any{limit},
		func(rows pgx.Rows) error {
			var st PRSyncState
			var ghID string
			if err := rows.Scan(&st.UserID, &ghID, &st.Login, &st.Joined, &st.SyncedThrough, &st.PassCursor, &st.PassHigh, &st.Attempts); err != nil {
				return err
			}
			id, err := strconv.ParseInt(ghID, 10, 64)
			if err != nil {
				return fmt.Errorf("user %d has a non-numeric github_id: %w", st.UserID, err)
			}
			st.GitHubID = id
			out = append(out, st)
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("ingest: select people due a pull request read: %w", err)
	}
	return out, nil
}

// PRIntervals are the pr_sync schedule, from settings.
type PRIntervals struct {
	Hot, Warm, Cold time.Duration
	HotWindow       time.Duration
	RetryBase       time.Duration
}

// PRIntervalsFrom reads the schedule from a settings snapshot.
func PRIntervalsFrom(s *settings.Snapshot) PRIntervals {
	return PRIntervals{
		Hot:       s.Seconds(settings.IngestPRsHotIntervalSeconds),
		Warm:      s.Seconds(settings.IngestPRsWarmIntervalSeconds),
		Cold:      s.Seconds(settings.IngestPRsColdIntervalSeconds),
		HotWindow: time.Duration(s.Int(settings.IngestPRsHotWindowDays)) * 24 * time.Hour,
		RetryBase: s.Seconds(settings.IngestPRsRetryBaseSeconds),
	}
}

// PRPageOutcome says what ApplyPullsPage did.
type PRPageOutcome struct {
	Stored   int            // pull requests to catalog repositories written
	Complete bool           // the pass reached its end
	Awards   map[string]int // ledger rows written by source (only when Complete)
	// OtherAccount: the login now belongs to someone else, or to no one.
	OtherAccount bool
}

// ApplyPullsPage writes one page of a person's pull requests and moves their
// pass on, in one transaction. now is the read's time.
func ApplyPullsPage(ctx context.Context, pool *db.Pool, st PRSyncState, page github.UserPullsPage,
	rules xp.Rules, iv PRIntervals, now time.Time) (PRPageOutcome, error) {
	var out PRPageOutcome
	if !page.Found || page.DatabaseID != st.GitHubID {
		// The login is someone else's now (or no one's): never store their
		// pull requests as this person's. Signing in again updates the login.
		out.OtherAccount = true
		err := pool.Exec(ctx,
			`UPDATE pr_sync SET next_check_at = $2, last_checked_at = $3, pass_cursor = NULL, pass_high = NULL,
			    last_error = 'the GitHub username no longer belongs to this account; signing in again updates it'
			 WHERE user_id = $1`, st.UserID, now.Add(iv.Cold), now)
		if err != nil {
			return out, fmt.Errorf("ingest: park pr_sync for user %d: %w", st.UserID, err)
		}
		return out, nil
	}

	// What this page adds to the pass: everything down to the watermark.
	pulls := page.Pulls
	reachedWatermark := false
	if st.SyncedThrough != nil {
		floor := st.SyncedThrough.Add(-prOverlap)
		for i, p := range pulls {
			if p.UpdatedAt.Before(floor) {
				pulls, reachedWatermark = pulls[:i], true
				break
			}
		}
	}
	high := st.PassHigh
	for _, p := range pulls {
		if high == nil || p.UpdatedAt.After(*high) {
			t := p.UpdatedAt
			high = &t
		}
	}
	complete := reachedWatermark || !page.HasNextPage

	err := pool.InTx(ctx, func(tx pgx.Tx) error {
		n, err := storePullsTx(ctx, tx, st, pulls, now)
		if err != nil {
			return err
		}
		out.Stored = n
		if !complete {
			_, err := tx.Exec(ctx,
				`UPDATE pr_sync SET pass_cursor = $2, pass_high = $3, next_check_at = $4, last_checked_at = $4,
				    attempts = 0, last_error = NULL
				 WHERE user_id = $1`, st.UserID, page.EndCursor, high, now)
			if err != nil {
				return fmt.Errorf("ingest: advance pr_sync for user %d: %w", st.UserID, err)
			}
			return nil
		}
		awards, err := awardTx(ctx, tx, st, rules)
		if err != nil {
			return err
		}
		out.Awards, out.Complete = awards, true
		through := st.SyncedThrough
		if high != nil {
			through = high
		}
		next, err := nextPRCheckTx(ctx, tx, st.UserID, iv, now)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx,
			`UPDATE pr_sync SET synced_through = $2, pass_cursor = NULL, pass_high = NULL, next_check_at = $3,
			    last_checked_at = $4, attempts = 0, last_error = NULL
			 WHERE user_id = $1`, st.UserID, through, next, now)
		if err != nil {
			return fmt.Errorf("ingest: complete pr_sync for user %d: %w", st.UserID, err)
		}
		return nil
	})
	return out, err
}

// FailPRSync records a failed read and backs off: retry_base doubled per
// attempt, never longer than the cold interval.
func FailPRSync(ctx context.Context, pool *db.Pool, st PRSyncState, iv PRIntervals, now time.Time, cause error) error {
	wait := iv.RetryBase
	for i := 0; i < st.Attempts && wait < iv.Cold; i++ {
		wait *= 2
	}
	wait = min(wait, iv.Cold)
	msg := cause.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	if err := pool.Exec(ctx,
		`UPDATE pr_sync SET attempts = attempts + 1, last_error = $2, next_check_at = $3, last_checked_at = $4 WHERE user_id = $1`,
		st.UserID, msg, now.Add(wait), now); err != nil {
		return fmt.Errorf("ingest: record pr_sync failure for user %d: %w", st.UserID, err)
	}
	return nil
}

// RequestPRSync makes a person due now. It is what "check my pull requests"
// does; go-api calls it.
func RequestPRSync(ctx context.Context, pool *db.Pool, userID int) error {
	return pool.Exec(ctx,
		`INSERT INTO pr_sync (user_id, next_check_at) VALUES ($1, now())
		 ON CONFLICT (user_id) DO UPDATE SET next_check_at = LEAST(pr_sync.next_check_at, now())`, userID)
}

// storePullsTx writes the pull requests to catalog repositories, and the
// catalog issues each says it closes.
func storePullsTx(ctx context.Context, tx pgx.Tx, st PRSyncState, pulls []github.UserPullRequest, now time.Time) (int, error) {
	if len(pulls) == 0 {
		return 0, nil
	}
	ghIDs := make([]int64, 0, len(pulls))
	names := make([]string, 0, len(pulls))
	for _, p := range pulls {
		ghIDs = append(ghIDs, p.RepositoryID)
		names = append(names, strings.ToLower(p.Repository))
		for _, c := range p.Closes {
			ghIDs = append(ghIDs, c.RepositoryID)
			names = append(names, strings.ToLower(c.Repository))
		}
	}
	// Catalog repositories by GitHub's id, or by name for a row that has no
	// id yet. Excluded and duplicate rows are not the catalog.
	byID, byName := map[int64]int{}, map[string]int{}
	rows, err := tx.Query(ctx,
		`SELECT id, github_database_id, lower(full_name) FROM repositories
		 WHERE (github_database_id = ANY($1) OR lower(full_name) = ANY($2))
		   AND eligibility_reason IS DISTINCT FROM 'excluded' AND eligibility_reason IS DISTINCT FROM 'duplicate'`,
		ghIDs, names)
	if err != nil {
		return 0, fmt.Errorf("ingest: resolve pull request repositories: %w", err)
	}
	for rows.Next() {
		var id int
		var gh *int64
		var name string
		if err := rows.Scan(&id, &gh, &name); err != nil {
			rows.Close()
			return 0, fmt.Errorf("ingest: scan pull request repository: %w", err)
		}
		if gh != nil {
			byID[*gh] = id
		}
		byName[name] = id
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("ingest: resolve pull request repositories: %w", err)
	}
	repoOf := func(ghID int64, name string) (int, bool) {
		if id, ok := byID[ghID]; ok {
			return id, true
		}
		id, ok := byName[strings.ToLower(name)]
		return id, ok
	}

	stored := 0
	for _, p := range pulls {
		repoID, ok := repoOf(p.RepositoryID, p.Repository)
		if !ok {
			continue // not a catalog repository: earns nothing, so not kept
		}
		var reason *string
		switch {
		case p.OwnerID != nil && *p.OwnerID == st.GitHubID, p.AuthorAssociation == "OWNER":
			reason = new("own_repository")
		case p.AuthorAssociation == "MEMBER":
			reason = new("organisation_member")
		}
		var prID int64
		err := tx.QueryRow(ctx,
			`INSERT INTO pull_requests (github_id, user_id, repository_id, number, title, url, state, created_at, updated_at,
			    merged_at, closed_at, author_association, additions, deletions, checks, approvals, changes_requested,
			    no_xp_reason, last_synced_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
			 ON CONFLICT (github_id) DO UPDATE SET
			    user_id = EXCLUDED.user_id, repository_id = EXCLUDED.repository_id, number = EXCLUDED.number,
			    title = EXCLUDED.title, url = EXCLUDED.url, state = EXCLUDED.state, updated_at = EXCLUDED.updated_at,
			    merged_at = EXCLUDED.merged_at, closed_at = EXCLUDED.closed_at,
			    author_association = EXCLUDED.author_association, additions = EXCLUDED.additions,
			    deletions = EXCLUDED.deletions, checks = EXCLUDED.checks, approvals = EXCLUDED.approvals,
			    changes_requested = EXCLUDED.changes_requested, no_xp_reason = EXCLUDED.no_xp_reason,
			    last_synced_at = EXCLUDED.last_synced_at
			 RETURNING id`,
			p.DatabaseID, st.UserID, repoID, p.Number, p.Title, p.URL, p.State, p.CreatedAt, p.UpdatedAt,
			p.MergedAt, p.ClosedAt, p.AuthorAssociation, p.Additions, p.Deletions, p.Checks, p.Approvals,
			p.ChangesRequested, reason, now,
		).Scan(&prID)
		if err != nil {
			return 0, fmt.Errorf("ingest: store pull request %s#%d: %w", p.Repository, p.Number, err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM pull_request_issues WHERE pull_request_id = $1`, prID); err != nil {
			return 0, fmt.Errorf("ingest: clear pull request links: %w", err)
		}
		for _, c := range p.Closes {
			issueRepo, ok := repoOf(c.RepositoryID, c.Repository)
			if !ok {
				continue
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO pull_request_issues (pull_request_id, issue_id)
				 SELECT $1, id FROM issues WHERE repository_id = $2 AND number = $3
				 ON CONFLICT DO NOTHING`, prID, issueRepo, c.Number); err != nil {
				return 0, fmt.Errorf("ingest: link pull request to %s#%d: %w", c.Repository, c.Number, err)
			}
		}
		stored++
	}
	return stored, nil
}

// awardTx plans the person's XP over everything stored and writes what is
// new. The ledger's unique keys make a repeat a no-op.
func awardTx(ctx context.Context, tx pgx.Tx, st PRSyncState, rules xp.Rules) (map[string]int, error) {
	prs, err := loadXPPullsTx(ctx, tx, st.UserID)
	if err != nil {
		return nil, err
	}
	paid, err := loadPaidTx(ctx, tx, st.UserID)
	if err != nil {
		return nil, err
	}
	written := map[string]int{}
	for _, a := range xp.Plan(rules, st.Joined, prs, paid) {
		detail, err := json.Marshal(a.Detail)
		if err != nil {
			return nil, fmt.Errorf("ingest: encode xp detail: %w", err)
		}
		tag, err := tx.Exec(ctx,
			`INSERT INTO xp_events (user_id, source, pull_request_id, amount, detail, earned_at)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (user_id, source, pull_request_id) WHERE pull_request_id IS NOT NULL DO NOTHING`,
			st.UserID, a.Source, a.PRID, a.Amount, detail, a.EarnedAt)
		if err != nil {
			return nil, fmt.Errorf("ingest: write xp for pull request %d: %w", a.PRID, err)
		}
		written[a.Source] += int(tag.RowsAffected())
	}
	return written, nil
}

func loadXPPullsTx(ctx context.Context, tx pgx.Tx, userID int) ([]xp.PR, error) {
	rows, err := tx.Query(ctx,
		`SELECT p.id, p.repository_id, COALESCE(r.language, ''), p.state, p.created_at, p.merged_at, p.checks,
		        p.approvals, p.changes_requested, p.no_xp_reason,
		        COALESCE(json_agg(json_build_object('id', i.id, 'difficulty', i.difficulty, 'created_at', i.created_at,
		                 'labels', COALESCE(i.labels, '{}'))) FILTER (WHERE i.id IS NOT NULL), '[]')
		 FROM pull_requests p
		 JOIN repositories r ON r.id = p.repository_id
		 LEFT JOIN pull_request_issues l ON l.pull_request_id = p.id
		 LEFT JOIN issues i ON i.id = l.issue_id
		 WHERE p.user_id = $1
		 GROUP BY p.id, r.language`, userID)
	if err != nil {
		return nil, fmt.Errorf("ingest: load pull requests for xp: %w", err)
	}
	defer rows.Close()
	var out []xp.PR
	for rows.Next() {
		var p xp.PR
		var issues []byte
		if err := rows.Scan(&p.ID, &p.RepositoryID, &p.Language, &p.State, &p.CreatedAt, &p.MergedAt, &p.Checks,
			&p.Approvals, &p.ChangesRequested, &p.NoXPReason, &issues); err != nil {
			return nil, fmt.Errorf("ingest: scan pull request for xp: %w", err)
		}
		var linked []struct {
			ID         int        `json:"id"`
			Difficulty string     `json:"difficulty"`
			CreatedAt  *time.Time `json:"created_at"`
			Labels     []string   `json:"labels"`
		}
		if err := json.Unmarshal(issues, &linked); err != nil {
			return nil, fmt.Errorf("ingest: decode linked issues: %w", err)
		}
		for _, l := range linked {
			p.Issues = append(p.Issues, xp.Issue{ID: l.ID, Difficulty: l.Difficulty, CreatedAt: l.CreatedAt, Labels: l.Labels})
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func loadPaidTx(ctx context.Context, tx pgx.Tx, userID int) ([]xp.Paid, error) {
	rows, err := tx.Query(ctx,
		`SELECT source, pull_request_id, amount, detail, earned_at FROM xp_events
		 WHERE user_id = $1 AND pull_request_id IS NOT NULL`, userID)
	if err != nil {
		return nil, fmt.Errorf("ingest: load paid xp: %w", err)
	}
	defer rows.Close()
	var out []xp.Paid
	for rows.Next() {
		var p xp.Paid
		var detail []byte
		if err := rows.Scan(&p.Source, &p.PRID, &p.Amount, &detail, &p.EarnedAt); err != nil {
			return nil, fmt.Errorf("ingest: scan paid xp: %w", err)
		}
		if err := json.Unmarshal(detail, &p.Detail); err != nil {
			return nil, fmt.Errorf("ingest: decode paid xp detail: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// nextPRCheckTx is when to read the person next: soon while one of their
// pull requests is open and moving, less often while one is open, daily
// otherwise.
func nextPRCheckTx(ctx context.Context, tx pgx.Tx, userID int, iv PRIntervals, now time.Time) (time.Time, error) {
	var newest *time.Time
	err := tx.QueryRow(ctx,
		`SELECT max(updated_at) FROM pull_requests WHERE user_id = $1 AND state = 'OPEN'`, userID).Scan(&newest)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, fmt.Errorf("ingest: read open pull requests: %w", err)
	}
	switch {
	case newest != nil && now.Sub(*newest) <= iv.HotWindow:
		return now.Add(iv.Hot), nil
	case newest != nil:
		return now.Add(iv.Warm), nil
	default:
		return now.Add(iv.Cold), nil
	}
}
