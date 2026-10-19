package ingest

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/flow"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/httpx"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// The GH Archive reader.
//
// GH Archive (https://www.gharchive.org) publishes every public GitHub event
// as one gzipped JSON-lines file per hour, about an hour after it ends.
// IssuesEvent and IssueCommentEvent carry the whole issue as it stood after
// the event — labels, state, updated_at — so applying them keeps stored
// issues fresh without a GitHub API call, and the per-repository freshness
// crawl only has to catch what events cannot show (a deleted or transferred
// issue).
//
// Hours are a leased, fenced queue (archive_hours), like ingest_jobs. An hour
// is read as a stream, never held whole; of its events only issue activity on
// stored repositories is applied, through the same write path as the crawl
// (writeIssueTx), and only when newer than what is stored, so hours can be
// read in any order and read again. Issue activity on repositories that are
// not stored makes them candidates (repo_candidates), looked up on GitHub and
// stored if they are eligible: the archive is also how new repositories are
// found.

// DefaultArchiveURL is where GH Archive's hourly files are published.
const DefaultArchiveURL = "https://data.gharchive.org"

// ArchiveHour is one claimed archive hour.
type ArchiveHour struct {
	Hour       time.Time
	Attempts   int
	LeaseOwner string
}

// ErrArchiveNotPublished is returned for an hour whose file does not exist
// (yet).
var ErrArchiveNotPublished = errors.New("ingest: archive hour not published")

// SeedArchiveHours queues every hour that has ended since the newest queued
// one, and returns how many it added. On the first run it starts at the oldest
// completed crawl pass of an eligible repository, so nothing a pass did not
// see is skipped, but no earlier than maxBackfill ago.
func SeedArchiveHours(ctx context.Context, pool *db.Pool, maxBackfill time.Duration) (int, error) {
	var n int
	err := pool.QueryRow(ctx,
		`WITH bounds AS (
		   SELECT COALESCE(
		            (SELECT max(hour) + interval '1 hour' FROM archive_hours),
		            GREATEST(
		              date_trunc('hour', COALESCE(
		                (SELECT min(issues_synced_through) FROM repositories WHERE is_eligible),
		                now() - interval '1 hour')),
		              date_trunc('hour', now()) - $1::interval)) AS first,
		          date_trunc('hour', now()) - interval '1 hour' AS last
		 ), added AS (
		   INSERT INTO archive_hours (hour)
		   SELECT generate_series(first, last, interval '1 hour') FROM bounds
		   ON CONFLICT (hour) DO NOTHING
		   RETURNING 1
		 )
		 SELECT count(*) FROM added`,
		[]any{maxBackfill},
		func(row pgx.Row) error { return row.Scan(&n) })
	if err != nil {
		return 0, fmt.Errorf("ingest: seed archive hours: %w", err)
	}
	return n, nil
}

// ClaimArchiveHours leases up to limit due hours, oldest first.
func ClaimArchiveHours(ctx context.Context, pool *db.Pool, limit int, owner string, lease time.Duration) ([]ArchiveHour, error) {
	var out []ArchiveHour
	err := pool.QueryRows(ctx,
		`UPDATE archive_hours h SET status = 'running', lease_owner = $2, lease_expires_at = now() + $3::interval
		 FROM (
		   SELECT hour FROM archive_hours
		   WHERE status = 'pending' AND next_retry_at <= now()
		   ORDER BY hour
		   LIMIT $1
		   FOR UPDATE SKIP LOCKED
		 ) due
		 WHERE h.hour = due.hour
		 RETURNING h.hour, h.attempts, h.lease_owner`,
		[]any{limit, owner, lease},
		func(rows pgx.Rows) error {
			var h ArchiveHour
			if err := rows.Scan(&h.Hour, &h.Attempts, &h.LeaseOwner); err != nil {
				return err
			}
			out = append(out, h)
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("ingest: claim archive hours: %w", err)
	}
	return out, nil
}

// ReclaimExpiredArchiveLeases returns hours whose lease expired to pending.
func ReclaimExpiredArchiveLeases(ctx context.Context, pool *db.Pool) (int, error) {
	var n int
	err := pool.QueryRow(ctx,
		`WITH reclaimed AS (
		   UPDATE archive_hours SET status = 'pending', lease_owner = NULL, lease_expires_at = NULL
		   WHERE status = 'running' AND lease_expires_at < now()
		   RETURNING 1
		 ) SELECT count(*) FROM reclaimed`, nil,
		func(row pgx.Row) error { return row.Scan(&n) })
	if err != nil {
		return 0, fmt.Errorf("ingest: reclaim archive leases: %w", err)
	}
	return n, nil
}

// finishArchiveHour runs one fenced write that ends the hour's lease. Zero
// rows means the lease was lost: the hour was reclaimed while this worker
// held it.
func finishArchiveHour(ctx context.Context, pool *db.Pool, h ArchiveHour, set string, args ...any) error {
	var n int
	err := pool.QueryRow(ctx,
		`WITH finished AS (
		   UPDATE archive_hours SET `+set+`, lease_owner = NULL, lease_expires_at = NULL
		   WHERE hour = $1 AND status = 'running' AND lease_owner = $2
		   RETURNING 1
		 ) SELECT count(*) FROM finished`,
		append([]any{h.Hour, h.LeaseOwner}, args...),
		func(row pgx.Row) error { return row.Scan(&n) })
	if err != nil {
		return fmt.Errorf("ingest: finish archive hour: %w", err)
	}
	if n == 0 {
		return ErrLeaseLost
	}
	return nil
}

func completeArchiveHour(ctx context.Context, pool *db.Pool, h ArchiveHour, r archiveHourResult) error {
	return finishArchiveHour(ctx, pool, h,
		`status = 'done', finished_at = now(), last_error = NULL,
		 events_read = $3, issues_applied = $4, candidates_found = $5`,
		r.eventsRead, r.applied, r.candidates)
}

// releaseArchiveHour puts an unpublished hour back without counting an attempt.
func releaseArchiveHour(ctx context.Context, pool *db.Pool, h ArchiveHour, retryAfter time.Duration) error {
	return finishArchiveHour(ctx, pool, h,
		`status = 'pending', next_retry_at = now() + $3::interval`, retryAfter)
}

func markArchiveHourMissing(ctx context.Context, pool *db.Pool, h ArchiveHour) error {
	return finishArchiveHour(ctx, pool, h,
		`status = 'missing', finished_at = now(), last_error = 'not published'`)
}

// failArchiveHour counts a processing failure: retried after retryAfter, and
// failed for good after maxAttempts. A failed hour holds archive coverage
// back (ArchiveCoveredUntil) until someone sets it pending again, so the
// freshness crawl takes over rather than its issues silently going stale.
func failArchiveHour(ctx context.Context, pool *db.Pool, h ArchiveHour, cause error, retryAfter time.Duration, maxAttempts int) error {
	return finishArchiveHour(ctx, pool, h,
		`attempts = attempts + 1, last_error = $3,
		 status = CASE WHEN attempts + 1 >= $5 THEN 'failed' ELSE 'pending' END,
		 finished_at = CASE WHEN attempts + 1 >= $5 THEN now() END,
		 next_retry_at = now() + $4::interval`,
		SanitizeText(cause.Error()), retryAfter, maxAttempts)
}

// ArchiveCoveredUntil returns the time up to which the archive has been
// applied completely: every hour before it is done or missing. ok is false
// when no hour has been queued yet.
func ArchiveCoveredUntil(ctx context.Context, pool *db.Pool) (until time.Time, ok bool, err error) {
	var t *time.Time
	err = pool.QueryRow(ctx,
		`SELECT COALESCE(
		   (SELECT min(hour) FROM archive_hours WHERE status IN ('pending', 'running', 'failed')),
		   (SELECT max(hour) + interval '1 hour' FROM archive_hours))`, nil,
		func(row pgx.Row) error { return row.Scan(&t) })
	if err != nil {
		return time.Time{}, false, fmt.Errorf("ingest: read archive coverage: %w", err)
	}
	if t == nil {
		return time.Time{}, false, nil
	}
	return *t, true, nil
}

// archiveSource downloads hour files.
type archiveSource struct {
	baseURL string
	client  httpx.Doer
}

func archiveFileURL(base string, hour time.Time) string {
	h := hour.UTC()
	// The hour is not zero-padded: 2026-10-18-5.json.gz.
	return strings.TrimRight(base, "/") + "/" + h.Format("2006-01-02") + "-" + strconv.Itoa(h.Hour()) + ".json.gz"
}

// open returns the hour's file body, limited to maxBytes compressed.
func (s archiveSource) open(ctx context.Context, hour time.Time, maxBytes int64) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(metrics.WithOperation(ctx, "ArchiveHour"), http.MethodGet, archiveFileURL(s.baseURL, hour), nil)
	if err != nil {
		return nil, fmt.Errorf("ingest: archive request: %w", err)
	}
	req.Header.Set("User-Agent", "SkillIssues-Go/2.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ingest: download archive hour: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		resp.Body.Close()
		return nil, ErrArchiveNotPublished
	case resp.StatusCode != http.StatusOK:
		resp.Body.Close()
		return nil, fmt.Errorf("ingest: download archive hour: HTTP %d", resp.StatusCode)
	case resp.ContentLength > maxBytes:
		resp.Body.Close()
		return nil, fmt.Errorf("ingest: archive hour is %d bytes, over ingest.archive.max_file_mb", resp.ContentLength)
	}
	return &limitedBody{r: io.LimitReader(resp.Body, maxBytes+1), c: resp.Body, max: maxBytes}, nil
}

// limitedBody fails the read, rather than truncating it, once the body runs
// past max bytes: a truncated hour must not be recorded as done.
type limitedBody struct {
	r    io.Reader
	c    io.Closer
	max  int64
	read int64
}

func (b *limitedBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.read += int64(n)
	if b.read > b.max {
		return n, fmt.Errorf("ingest: archive hour is over ingest.archive.max_file_mb")
	}
	return n, err
}

func (b *limitedBody) Close() error { return b.c.Close() }

// archiveEvent is the part of an archive line the reader looks at; the
// payload is decoded only for issue events.
type archiveEvent struct {
	Type string `json:"type"`
	Repo struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"repo"`
	Payload json.RawMessage `json:"payload"`
}

type archiveIssuePayload struct {
	Issue *struct {
		ID        int64     `json:"id"`
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		HTMLURL   string    `json:"html_url"`
		State     string    `json:"state"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Comments  int       `json:"comments"`
		User      *struct {
			Login string `json:"login"`
		} `json:"user"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		// Present only when the "issue" is a pull request, which GitHub's
		// issue events also report.
		PullRequest json.RawMessage `json:"pull_request"`
	} `json:"issue"`
}

// archiveIssue is the newest state of one issue seen in an hour.
type archiveIssue struct {
	repoID   int64
	repoName string
	node     github.GraphQLIssueNode
}

type archiveIssueKey struct {
	repoID int64
	number int
}

// readArchiveHour streams one hour's file and returns the newest state of
// every issue it has issue events for, and how many events it read.
func readArchiveHour(r io.Reader) (map[archiveIssueKey]archiveIssue, int, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, 0, fmt.Errorf("ingest: archive hour is not gzip: %w", err)
	}
	defer gz.Close()

	issues := make(map[archiveIssueKey]archiveIssue)
	dec := json.NewDecoder(gz)
	events := 0
	for {
		var ev archiveEvent
		err := dec.Decode(&ev)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, events, fmt.Errorf("ingest: archive event %d: %w", events+1, err)
		}
		events++
		if ev.Type != "IssuesEvent" && ev.Type != "IssueCommentEvent" {
			continue
		}
		if ev.Repo.ID == 0 {
			continue
		}
		var p archiveIssuePayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return nil, events, fmt.Errorf("ingest: archive %s payload in %s: %w", ev.Type, ev.Repo.Name, err)
		}
		it := p.Issue
		if it == nil || it.Number == 0 || it.UpdatedAt.IsZero() || len(it.PullRequest) > 0 && string(it.PullRequest) != "null" {
			continue // not an issue, or not one that can be placed in time
		}
		state := strings.ToUpper(it.State)
		if state != "OPEN" && state != "CLOSED" {
			continue
		}
		key := archiveIssueKey{ev.Repo.ID, it.Number}
		if seen, ok := issues[key]; ok && !it.UpdatedAt.After(seen.node.UpdatedAt) {
			continue
		}
		node := github.GraphQLIssueNode{
			DatabaseID:    it.ID,
			Number:        it.Number,
			Title:         it.Title,
			URL:           it.HTMLURL,
			State:         state,
			CreatedAt:     it.CreatedAt,
			UpdatedAt:     it.UpdatedAt,
			CommentsCount: it.Comments,
		}
		if it.User != nil {
			node.AuthorLogin = it.User.Login
		}
		for _, l := range it.Labels {
			node.Labels = append(node.Labels, l.Name)
		}
		issues[key] = archiveIssue{repoID: ev.Repo.ID, repoName: ev.Repo.Name, node: node}
	}
	return issues, events, nil
}

// archiveRepo is a stored repository as the reader matches events to it.
type archiveRepo struct {
	id       int
	language string
	eligible bool
}

// archiveRepoIndex finds the stored repository an event's repo belongs to: by
// GitHub id, which survives a rename, or by name for a row whose id the crawl
// has not recorded yet.
type archiveRepoIndex struct {
	byGitHubID map[int64]archiveRepo
	byName     map[string]archiveRepo
}

// loadArchiveRepoIndex reads the stored repositories an hour's issues name,
// through the unique indexes on github_database_id and lower(full_name).
func loadArchiveRepoIndex(ctx context.Context, pool *db.Pool, issues map[archiveIssueKey]archiveIssue) (archiveRepoIndex, error) {
	idx := archiveRepoIndex{byGitHubID: make(map[int64]archiveRepo), byName: make(map[string]archiveRepo)}
	seen := make(map[int64]bool)
	var ids []int64
	var names []string
	for _, it := range issues {
		if !seen[it.repoID] {
			seen[it.repoID] = true
			ids = append(ids, it.repoID)
			names = append(names, strings.ToLower(it.repoName))
		}
	}
	if len(ids) == 0 {
		return idx, nil
	}
	err := pool.QueryRows(ctx,
		`SELECT id, github_database_id, lower(full_name), COALESCE(language, ''), is_eligible FROM repositories
		 WHERE github_database_id = ANY($1)
		    OR (github_database_id IS NULL AND lower(full_name) = ANY($2))`,
		[]any{ids, names},
		func(rows pgx.Rows) error {
			var r archiveRepo
			var ghID *int64
			var name string
			if err := rows.Scan(&r.id, &ghID, &name, &r.language, &r.eligible); err != nil {
				return err
			}
			if ghID != nil {
				idx.byGitHubID[*ghID] = r
			} else {
				idx.byName[name] = r
			}
			return nil
		})
	if err != nil {
		return archiveRepoIndex{}, fmt.Errorf("ingest: load repositories for the archive: %w", err)
	}
	return idx, nil
}

func (idx archiveRepoIndex) find(ghID int64, name string) (archiveRepo, bool) {
	if r, ok := idx.byGitHubID[ghID]; ok {
		return r, true
	}
	r, ok := idx.byName[strings.ToLower(name)]
	return r, ok
}

type archiveHourResult struct {
	eventsRead int
	applied    int
	candidates int
	firstTitle string
}

// applyArchiveIssues writes an hour's issues to the stored repositories they
// belong to, batchSize per transaction, and queues the repositories that are
// not stored as candidates. With updateOnly (the disk guard is on) nothing is
// inserted, only stored issues updated.
func applyArchiveIssues(ctx context.Context, pool *db.Pool, hour time.Time, issues map[archiveIssueKey]archiveIssue, idx archiveRepoIndex, batchSize int, updateOnly bool) (archiveHourResult, error) {
	var res archiveHourResult
	results := map[string]int{}

	type write struct {
		repo archiveRepo
		node github.GraphQLIssueNode
	}
	var writes []write
	candidates := map[int64]string{}
	for _, it := range issues {
		repo, ok := idx.find(it.repoID, it.repoName)
		switch {
		case !ok:
			candidates[it.repoID] = it.repoName
			results["candidate"]++
		case !repo.eligible:
			results["ineligible_repo"]++
		default:
			writes = append(writes, write{repo, it.node})
		}
	}

	for start := 0; start < len(writes); start += batchSize {
		batch := writes[start:min(start+batchSize, len(writes))]
		err := pool.InTx(ctx, func(tx pgx.Tx) error {
			for _, w := range batch {
				mode := writeIfNewer
				if updateOnly || w.node.State == "CLOSED" {
					mode = updateIfNewer
				}
				applied, err := writeIssueTx(ctx, tx, w.repo.id, w.node, w.repo.language, mode)
				if err != nil {
					return fmt.Errorf("ingest: apply archive issue %d of repository %d: %w", w.node.Number, w.repo.id, err)
				}
				switch {
				case applied:
					results["applied"]++
					res.applied++
					if res.firstTitle == "" {
						res.firstTitle = w.node.Title
					}
				case mode == updateIfNewer:
					// Either stored and not older, or never stored.
					results["stale_or_not_stored"]++
				default:
					results["stale"]++
				}
			}
			return nil
		})
		if err != nil {
			return res, err
		}
	}

	if err := upsertRepoCandidates(ctx, pool, hour, candidates); err != nil {
		return res, err
	}
	res.candidates = len(candidates)

	for result, n := range results {
		metrics.ArchiveIssueEvents.WithLabelValues(result).Add(float64(n))
	}
	return res, nil
}

func upsertRepoCandidates(ctx context.Context, pool *db.Pool, hour time.Time, candidates map[int64]string) error {
	if len(candidates) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(candidates))
	names := make([]string, 0, len(candidates))
	for id, name := range candidates {
		ids = append(ids, id)
		names = append(names, SanitizeText(name))
	}
	err := pool.Exec(ctx,
		`INSERT INTO repo_candidates (github_database_id, full_name, first_seen_hour, last_seen_hour)
		 SELECT id, name, $3, $3 FROM unnest($1::bigint[], $2::text[]) AS c(id, name)
		 ON CONFLICT (github_database_id) DO UPDATE SET
		   full_name = EXCLUDED.full_name,
		   last_seen_hour = GREATEST(repo_candidates.last_seen_hour, EXCLUDED.last_seen_hour)`,
		ids, names, hour)
	if err != nil {
		return fmt.Errorf("ingest: store repository candidates: %w", err)
	}
	return nil
}

// ProcessArchiveHour downloads, reads and applies one claimed hour.
// updateOnly is the disk guard: while it is on, the hour updates stored
// issues and inserts none.
func (w *Worker) ProcessArchiveHour(ctx context.Context, h ArchiveHour, updateOnly bool) error {
	outcome, err := w.processArchiveHour(ctx, h, updateOnly)
	if errors.Is(err, ErrLeaseLost) {
		outcome = "lease_lost"
	}
	metrics.ArchiveHourOutcomes.WithLabelValues(outcome).Inc()
	return err
}

func (w *Worker) processArchiveHour(parent context.Context, h ArchiveHour, updateOnly bool) (string, error) {
	snap := w.live()
	retry := snap.Seconds(settings.IngestArchiveRetrySeconds)
	// The writes that end the hour's lease run outside the download deadline:
	// an hour that timed out must still record that it did.
	finishCtx := context.WithoutCancel(parent)
	fail := func(outcome string, cause error) (string, error) {
		if err := failArchiveHour(finishCtx, w.pool, h, cause, retry, snap.Int(settings.IngestMaxAttempts)); err != nil {
			return "persist_error", err
		}
		return outcome, cause
	}

	ctx, cancel := context.WithTimeout(parent, snap.Seconds(settings.IngestArchiveDownloadTimeoutSeconds))
	defer cancel()

	body, err := w.archive.open(ctx, h.Hour, int64(snap.Int(settings.IngestArchiveMaxFileMB))<<20)
	if errors.Is(err, ErrArchiveNotPublished) {
		missingAfter := time.Duration(snap.Int(settings.IngestArchiveMissingAfterHours)) * time.Hour
		if time.Since(h.Hour.Add(time.Hour)) > missingAfter {
			if err := markArchiveHourMissing(finishCtx, w.pool, h); err != nil {
				return "persist_error", err
			}
			slog.Warn("archive hour was never published; the repair crawl covers it", "hour", h.Hour)
			return "missing", nil
		}
		if err := releaseArchiveHour(finishCtx, w.pool, h, retry); err != nil {
			return "persist_error", err
		}
		return "not_published", nil
	}
	if err != nil {
		return fail("fetch_error", err)
	}
	defer body.Close()

	issues, events, err := readArchiveHour(body)
	if err != nil {
		return fail("parse_error", err)
	}

	idx, err := loadArchiveRepoIndex(ctx, w.pool, issues)
	if err != nil {
		return fail("persist_error", err)
	}
	res, err := applyArchiveIssues(ctx, w.pool, h.Hour, issues, idx, snap.Int(settings.IngestArchiveApplyBatchSize), updateOnly)
	if err != nil {
		return fail("persist_error", err)
	}
	res.eventsRead = events

	if err := completeArchiveHour(finishCtx, w.pool, h, res); err != nil {
		return "persist_error", err
	}
	metrics.IngestIssuesUpserted.Add(float64(res.applied))
	flow.Emit(ctx, flow.Event{
		From: flow.GitHub, To: flow.Ingestd, Kind: flow.KindCrawl,
		Label:   fmt.Sprintf("archive %s +%d", h.Hour.UTC().Format("2006-01-02T15"), res.applied),
		Payload: res.firstTitle,
	})
	slog.Info("applied archive hour", "hour", h.Hour, "events", events, "issues", len(issues),
		"applied", res.applied, "candidates", res.candidates)
	return "done", nil
}

// RepoCandidate is a repository the archive showed issue activity on that is
// not stored.
type RepoCandidate struct {
	GitHubID int64
	FullName string
}

// DueRepoCandidates returns up to limit candidates to look up: never looked
// up, or looked up more than recheck ago and active since. One stored since
// (by discovery, say) is not due.
func DueRepoCandidates(ctx context.Context, pool *db.Pool, limit int, recheck time.Duration) ([]RepoCandidate, error) {
	var out []RepoCandidate
	err := pool.QueryRows(ctx,
		`SELECT c.github_database_id, c.full_name FROM repo_candidates c
		 WHERE (c.checked_at IS NULL OR (c.checked_at < now() - $2::interval AND c.last_seen_hour > c.checked_at))
		   AND NOT EXISTS (SELECT 1 FROM repositories r WHERE r.github_database_id = c.github_database_id)
		 ORDER BY c.checked_at NULLS FIRST, c.github_database_id
		 LIMIT $1`,
		[]any{limit, recheck},
		func(rows pgx.Rows) error {
			var c RepoCandidate
			if err := rows.Scan(&c.GitHubID, &c.FullName); err != nil {
				return err
			}
			out = append(out, c)
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("ingest: select repository candidates: %w", err)
	}
	return out, nil
}

// PruneRepoCandidates deletes candidates that can never be due again on their
// own: looked up more than recheck ago, and quiet since. Without it the table
// grows by every small repository anyone opens an issue on. One that turns
// active again is queued afresh by the next hour that names it.
func PruneRepoCandidates(ctx context.Context, pool *db.Pool, recheck time.Duration) (int, error) {
	var n int
	err := pool.QueryRow(ctx,
		`WITH pruned AS (
		   DELETE FROM repo_candidates
		   WHERE checked_at < now() - $1::interval AND last_seen_hour <= checked_at
		   RETURNING 1
		 ) SELECT count(*) FROM pruned`,
		[]any{recheck},
		func(row pgx.Row) error { return row.Scan(&n) })
	if err != nil {
		return 0, fmt.Errorf("ingest: prune repository candidates: %w", err)
	}
	return n, nil
}

// ResolveRepoCandidates looks one batch of due candidates up on GitHub and
// stores those that pass Eligible and are not excluded, through the same path
// as the Discovery Sweeper. It returns how many it looked up; 0 with a nil
// error means none were due or the budget said wait.
func (w *Worker) ResolveRepoCandidates(ctx context.Context) (int, error) {
	snap := w.live()
	due, err := DueRepoCandidates(ctx, w.pool, snap.Int(settings.IngestArchiveCandidateBatchSize),
		time.Duration(snap.Int(settings.IngestArchiveCandidateRecheckDays))*24*time.Hour)
	if err != nil || len(due) == 0 {
		return 0, err
	}
	token := w.token()
	if token == "" {
		return 0, fmt.Errorf("no github token configured")
	}

	// Finding repositories is discovery work: it yields where discovery does.
	proceed, err := CanProceed(ctx, w.pool, LaneCrawler, installationIdentity,
		snap.Int(settings.IngestDiscoveryRateLimitFloor), snap.Int(settings.IngestInteractiveReservePercent))
	if err != nil {
		metrics.BudgetErrors.WithLabelValues("read", string(LaneCrawler)).Inc()
		return 0, err
	}
	if !proceed {
		return 0, nil
	}

	names := make([]string, len(due))
	for i, c := range due {
		names[i] = c.FullName
	}
	found, rl, err := w.github.LookupRepositories(ctx, token, names)
	if err != nil {
		return 0, fmt.Errorf("look up repository candidates: %w", err)
	}
	if err := UpdateBudget(ctx, w.pool, LaneCrawler, installationIdentity, rl); err != nil {
		metrics.BudgetErrors.WithLabelValues("write", string(LaneCrawler)).Inc()
		slog.Warn("failed to persist budget", "error", err)
	}

	minStars := snap.Int(settings.IngestEligibilityMinStars)
	pushedWithin := snap.Int(settings.IngestEligibilityPushedWithinDays)
	outcomes := map[string]int{}
	for _, c := range due {
		outcome := "not_found"
		err := w.pool.InTx(ctx, func(tx pgx.Tx) error {
			if r, ok := found[c.FullName]; ok {
				o, conflict, err := upsertDiscoveredTx(ctx, tx, r, minStars, pushedWithin)
				if err != nil {
					return err
				}
				outcome = o
				if conflict != "" {
					metrics.RepositoryIdentityConflicts.WithLabelValues(conflict).Inc()
				}
			}
			_, err := tx.Exec(ctx,
				`UPDATE repo_candidates SET checked_at = now(), outcome = $2 WHERE github_database_id = $1`,
				c.GitHubID, outcome)
			return err
		})
		if err != nil {
			return 0, fmt.Errorf("store repository candidate %s: %w", c.FullName, err)
		}
		outcomes[outcome]++
	}
	for o, n := range outcomes {
		metrics.ArchiveCandidates.WithLabelValues(o).Add(float64(n))
	}
	if outcomes[discoveredNew] > 0 {
		slog.Info("stored repositories found in the archive", "new", outcomes[discoveredNew], "looked_up", len(due))
	}
	return len(due), nil
}
