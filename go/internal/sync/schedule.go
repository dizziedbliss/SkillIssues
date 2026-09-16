package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/jackc/pgx/v5"
)

// RepositoryJob represents a repository scheduled for issue synchronization.
type RepositoryJob struct {
	ID              int
	GithubID        int64
	FullName        string
	Language        *string
	Stars           int
	LastSyncedAt    *time.Time
	LastIssueSyncAt *time.Time
	Priority        string
}

// GetPendingRepositories queries repositories needing synchronization ordered by priority and recency.
func GetPendingRepositories(ctx context.Context, pool *db.Pool, limit int) ([]RepositoryJob, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT
			r.id,
			r.github_id,
			r.full_name,
			r.language,
			r.stars,
			s.last_synced_at,
			s.last_issue_sync_at,
			COALESCE(s.priority, 'MEDIUM') AS priority
		FROM repositories r
		LEFT JOIN sync_state s ON s.repository_id = r.id
		WHERE s.sync_status IS NULL OR s.sync_status != 'RUNNING'
		ORDER BY
			CASE COALESCE(s.priority, 'MEDIUM')
				WHEN 'CRITICAL' THEN 1
				WHEN 'HIGH' THEN 2
				WHEN 'MEDIUM' THEN 3
				ELSE 4
			END,
			COALESCE(s.last_synced_at, '1970-01-01'::timestamptz) ASC
		LIMIT $1;
	`

	var jobs []RepositoryJob
	err := pool.QueryRows(ctx, query, []any{limit}, func(rows pgx.Rows) error {
		var job RepositoryJob
		if err := rows.Scan(
			&job.ID,
			&job.GithubID,
			&job.FullName,
			&job.Language,
			&job.Stars,
			&job.LastSyncedAt,
			&job.LastIssueSyncAt,
			&job.Priority,
		); err != nil {
			return fmt.Errorf("scan pending repo: %w", err)
		}
		jobs = append(jobs, job)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("query pending repos: %w", err)
	}

	return jobs, nil
}

// MarkSyncStarted sets the sync status for a repository to RUNNING.
func MarkSyncStarted(ctx context.Context, pool *db.Pool, repoID int) error {
	query := `
		INSERT INTO sync_state (repository_id, sync_status, next_sync_at)
		VALUES ($1, 'RUNNING', now())
		ON CONFLICT (repository_id) DO UPDATE SET
			sync_status = 'RUNNING';
	`
	return pool.Exec(ctx, query, repoID)
}

// MarkSyncComplete updates sync_state telemetry, rate limits, and repository timestamps.
func MarkSyncComplete(ctx context.Context, pool *db.Pool, repoID int, rateLimit github.GraphQLRateLimit) error {
	query := `
		INSERT INTO sync_state (
			repository_id, last_synced_at, last_issue_sync_at, sync_status,
			failure_count, next_sync_at, rate_limit_limit, rate_limit_remaining,
			rate_limit_used, rate_limit_reset_at, rate_limit_cost
		)
		VALUES ($1, now(), now(), 'DONE', 0, now() + INTERVAL '15 minutes', $2, $3, $4, $5, $6)
		ON CONFLICT (repository_id) DO UPDATE SET
			last_synced_at = now(),
			last_issue_sync_at = now(),
			sync_status = 'DONE',
			failure_count = 0,
			next_sync_at = now() + INTERVAL '15 minutes',
			rate_limit_limit = EXCLUDED.rate_limit_limit,
			rate_limit_remaining = EXCLUDED.rate_limit_remaining,
			rate_limit_used = EXCLUDED.rate_limit_used,
			rate_limit_reset_at = EXCLUDED.rate_limit_reset_at,
			rate_limit_cost = EXCLUDED.rate_limit_cost;
	`
	var resetAt *time.Time
	if !rateLimit.ResetAt.IsZero() {
		resetAt = &rateLimit.ResetAt
	}

	err := pool.Exec(ctx, query, repoID, rateLimit.Limit, rateLimit.Remaining, rateLimit.Used, resetAt, rateLimit.Cost)
	if err != nil {
		return fmt.Errorf("update sync_state: %w", err)
	}

	_ = pool.Exec(ctx, "UPDATE repositories SET last_synced_at = now() WHERE id = $1", repoID)
	return nil
}

// MarkSyncFailed marks sync status as FAILED and increments failure_count with exponential backoff.
func MarkSyncFailed(ctx context.Context, pool *db.Pool, repoID int) error {
	query := `
		INSERT INTO sync_state (repository_id, sync_status, failure_count, next_sync_at)
		VALUES ($1, 'FAILED', 1, now() + INTERVAL '5 minutes')
		ON CONFLICT (repository_id) DO UPDATE SET
			sync_status = 'FAILED',
			failure_count = sync_state.failure_count + 1,
			next_sync_at = now() + (INTERVAL '5 minutes' * (sync_state.failure_count + 1));
	`
	return pool.Exec(ctx, query, repoID)
}

// UpsertIssue inserts or updates an issue in the catalog.
func UpsertIssue(
	ctx context.Context,
	pool *db.Pool,
	repoID int,
	issue github.GraphQLIssueNode,
	score int,
	difficulty string,
	hasValid bool,
	skills, tech, tags []string,
) error {
	query := `
		INSERT INTO issues (
			github_id, repository_id, number, title, body, url, state,
			difficulty_score, difficulty, required_skills, technologies, learning_tags,
			comments, has_difficulty_label, updated_at, author
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (github_id) DO UPDATE SET
			repository_id = EXCLUDED.repository_id,
			number = EXCLUDED.number,
			title = EXCLUDED.title,
			body = EXCLUDED.body,
			url = EXCLUDED.url,
			state = EXCLUDED.state,
			difficulty_score = EXCLUDED.difficulty_score,
			difficulty = EXCLUDED.difficulty,
			required_skills = EXCLUDED.required_skills,
			technologies = EXCLUDED.technologies,
			learning_tags = EXCLUDED.learning_tags,
			comments = EXCLUDED.comments,
			has_difficulty_label = EXCLUDED.has_difficulty_label,
			updated_at = EXCLUDED.updated_at,
			author = EXCLUDED.author;
	`
	author := issue.AuthorLogin
	if author == "" {
		author = "open-source"
	}

	updatedAt := issue.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	return pool.Exec(
		ctx,
		query,
		issue.DatabaseID,
		repoID,
		issue.Number,
		issue.Title,
		issue.Body,
		issue.URL,
		issue.State,
		score,
		difficulty,
		skills,
		tech,
		tags,
		issue.CommentsCount,
		hasValid,
		updatedAt,
		author,
	)
}

// SetRepositoryPriority updates or inserts scheduling priority ('CRITICAL', 'HIGH', 'MEDIUM', 'LOW').
func SetRepositoryPriority(ctx context.Context, pool *db.Pool, repoID int, priority string) error {
	query := `
		INSERT INTO sync_state (repository_id, priority)
		VALUES ($1, $2)
		ON CONFLICT (repository_id) DO UPDATE SET priority = EXCLUDED.priority;
	`
	return pool.Exec(ctx, query, repoID, priority)
}
