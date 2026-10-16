package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
)

// PruneClosedIssues deletes up to batch issues that closed on GitHub more
// than retention ago (updated_at is GitHub's updatedAt, which closing sets)
// and that no user holds onto: not saved by anyone. saved_issues would cascade
// on delete, which is exactly why saved issues are excluded rather than relied
// on. It returns how many rows it deleted.
func PruneClosedIssues(ctx context.Context, pool *db.Pool, retention time.Duration, batch int) (int, error) {
	tag, err := pool.Raw().Exec(ctx,
		`DELETE FROM issues WHERE id IN (
			SELECT i.id FROM issues i
			WHERE i.state = 'CLOSED'
			  AND i.updated_at < now() - ($1 * interval '1 second')
			  AND NOT EXISTS (SELECT 1 FROM saved_issues s WHERE s.issue_id = i.id)
			ORDER BY i.updated_at
			LIMIT $2
		 )`,
		retention.Seconds(), batch,
	)
	if err != nil {
		return 0, fmt.Errorf("ingest: prune closed issues: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
