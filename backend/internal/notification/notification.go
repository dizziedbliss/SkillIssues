// Package notification reads and updates a user's in-app notifications.
//
// Notifications are created by the database, not here: the
// issues_notify_closed trigger (migration 00024) writes one when an issue a
// user saved or is working on goes OPEN -> CLOSED, whichever path closed it,
// and xp_events_notify_merged (00057) one when a merge of theirs is paid.
package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

// Notification is one entry in a user's list.
type Notification struct {
	ID        int64      `json:"id"`
	Kind      string     `json:"kind"` // "issue_closed" or "pr_merged"
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at"`
	// Issue is set for issue_closed, PullRequest for pr_merged.
	Issue       *Issue       `json:"issue,omitempty"`
	PullRequest *PullRequest `json:"pull_request,omitempty"`
}

// PullRequest is the merged pull request a notification is about, and the XP
// its merge earned.
type PullRequest struct {
	ID         int64  `json:"id"`
	Repository string `json:"repository"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	XP         int    `json:"xp"`
}

// Issue is the issue a notification is about, as it stands now.
type Issue struct {
	ID         int    `json:"id"`
	Repository string `json:"repository"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	State      string `json:"state"`
}

// Service reads and updates notifications.
type Service struct {
	db *db.Pool
}

// NewService builds a Service.
func NewService(pool *db.Pool) *Service {
	return &Service{db: pool}
}

// List returns up to limit of the user's notifications, newest first, and
// how many of all their notifications are unread.
func (s *Service) List(ctx context.Context, userID int, unreadOnly bool, limit int) ([]Notification, int, error) {
	out := []Notification{}
	err := s.db.QueryRows(ctx,
		`SELECT n.id, n.kind, n.created_at, n.read_at,
		        i.id, ir.full_name, i.number, i.title, i.url, i.state,
		        p.id, pr.full_name, p.number, p.title, p.url,
		        (SELECT COALESCE(sum(e.amount), 0) FROM xp_events e
		          WHERE e.user_id = n.user_id AND e.pull_request_id = p.id AND e.source = 'merged')
		 FROM notifications n
		 LEFT JOIN issues i ON i.id = n.issue_id
		 LEFT JOIN repositories ir ON ir.id = i.repository_id
		 LEFT JOIN pull_requests p ON p.id = n.pull_request_id
		 LEFT JOIN repositories pr ON pr.id = p.repository_id
		 WHERE n.user_id = $1 AND (NOT $2 OR n.read_at IS NULL)
		 ORDER BY n.created_at DESC, n.id DESC
		 LIMIT $3`,
		[]any{userID, unreadOnly, limit},
		func(rows pgx.Rows) error {
			var n Notification
			var iID, iNumber, pNumber *int
			var pID *int64
			var iRepo, iTitle, iURL, iState, pRepo, pTitle, pURL *string
			var xpAmount int
			if err := rows.Scan(&n.ID, &n.Kind, &n.CreatedAt, &n.ReadAt,
				&iID, &iRepo, &iNumber, &iTitle, &iURL, &iState,
				&pID, &pRepo, &pNumber, &pTitle, &pURL, &xpAmount); err != nil {
				return err
			}
			switch {
			case iID != nil:
				n.Issue = &Issue{ID: *iID, Repository: *iRepo, Number: *iNumber, Title: *iTitle, URL: *iURL, State: *iState}
			case pID != nil:
				n.PullRequest = &PullRequest{ID: *pID, Repository: *pRepo, Number: *pNumber, Title: *pTitle, URL: *pURL, XP: xpAmount}
			default:
				return fmt.Errorf("notification %d names neither an issue nor a pull request", n.ID)
			}
			out = append(out, n)
			return nil
		},
	)
	if err != nil {
		return nil, 0, fmt.Errorf("notification: list: %w", err)
	}

	var unread int
	if err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`,
		[]any{userID},
		func(row pgx.Row) error { return row.Scan(&unread) },
	); err != nil {
		return nil, 0, fmt.Errorf("notification: count unread: %w", err)
	}
	return out, unread, nil
}

// MarkRead marks one of the user's notifications read. It reports false when
// the user has no notification with that id. Marking an already-read one
// again keeps its original read_at.
func (s *Service) MarkRead(ctx context.Context, userID int, id int64) (bool, error) {
	var found bool
	err := s.db.QueryRow(ctx,
		`WITH target AS (
			SELECT id FROM notifications WHERE id = $1 AND user_id = $2
		 ), marked AS (
			UPDATE notifications SET read_at = now()
			WHERE id IN (SELECT id FROM target) AND read_at IS NULL
		 )
		 SELECT EXISTS (SELECT 1 FROM target)`,
		[]any{id, userID},
		func(row pgx.Row) error { return row.Scan(&found) },
	)
	if err != nil {
		return false, fmt.Errorf("notification: mark read: %w", err)
	}
	return found, nil
}

// MarkAllRead marks every unread notification of the user read and returns
// how many changed.
func (s *Service) MarkAllRead(ctx context.Context, userID int) (int, error) {
	var n int
	err := s.db.ExecReturning(ctx,
		`WITH marked AS (
			UPDATE notifications SET read_at = now()
			WHERE user_id = $1 AND read_at IS NULL
			RETURNING 1
		 )
		 SELECT count(*) FROM marked`,
		[]any{userID},
		func(row pgx.Row) error { return row.Scan(&n) },
	)
	if err != nil {
		return 0, fmt.Errorf("notification: mark all read: %w", err)
	}
	return n, nil
}
