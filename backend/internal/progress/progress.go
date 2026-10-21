// Package progress is go-api's read side of pull request tracking and XP:
// a person's summary (level, streak, badges, languages), their recent XP, the
// pull requests the tracker holds for them, and "check now". ingestd writes all
// of it (internal/ingest/prs.go); this package only reads, and moves a check
// forward.
package progress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/xp"
)

// Service reads progress.
type Service struct {
	db *db.Pool
}

// New builds a Service.
func New(pool *db.Pool) *Service { return &Service{db: pool} }

// Event is a ledger row as the app shows it.
type Event struct {
	Source   string       `json:"source"`
	Key      string       `json:"key,omitempty"`
	Amount   int          `json:"amount"`
	Detail   xp.Detail    `json:"detail"`
	EarnedAt time.Time    `json:"earned_at"`
	Pull     *PullSummary `json:"pull_request,omitempty"`
}

// PullSummary names the pull request an event is for.
type PullSummary struct {
	ID         int64  `json:"id"`
	Repository string `json:"repository"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	URL        string `json:"url"`
}

// Sync is where tracking stands for a person.
type Sync struct {
	// Tracked is false until ingestd has enrolled them (within a cycle of
	// signing up).
	Tracked       bool       `json:"tracked"`
	SyncedThrough *time.Time `json:"synced_through"`
	// Backfilling: the first read of their history hasn't finished.
	Backfilling   bool       `json:"backfilling"`
	LastCheckedAt *time.Time `json:"last_checked_at"`
	NextCheckAt   *time.Time `json:"next_check_at"`
	RequestedAt   *time.Time `json:"requested_at"`
	LastError     *string    `json:"last_error"`
}

// Events returns a person's whole ledger, oldest first, for Summarise.
func (s *Service) Events(ctx context.Context, userID int) ([]xp.Event, error) {
	out := []xp.Event{}
	err := s.db.QueryRows(ctx,
		`SELECT source, COALESCE(key, ''), amount, detail, earned_at FROM xp_events WHERE user_id = $1 ORDER BY earned_at, id`,
		[]any{userID}, func(rows pgx.Rows) error {
			var e xp.Event
			var detail []byte
			if err := rows.Scan(&e.Source, &e.Key, &e.Amount, &detail, &e.EarnedAt); err != nil {
				return err
			}
			if err := json.Unmarshal(detail, &e.Detail); err != nil {
				return fmt.Errorf("decode xp detail: %w", err)
			}
			out = append(out, e)
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("progress: read ledger: %w", err)
	}
	return out, nil
}

// Recent returns a person's latest ledger rows, newest first, with the pull
// request each is for.
func (s *Service) Recent(ctx context.Context, userID, limit int) ([]Event, error) {
	out := []Event{}
	err := s.db.QueryRows(ctx,
		`SELECT e.source, COALESCE(e.key, ''), e.amount, e.detail, e.earned_at,
		        p.id, r.full_name, p.number, p.title, p.url
		 FROM xp_events e
		 LEFT JOIN pull_requests p ON p.id = e.pull_request_id
		 LEFT JOIN repositories r ON r.id = p.repository_id
		 WHERE e.user_id = $1
		 ORDER BY e.earned_at DESC, e.id DESC
		 LIMIT $2`, []any{userID, limit}, func(rows pgx.Rows) error {
			var e Event
			var detail []byte
			var pID *int64
			var repo, title, url *string
			var number *int
			if err := rows.Scan(&e.Source, &e.Key, &e.Amount, &detail, &e.EarnedAt, &pID, &repo, &number, &title, &url); err != nil {
				return err
			}
			if err := json.Unmarshal(detail, &e.Detail); err != nil {
				return fmt.Errorf("decode xp detail: %w", err)
			}
			if pID != nil {
				e.Pull = &PullSummary{ID: *pID, Repository: *repo, Number: *number, Title: *title, URL: *url}
			}
			out = append(out, e)
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("progress: read recent xp: %w", err)
	}
	return out, nil
}

// Pull is a tracked pull request with what it earned.
type Pull struct {
	ID               int64      `json:"id"`
	Repository       string     `json:"repository"`
	Number           int        `json:"number"`
	Title            string     `json:"title"`
	URL              string     `json:"url"`
	State            string     `json:"state"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	MergedAt         *time.Time `json:"merged_at"`
	ClosedAt         *time.Time `json:"closed_at"`
	Checks           *string    `json:"checks"`
	Approvals        int        `json:"approvals"`
	ChangesRequested int        `json:"changes_requested"`
	NoXPReason       *string    `json:"no_xp_reason"`
	// Issues are the catalog issues it says it closes.
	Issues []int `json:"issues"`
	// XP is what it has earned so far, all sources together.
	XP int `json:"xp"`
}

// Pulls returns a person's tracked pull requests, newest-updated first; open
// only when openOnly.
func (s *Service) Pulls(ctx context.Context, userID int, openOnly bool, limit int) ([]Pull, error) {
	out := []Pull{}
	err := s.db.QueryRows(ctx,
		`SELECT p.id, r.full_name, p.number, p.title, p.url, p.state, p.created_at, p.updated_at, p.merged_at, p.closed_at,
		        p.checks, p.approvals, p.changes_requested, p.no_xp_reason,
		        COALESCE((SELECT array_agg(l.issue_id ORDER BY l.issue_id) FROM pull_request_issues l WHERE l.pull_request_id = p.id), '{}'),
		        COALESCE((SELECT sum(e.amount) FROM xp_events e WHERE e.pull_request_id = p.id AND e.user_id = p.user_id), 0)
		 FROM pull_requests p JOIN repositories r ON r.id = p.repository_id
		 WHERE p.user_id = $1 AND (NOT $2 OR p.state = 'OPEN')
		 ORDER BY p.updated_at DESC, p.id DESC
		 LIMIT $3`, []any{userID, openOnly, limit}, func(rows pgx.Rows) error {
			var p Pull
			if err := rows.Scan(&p.ID, &p.Repository, &p.Number, &p.Title, &p.URL, &p.State, &p.CreatedAt, &p.UpdatedAt,
				&p.MergedAt, &p.ClosedAt, &p.Checks, &p.Approvals, &p.ChangesRequested, &p.NoXPReason, &p.Issues, &p.XP); err != nil {
				return err
			}
			out = append(out, p)
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("progress: read pull requests: %w", err)
	}
	return out, nil
}

// SyncOf says where tracking stands for a person.
func (s *Service) SyncOf(ctx context.Context, userID int) (Sync, error) {
	var st Sync
	err := s.db.QueryRow(ctx,
		`SELECT synced_through, last_checked_at, next_check_at, requested_at, last_error FROM pr_sync WHERE user_id = $1`,
		[]any{userID}, func(r pgx.Row) error {
			return r.Scan(&st.SyncedThrough, &st.LastCheckedAt, &st.NextCheckAt, &st.RequestedAt, &st.LastError)
		})
	if errors.Is(err, db.ErrNotFound) {
		return Sync{}, nil
	}
	if err != nil {
		return Sync{}, fmt.Errorf("progress: read pr_sync: %w", err)
	}
	st.Tracked = true
	st.Backfilling = st.SyncedThrough == nil
	return st, nil
}

// ErrTooSoon is a "check now" inside api.prs.refresh_min_seconds of the last.
var ErrTooSoon = errors.New("progress: checked too recently")

// RequestCheck makes the person due now, at most once per minGap. It
// returns when the next request is allowed.
func (s *Service) RequestCheck(ctx context.Context, userID int, minGap time.Duration) (time.Time, error) {
	var requested time.Time
	var granted bool
	err := s.db.QueryRow(ctx,
		`WITH prev AS (SELECT requested_at FROM pr_sync WHERE user_id = $1),
		 up AS (
		   INSERT INTO pr_sync (user_id, next_check_at, requested_at) VALUES ($1, now(), now())
		   ON CONFLICT (user_id) DO UPDATE SET next_check_at = LEAST(pr_sync.next_check_at, now()), requested_at = now()
		     WHERE pr_sync.requested_at IS NULL OR pr_sync.requested_at <= now() - make_interval(secs => $2)
		   RETURNING requested_at)
		 SELECT COALESCE((SELECT requested_at FROM up), (SELECT requested_at FROM prev)), EXISTS (SELECT 1 FROM up)`,
		[]any{userID, minGap.Seconds()}, func(r pgx.Row) error { return r.Scan(&requested, &granted) })
	if err != nil {
		return time.Time{}, fmt.Errorf("progress: request a check: %w", err)
	}
	if !granted {
		return requested.Add(minGap), ErrTooSoon
	}
	return requested.Add(minGap), nil
}
