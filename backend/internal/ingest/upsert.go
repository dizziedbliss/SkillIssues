package ingest

import (
	"context"
	"time"

	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/sync"
	"github.com/jackc/pgx/v5"
)

// classificationRulesVersion bumps whenever ClassifyLabels' rules change in
// a way that would alter existing rows' difficulty, so a future backfill
// knows which rows were classified under an older rule set.
const classificationRulesVersion = 1

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
	updatedAt := issue.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO issues (
			github_id, repository_id, number, title, url, state,
			difficulty_score, difficulty, required_skills, technologies, learning_tags,
			labels, classification_rules_version, comments, has_difficulty_label,
			created_at, updated_at, author
		 )
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
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
			classification_rules_version = EXCLUDED.classification_rules_version,
			comments                     = EXCLUDED.comments,
			has_difficulty_label         = EXCLUDED.has_difficulty_label,
			updated_at                   = EXCLUDED.updated_at,
			last_synced_at               = now(),
			author                       = EXCLUDED.author`,
		issue.DatabaseID, repoID, issue.Number, SanitizeText(issue.Title), issue.URL, issue.State,
		difficultyScore, difficulty, skills, tech, tags,
		labels, classificationRulesVersion, issue.CommentsCount, hasValid,
		createdAt, updatedAt, author,
	)
	return err
}
