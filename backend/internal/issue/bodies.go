package issue

import (
	"context"
	"errors"
	"fmt"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

// CachedBody returns an issue's body from the body cache (issue_bodies) and
// marks it read, which keeps it from being the next one evicted. ok is false
// on a miss. Only ingestd writes bodies.
func (s *Service) CachedBody(ctx context.Context, issueID int) (body string, ok bool, err error) {
	err = s.db.QueryRow(ctx,
		`UPDATE issue_bodies SET last_read_at = now() WHERE issue_id = $1 RETURNING body`,
		[]any{issueID}, func(row pgx.Row) error { return row.Scan(&body) })
	if errors.Is(err, db.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("issue: read cached body: %w", err)
	}
	return body, true, nil
}
