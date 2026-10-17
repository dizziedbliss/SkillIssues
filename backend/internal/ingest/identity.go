package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// Kinds of repository identity conflict, the label values of
// repository_identity_conflicts_total. The caller counts them once its
// transaction commits, so a rolled-back page is not counted.
const (
	// ConflictDuplicate: an id-less row turned out to be a repository another
	// row already holds by GitHub id (a dataset row imported under the name
	// it was renamed to). It was marked eligibility_reason='duplicate'.
	ConflictDuplicate = "duplicate"
	// ConflictRenameBlocked: a repository was renamed onto a name another
	// repository's row (with its own GitHub id) still holds. Nothing changed.
	ConflictRenameBlocked = "rename_blocked"
	// ConflictIDMismatch: a name now resolves to a different GitHub
	// repository than the one its row was recorded with. Nothing changed.
	ConflictIDMismatch = "id_mismatch"
)

// RecordRepositoryIDTx stores GitHub's id for the repository the crawl just
// fetched by name, inside the page's transaction. It returns a conflict kind
// ("" for none):
//
//   - no id yet and no other row holds it: the id is stored.
//   - another row already holds the id: this row is the id-less twin. It is
//     marked 'duplicate' (not crawled again, nothing deleted) and the row
//     with the id wins.
//   - this row holds a different id: the name now belongs to another
//     repository. Left as it is and reported.
func RecordRepositoryIDTx(ctx context.Context, tx pgx.Tx, repoID int, githubID int64) (string, error) {
	if githubID == 0 {
		return "", nil
	}
	var current *int64
	if err := tx.QueryRow(ctx,
		`SELECT github_database_id FROM repositories WHERE id = $1`, repoID,
	).Scan(&current); err != nil {
		return "", fmt.Errorf("ingest: read repository id: %w", err)
	}
	if current != nil {
		if *current == githubID {
			return "", nil
		}
		slog.Warn("repository name now resolves to a different GitHub repository; row left as it is",
			"repository_id", repoID, "stored_github_id", *current, "fetched_github_id", githubID)
		return ConflictIDMismatch, nil
	}

	var holder int
	err := tx.QueryRow(ctx,
		`SELECT id FROM repositories WHERE github_database_id = $1`, githubID,
	).Scan(&holder)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if _, err := tx.Exec(ctx,
			`UPDATE repositories SET github_database_id = $2 WHERE id = $1`, repoID, githubID,
		); err != nil {
			return "", fmt.Errorf("ingest: store repository id: %w", err)
		}
		return "", nil
	case err != nil:
		return "", fmt.Errorf("ingest: find repository by id: %w", err)
	}

	marked, err := markDuplicateTx(ctx, tx, repoID)
	if err != nil {
		return "", err
	}
	if !marked {
		return "", nil
	}
	slog.Warn("repository is a duplicate of a row holding its GitHub id; marked duplicate",
		"repository_id", repoID, "holder_repository_id", holder, "github_id", githubID)
	return ConflictDuplicate, nil
}

// markDuplicateTx makes a row ineligible as a duplicate. It reports whether
// the row changed, so a pair already resolved is not counted again.
func markDuplicateTx(ctx context.Context, tx pgx.Tx, repoID int) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE repositories SET is_eligible = false, eligibility_reason = 'duplicate'
		 WHERE id = $1 AND eligibility_reason IS DISTINCT FROM 'duplicate'`, repoID)
	if err != nil {
		return false, fmt.Errorf("ingest: mark repository duplicate: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
