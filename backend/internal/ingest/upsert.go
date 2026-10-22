package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/sync"
	"github.com/jackc/pgx/v5"
)

// UpsertIssueTx inserts or updates one issue inside an already-open
// transaction, so the caller can commit it atomically alongside the page's
// cursor/job-state update (plan §7 "atomic progress").
//
// Conflicts on (repository_id, number) — the issue's real identity — never
// on github_id, which the CLAUDE.md schema note and plan both call out:
// conflicting on github_id let a page's upsert silently clobber the wrong
// row whenever two repositories' issue numbers coincided with a stale
// github_id (defect fixed here, not carried forward from internal/sync).
func UpsertIssueTx(ctx context.Context, tx pgx.Tx, repoID int, issue github.GraphQLIssueNode, repoLanguage string) error {
	_, err := writeIssueTx(ctx, tx, repoID, issue, repoLanguage, writeAlways)
	return err
}

// issueWrite is when writeIssueTx may write an issue.
type issueWrite int

const (
	// writeAlways is the crawl: GitHub's answer as of now, written whatever
	// is stored.
	writeAlways issueWrite = iota
	// writeIfNewer is an archive event: the issue as it stood when the event
	// happened, which may be older than what is stored (hours are read in any
	// order, and a crawl may have run since). Inserted if not stored, updated
	// only if newer.
	writeIfNewer
	// updateIfNewer is writeIfNewer that never inserts: for a closed issue
	// nobody stored (it would only be pruned), or while the disk guard is on.
	updateIfNewer
)

// writeIssueTx writes one issue as mode allows and reports whether it did.
func writeIssueTx(ctx context.Context, tx pgx.Tx, repoID int, issue github.GraphQLIssueNode, repoLanguage string, mode issueWrite) (bool, error) {
	labels := SanitizeSlice(issue.Labels)
	score, difficulty, hasValid, skills, tech, tags := sync.ExtractMetadata(labels, repoLanguage)

	var difficultyScore *int
	if hasValid {
		difficultyScore = &score
	}

	author := SanitizeText(issue.AuthorLogin)
	if author == "" {
		author = "open-source"
	}

	var createdAt *time.Time
	if !issue.CreatedAt.IsZero() {
		createdAt = &issue.CreatedAt
	}
	// updated_at is GitHub's, never ours: both sources refuse an issue
	// without one, so this only guards a new caller from inventing one.
	updatedAt := issue.UpdatedAt
	if updatedAt.IsZero() {
		return false, fmt.Errorf("ingest: issue #%d has no updatedAt from GitHub", issue.Number)
	}

	args := []any{
		issue.DatabaseID, repoID, issue.Number, SanitizeText(issue.Title), issue.URL, issue.State,
		difficultyScore, difficulty, skills, tech, tags,
		labels, issue.CommentsCount, createdAt, updatedAt, author, issue.Assignees,
	}

	if mode == updateIfNewer {
		// The INSERT's arguments without created_at, which an update never
		// rewrites.
		tag, err := tx.Exec(ctx,
			`UPDATE issues SET
				github_id = $1, title = $4, url = $5, state = $6,
				difficulty_score = $7, difficulty = $8, required_skills = $9, technologies = $10, learning_tags = $11,
				labels = $12, comments = $13, updated_at = $14, last_synced_at = now(), author = $15,
				assignees = COALESCE($16, assignees)
			 WHERE repository_id = $2 AND number = $3 AND updated_at < $14`,
			append(append([]any{}, args[:13]...), args[14:]...)...)
		return tag.RowsAffected() > 0, err
	}

	newer := ""
	if mode == writeIfNewer {
		newer = " WHERE issues.updated_at < EXCLUDED.updated_at"
	}
	tag, err := tx.Exec(ctx,
		`INSERT INTO issues (
			github_id, repository_id, number, title, url, state,
			difficulty_score, difficulty, required_skills, technologies, learning_tags,
			labels, comments, created_at, updated_at, author, assignees
		 )
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		 ON CONFLICT (repository_id, number) DO UPDATE SET
			github_id                    = EXCLUDED.github_id,
			title                        = EXCLUDED.title,
			url                          = EXCLUDED.url,
			state                        = EXCLUDED.state,
			difficulty_score             = EXCLUDED.difficulty_score,
			difficulty                   = EXCLUDED.difficulty,
			required_skills              = EXCLUDED.required_skills,
			technologies                 = EXCLUDED.technologies,
			learning_tags                = EXCLUDED.learning_tags,
			labels                       = EXCLUDED.labels,
			comments                     = EXCLUDED.comments,
			updated_at                   = EXCLUDED.updated_at,
			last_synced_at               = now(),
			author                       = EXCLUDED.author,
			-- A source that didn't say keeps what was known.
			assignees                    = COALESCE(EXCLUDED.assignees, issues.assignees)`+newer,
		args...)
	return tag.RowsAffected() > 0, err
}
