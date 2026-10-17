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

// PruneExcludedIssues deletes up to batch issues of repositories that are
// excluded, archived or a duplicate, except any a user saved. All three are
// facts about the repository rather than a threshold: excluded repositories
// are never crawled, nobody can contribute to an archive, and a duplicate's
// issues are the same issues stored again under the row that holds the
// repository's GitHub id, so what the crawl stored before that was known
// only wastes disk and fills the catalog. Repositories
// that merely fall below a threshold keep their issues; they may climb back.
// It returns how many rows it deleted.
func PruneExcludedIssues(ctx context.Context, pool *db.Pool, batch int) (int, error) {
	tag, err := pool.Raw().Exec(ctx,
		`DELETE FROM issues WHERE id IN (
			SELECT i.id FROM issues i
			JOIN repositories r ON r.id = i.repository_id
			WHERE NOT r.is_eligible
			  AND r.eligibility_reason ~ '(^|;)(excluded|archived|duplicate)(;|$)'
			  AND NOT EXISTS (SELECT 1 FROM saved_issues s WHERE s.issue_id = i.id)
			LIMIT $1
		 )`,
		batch,
	)
	if err != nil {
		return 0, fmt.Errorf("ingest: prune excluded issues: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
