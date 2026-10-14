package issue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"golang.org/x/sync/singleflight"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/ingest"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/dizziedbliss/skillissues/internal/worker"
	"github.com/jackc/pgx/v5"
)

// userReserve is the point budget below which a user's own token is no
// longer preferred for an on-demand read (plan §3.1 "500-point user
// reserve"): the GraphQL budget is per-user and shared with that user's own
// writes (fork, push, PR), so browsing without a reserve could leave a user
// unable to start a contribution until their window resets. Deliberately
// not a setting — it's a safety invariant, not a tuning knob.
const userReserve = 500

// ErrBodyUnavailable is the base sentinel for "body not available right now,
// retry later" — callers match it with errors.Is to catch every case below
// uniformly and respond 202. ErrBodyBudget and ErrBodyNoCredential wrap it
// to additionally say *why*, for callers that want to tell a client the
// difference (a transient rate limit vs. a configuration problem).
var ErrBodyUnavailable = fmt.Errorf("issue: body temporarily unavailable, retry later")

// ErrBodyBudget means the GraphQL point budget (the user's own reserve, or
// the installation's interactive lane) is exhausted right now. It recovers
// on its own as the rate-limit window rolls over.
var ErrBodyBudget = fmt.Errorf("%w: rate limit budget exhausted", ErrBodyUnavailable)

// ErrBodyNoCredential means no GitHub credential is available to make the
// call at all — no GitHub App installation token configured, or the
// installation token call itself failed. Retrying immediately won't help;
// this needs an operator fix, not backoff.
var ErrBodyNoCredential = fmt.Errorf("%w: no usable GitHub credential", ErrBodyUnavailable)

// BodyFetcher implements the on-demand issue body read path (plan §3.1):
// Postgres cache first, then a single-flight-coalesced GraphQL call
// preferring the requesting user's own token over the server's.
//
// D17 removed body from the broad ingest query entirely — this is the only
// place an issue body is ever fetched from GitHub.
type BodyFetcher struct {
	db      *db.Pool
	github  *github.Client
	sf      singleflight.Group
	limiter *worker.Limiter // bounds total concurrent GraphQL body fetches
	live    func() *settings.Snapshot

	// installationToken mints/returns the server's own credential, read
	// fresh per call so a rotated GitHub App token takes effect without a
	// restart.
	installationToken func(ctx context.Context) (string, error)
}

// NewBodyFetcher builds a BodyFetcher. installationToken may be nil if the
// server has no crawling credential configured at all (reads then rely
// entirely on the requesting user's own token). live supplies the
// api.issue_body.* settings, read on every fetch.
func NewBodyFetcher(pool *db.Pool, ghClient *github.Client, installationToken func(ctx context.Context) (string, error), live func() *settings.Snapshot) *BodyFetcher {
	return &BodyFetcher{
		db:                pool,
		github:            ghClient,
		limiter:           worker.NewLimiter(live().Int(settings.IssueBodyMaxConcurrentFetches)),
		live:              live,
		installationToken: installationToken,
	}
}

// GetBody returns issueID's body text. userID is 0 and userToken is "" for
// an anonymous/unauthenticated caller.
func (b *BodyFetcher) GetBody(ctx context.Context, issueID, userID int, userToken string) (string, error) {
	body, ok, err := b.readCache(ctx, issueID)
	if err != nil {
		// Not fatal — GitHub can still serve it — but a broken cache would
		// otherwise turn every read into a GitHub call without a trace.
		slog.Warn("issue body: cache read failed", "issue_id", issueID, "error", err)
	} else if ok {
		return body, nil
	}

	key := strconv.Itoa(issueID)
	v, err, _ := b.sf.Do(key, func() (any, error) {
		// Re-check the cache inside the single-flight: a concurrent caller
		// may have just populated it while this one was queued behind the
		// limiter below.
		body, ok, err := b.readCache(ctx, issueID)
		if err != nil {
			slog.Warn("issue body: cache re-check failed", "issue_id", issueID, "error", err)
		} else if ok {
			return body, nil
		}
		return b.fetchAndCache(ctx, issueID, userID, userToken)
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (b *BodyFetcher) readCache(ctx context.Context, issueID int) (string, bool, error) {
	var body string
	err := b.db.QueryRow(ctx,
		`UPDATE issue_body_cache SET last_accessed_at = now() WHERE issue_id = $1 RETURNING body`,
		[]any{issueID},
		func(row pgx.Row) error { return row.Scan(&body) },
	)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return body, true, nil
}

func (b *BodyFetcher) fetchAndCache(ctx context.Context, issueID, userID int, userToken string) (string, error) {
	var fullName string
	var number int
	err := b.db.QueryRow(ctx,
		`SELECT r.full_name, i.number FROM issues i JOIN repositories r ON r.id = i.repository_id WHERE i.id = $1`,
		[]any{issueID},
		func(row pgx.Row) error { return row.Scan(&fullName, &number) },
	)
	if err != nil {
		return "", fmt.Errorf("issue: look up repository for body fetch: %w", err)
	}
	owner, name, ok := strings.Cut(fullName, "/")
	if !ok {
		return "", fmt.Errorf("issue: invalid repository full_name %q", fullName)
	}

	snap := b.live()
	if n := snap.Int(settings.IssueBodyMaxConcurrentFetches); n != b.limiter.Limit() {
		b.limiter.SetLimit(n)
	}
	if err := b.limiter.Acquire(ctx); err != nil {
		return "", err
	}
	defer b.limiter.Release()

	// Tier 3a: the user's own token, if they have reserve left on it.
	if userToken != "" && userID != 0 {
		identity := fmt.Sprintf("user:%d", userID)
		proceed, err := ingest.CanProceed(ctx, b.db, ingest.LaneInteractive, identity, userReserve)
		if err != nil {
			metrics.BudgetErrors.WithLabelValues("read", string(ingest.LaneInteractive)).Inc()
			slog.Warn("issue body: user budget check failed, using installation token", "error", err)
		}
		if proceed {
			result, err := b.github.FetchIssueBody(ctx, userToken, owner, name, number)
			if err == nil {
				b.recordBudget(ctx, identity, result.RateLimit)
				b.store(ctx, issueID, result.Body)
				return result.Body, nil
			}
			// Falls through to the installation token on any error
			// (expired/revoked token, transient failure, etc).
			slog.Warn("issue body: fetch with user token failed, using installation token", "issue_id", issueID, "user_id", userID, "error", err)
		}
	}

	// Tier 3b: the installation token's interactive lane.
	if b.installationToken == nil {
		return "", ErrBodyNoCredential
	}
	// The interactive lane is its own budget row, so a saturated crawl can't
	// block reads.
	proceed, err := ingest.CanProceed(ctx, b.db, ingest.LaneInteractive, "installation", snap.Int(settings.IssueBodyInstallationFloor))
	if err != nil {
		metrics.BudgetErrors.WithLabelValues("read", string(ingest.LaneInteractive)).Inc()
		return "", fmt.Errorf("issue: check installation budget: %w", err)
	}
	if !proceed {
		return "", ErrBodyBudget
	}
	instToken, err := b.installationToken(ctx)
	if err != nil {
		slog.Error("issue body: installation token unavailable", "issue_id", issueID, "error", err)
		return "", fmt.Errorf("%w: %v", ErrBodyNoCredential, err)
	}
	if instToken == "" {
		slog.Error("issue body: installation token provider returned no token", "issue_id", issueID)
		return "", ErrBodyNoCredential
	}
	result, err := b.github.FetchIssueBody(ctx, instToken, owner, name, number)
	if err != nil {
		return "", fmt.Errorf("issue: fetch body: %w", err)
	}
	b.recordBudget(ctx, "installation", result.RateLimit)
	b.store(ctx, issueID, result.Body)
	return result.Body, nil
}

// recordBudget persists rate-limit telemetry. A failure doesn't fail the
// read the user is waiting on, but it is logged and counted.
func (b *BodyFetcher) recordBudget(ctx context.Context, identity string, rl github.GraphQLRateLimit) {
	if err := ingest.UpdateBudget(ctx, b.db, ingest.LaneInteractive, identity, rl); err != nil {
		metrics.BudgetErrors.WithLabelValues("write", string(ingest.LaneInteractive)).Inc()
		slog.Warn("issue body: failed to persist budget", "error", err)
	}
}

// store caches a fetched body. A failure doesn't fail the read the user is
// waiting on (they already have the body), but it is logged: every later
// read of this issue will spend GitHub budget again.
func (b *BodyFetcher) store(ctx context.Context, issueID int, body string) {
	if err := b.db.Exec(ctx,
		`INSERT INTO issue_body_cache (issue_id, body, fetched_at, last_accessed_at)
		 VALUES ($1, $2, now(), now())
		 ON CONFLICT (issue_id) DO UPDATE SET body = EXCLUDED.body, fetched_at = now(), last_accessed_at = now()`,
		issueID, body,
	); err != nil {
		slog.Warn("issue body: failed to cache body", "issue_id", issueID, "error", err)
	}
}

// PruneBodyCache deletes rows beyond maxRows (api.issue_body.cache_max_rows;
// ~50k rows is about 115 MB), oldest-by-last-access first. Call periodically
// from a ticker.
func PruneBodyCache(ctx context.Context, pool *db.Pool, maxRows int) (int, error) {
	tag, err := pool.Raw().Exec(ctx,
		`DELETE FROM issue_body_cache WHERE issue_id IN (
			SELECT issue_id FROM issue_body_cache ORDER BY last_accessed_at DESC OFFSET $1
		)`,
		maxRows,
	)
	if err != nil {
		return 0, fmt.Errorf("issue: prune body cache: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
