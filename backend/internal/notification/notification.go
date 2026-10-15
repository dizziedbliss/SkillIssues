// Package notification reads and updates a user's in-app notifications.
//
// Notifications are created by the database, not here: the
// issues_notify_closed trigger (migration 00024) writes one when an issue a
// user saved or is working on goes OPEN -> CLOSED, whichever path closed it.
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
	Kind      string     `json:"kind"` // "issue_closed"
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at"`
	Issue     Issue      `json:"issue"`
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
		        i.id, r.full_name, i.number, i.title, i.url, i.state
		 FROM notifications n
		 JOIN issues i ON i.id = n.issue_id
		 JOIN repositories r ON r.id = i.repository_id
		 WHERE n.user_id = $1 AND (NOT $2 OR n.read_at IS NULL)
		 ORDER BY n.created_at DESC, n.id DESC
		 LIMIT $3`,
		[]any{userID, unreadOnly, limit},
		func(rows pgx.Rows) error {
			var n Notification
			if err := rows.Scan(&n.ID, &n.Kind, &n.CreatedAt, &n.ReadAt,
				&n.Issue.ID, &n.Issue.Repository, &n.Issue.Number, &n.Issue.Title, &n.Issue.URL, &n.Issue.State); err != nil {
				return err
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
