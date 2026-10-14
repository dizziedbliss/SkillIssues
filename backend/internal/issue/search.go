// Package issue manages the curated open-source issue catalog and saved issues.
package issue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

// Issue represents an open-source challenge issue joined with repository metadata.
type Issue struct {
	ID        int    `json:"id"`
	GitHubID  int64  `json:"github_id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	State     string `json:"state"`
	UpdatedAt string `json:"updated_at"`
	// DifficultyScore is nil for UNCLASSIFIED issues (D6) — never defaulted
	// to a number that would imply BEGINNER or any other difficulty.
	DifficultyScore       *int     `json:"difficulty_score"`
	Difficulty            string   `json:"difficulty"`
	RequiredSkills        []string `json:"required_skills"`
	Technologies          []string `json:"technologies"`
	LearningTags          []string `json:"learning_tags"`
	Comments              int      `json:"comments"`
	Author                string   `json:"author"`
	Repository            string   `json:"repository"`
	RepositoryURL         string   `json:"repository_url"`
	RepositoryDescription string   `json:"repository_description"`
	Language              string   `json:"language"`
	Stars                 int      `json:"stars"`
	License               string   `json:"license"`

	// Enriched fields added by recommendation scoring or search ranking
	RecommendationScore int      `json:"recommendation_score,omitempty"`
	WhyRecommended      []string `json:"why_recommended,omitempty"`
	MatchCategory       string   `json:"match_category,omitempty"`

	// Lifecycle is only populated for the saved-issues list (D2b): the
	// caller's own most recent contribution against this issue, one of
	// none|working|submitted|merged. Saved stays sticky regardless of
	// issue.State — this is what lets a client show why.
	Lifecycle string `json:"lifecycle,omitempty"`
}

// Filters for issue catalog queries.
type Filters struct {
	Difficulty string
	Language   string
	Search     string
	Limit      int
	Page       int
}

// Service manages issue queries and bookmarks.
type Service struct {
	db *db.Pool

	// perCategoryCap is read per query so the search shape can be retuned on a
	// running system. It is a narrow function rather than a config object so the
	// dependency is explicit and trivial to stub in tests.
	perCategoryCap func() int
}

// New creates a new issue Service. perCategoryCap may be nil, in which case a
// conservative built-in cap applies.
func New(db *db.Pool, perCategoryCap func() int) *Service {
	if perCategoryCap == nil {
		perCategoryCap = func() int { return defaultPerCategoryCap }
	}
	return &Service{db: db, perCategoryCap: perCategoryCap}
}

// defaultPerCategoryCap is the fallback when no provider is supplied.
const defaultPerCategoryCap = 25

// categoryCap clamps the configured cap into a sane range. The database
// validates bounds too, but a nil or corrupt snapshot must never produce a
// query that returns nothing or scans everything.
func (s *Service) categoryCap() int {
	n := s.perCategoryCap()
	if n < 1 {
		return 1
	}
	if n > 1000 {
		return 1000
	}
	return n
}

// No i.body here — D17 removed it from everything but the bounded
// issue_body_cache LRU (see internal/issue/body.go), which this doesn't read.
const baseIssueSelect = `
	SELECT i.id, COALESCE(i.github_id, 0), i.number, i.title,
	       i.url, i.state, i.updated_at::text,
	       i.difficulty_score, i.difficulty,
	       COALESCE(i.required_skills, '{}'),
	       COALESCE(i.technologies, '{}'),
	       COALESCE(i.learning_tags, '{}'),
	       i.comments, COALESCE(i.author, split_part(r.full_name, '/', 1)),
	       r.full_name, r.url, COALESCE(r.description, ''),
	       COALESCE(r.language, ''), r.stars, COALESCE(r.license, '')
	FROM issues i
	JOIN repositories r ON r.id = i.repository_id
`

func scanIssue(rows pgx.Rows) (Issue, error) {
	var it Issue
	err := rows.Scan(
		&it.ID, &it.GitHubID, &it.Number, &it.Title,
		&it.URL, &it.State, &it.UpdatedAt,
		&it.DifficultyScore, &it.Difficulty,
		&it.RequiredSkills, &it.Technologies, &it.LearningTags,
		&it.Comments, &it.Author,
		&it.Repository, &it.RepositoryURL, &it.RepositoryDescription,
		&it.Language, &it.Stars, &it.License,
	)
	return it, err
}

func scanIssueWithCategory(rows pgx.Rows) (Issue, error) {
	var it Issue
	err := rows.Scan(
		&it.ID, &it.GitHubID, &it.Number, &it.Title,
		&it.URL, &it.State, &it.UpdatedAt,
		&it.DifficultyScore, &it.Difficulty,
		&it.RequiredSkills, &it.Technologies, &it.LearningTags,
		&it.Comments, &it.Author,
		&it.Repository, &it.RepositoryURL, &it.RepositoryDescription,
		&it.Language, &it.Stars, &it.License,
		&it.MatchCategory,
	)
	return it, err
}

// SearchIssues queries the catalog with optional filters and user language preferences.
func (s *Service) SearchIssues(ctx context.Context, f Filters, prefLangs []string, userName string) ([]Issue, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	page := f.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	var args []any
	argIdx := 1

	if f.Search != "" {
		term := "%" + f.Search + "%"
		sRaw := f.Search

		var diffClause string
		if f.Difficulty != "" {
			diffClause = fmt.Sprintf(" AND i.difficulty = $%d", argIdx)
			args = append(args, strings.ToUpper(f.Difficulty))
			argIdx++
		}

		var langClause string
		if f.Language != "" {
			langClause = fmt.Sprintf(" AND lower(r.language) = lower($%d)", argIdx)
			args = append(args, f.Language)
			argIdx++
		}

		tIdx := argIdx
		sIdx := argIdx + 1
		args = append(args, term, sRaw)
		argIdx += 2

		// D12: Postgres FTS (title + labels, via the search_vector trigger —
		// there's no body to search, D17) replaces title ILIKE for the Title
		// category, and pg_trgm-backed ILIKE replaces it for Repository/
		// Author fuzzy matching. The Description category is gone entirely —
		// 4 categories remain (Repository, Title, Language, Author).
		searchQuery := fmt.Sprintf(`
			WITH ranked_matches AS (
				SELECT i.id, COALESCE(i.github_id, 0) AS github_id, i.number, i.title,
				       i.url AS issue_url, i.state, i.updated_at::text AS updated_at_text,
				       i.difficulty_score, i.difficulty,
				       COALESCE(i.required_skills, '{}') AS req_skills,
				       COALESCE(i.technologies, '{}') AS techs,
				       COALESCE(i.learning_tags, '{}') AS tags,
				       i.comments, COALESCE(i.author, split_part(r.full_name, '/', 1)) AS author_name,
				       r.full_name, r.url AS repo_url, COALESCE(r.description, '') AS repo_desc,
				       COALESCE(r.language, '') AS repo_lang, r.stars, COALESCE(r.license, '') AS repo_license,
				       CASE
				           WHEN lower(r.full_name) ILIKE $%d THEN 'Repository'
				           WHEN i.search_vector @@ websearch_to_tsquery('english', $%d) THEN 'Title'
				           WHEN lower(COALESCE(r.language, '')) = lower($%d)
				                OR lower(COALESCE(r.language, '')) ILIKE $%d
				                OR $%d = ANY(i.technologies)
				                OR $%d = ANY(i.learning_tags)
				                OR $%d = ANY(i.required_skills) THEN 'Language'
				           WHEN lower(COALESCE(i.author, split_part(r.full_name, '/', 1))) ILIKE $%d THEN 'Author'
				           ELSE 'Repository'
				       END AS match_cat,
				       ROW_NUMBER() OVER (
				           PARTITION BY CASE
				               WHEN lower(r.full_name) ILIKE $%d THEN 'Repository'
				               WHEN i.search_vector @@ websearch_to_tsquery('english', $%d) THEN 'Title'
				               WHEN lower(COALESCE(r.language, '')) = lower($%d)
				                    OR lower(COALESCE(r.language, '')) ILIKE $%d
				                    OR $%d = ANY(i.technologies)
				                    OR $%d = ANY(i.learning_tags)
				                    OR $%d = ANY(i.required_skills) THEN 'Language'
				               WHEN lower(COALESCE(i.author, split_part(r.full_name, '/', 1))) ILIKE $%d THEN 'Author'
				               ELSE 'Repository'
				           END
				           ORDER BY
				               CASE
				                   WHEN lower(split_part(r.full_name, '/', 2)) = lower($%d) THEN 1
				                   WHEN lower(split_part(r.full_name, '/', 2)) ILIKE $%d || '-%%'
				                        OR lower(split_part(r.full_name, '/', 2)) ILIKE $%d || '_%%'
				                        OR lower(split_part(r.full_name, '/', 2)) ILIKE '%%-' || $%d THEN 2
				                   WHEN lower(split_part(r.full_name, '/', 2)) ILIKE $%d || '%%' THEN 3
				                   ELSE 4
				               END ASC,
				               r.stars DESC,
				               i.difficulty_score ASC NULLS LAST,
				               i.updated_at DESC
				       ) as cat_rank
				FROM issues i
				JOIN repositories r ON r.id = i.repository_id
				WHERE i.state = 'OPEN' AND (
				    r.full_name ILIKE $%d
				    OR i.search_vector @@ websearch_to_tsquery('english', $%d)
				    OR r.language ILIKE $%d
				    OR $%d = ANY(i.technologies) OR $%d = ANY(i.learning_tags) OR $%d = ANY(i.required_skills)
				    OR i.author ILIKE $%d
				) %s %s
			)
			SELECT id, github_id, number, title, issue_url, state, updated_at_text,
			       difficulty_score, difficulty, req_skills, techs, tags,
			       comments, author_name, full_name, repo_url, repo_desc, repo_lang, stars, repo_license,
			       match_cat
			FROM ranked_matches
			WHERE cat_rank <= $%d
			ORDER BY
			    cat_rank ASC,
			    CASE match_cat
			        WHEN 'Repository' THEN 1
			        WHEN 'Title' THEN 2
			        WHEN 'Language' THEN 3
			        WHEN 'Author' THEN 4
			        ELSE 5
			    END ASC
			LIMIT $%d OFFSET $%d
		`,
			tIdx, sIdx, sIdx, tIdx, sIdx, sIdx, sIdx, tIdx,
			tIdx, sIdx, sIdx, tIdx, sIdx, sIdx, sIdx, tIdx,
			sIdx, sIdx, sIdx, sIdx, sIdx,
			tIdx, sIdx, tIdx, sIdx, sIdx, sIdx, tIdx,
			diffClause, langClause,
			argIdx, argIdx+1, argIdx+2)

		args = append(args, s.categoryCap(), limit, offset)

		var all []Issue
		err := s.db.QueryRows(ctx, searchQuery, args, func(rows pgx.Rows) error {
			it, err := scanIssueWithCategory(rows)
			if err != nil {
				return err
			}
			all = append(all, it)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("issue: search issues: %w", err)
		}
		return all, nil
	}

	// Normal browsing query without search
	var sb strings.Builder
	sb.WriteString(baseIssueSelect)
	sb.WriteString(" WHERE i.state = 'OPEN'")

	if f.Difficulty != "" {
		sb.WriteString(fmt.Sprintf(" AND i.difficulty = $%d", argIdx))
		args = append(args, strings.ToUpper(f.Difficulty))
		argIdx++
	}

	if f.Language != "" {
		sb.WriteString(fmt.Sprintf(" AND lower(r.language) = lower($%d)", argIdx))
		args = append(args, f.Language)
		argIdx++
	}

	if len(prefLangs) > 0 {
		var normLangs []string
		for _, l := range prefLangs {
			if tl := strings.TrimSpace(l); tl != "" {
				normLangs = append(normLangs, strings.ToLower(tl))
			}
		}
		sb.WriteString(fmt.Sprintf(` ORDER BY
			CASE WHEN lower(r.language) = ANY($%d) THEN 0 ELSE 1 END ASC,
			i.difficulty_score ASC,
			i.updated_at DESC`, argIdx))
		args = append(args, normLangs)
		argIdx++
	} else {
		sb.WriteString(" ORDER BY i.difficulty_score ASC, i.updated_at DESC")
	}

	sb.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1))
	args = append(args, limit, offset)

	var all []Issue
	err := s.db.QueryRows(ctx, sb.String(), args, func(rows pgx.Rows) error {
		it, err := scanIssue(rows)
		if err != nil {
			return err
		}
		all = append(all, it)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("issue: query issues: %w", err)
	}
	return all, nil
}

// GetIssue fetches a single issue by its database ID.
func (s *Service) GetIssue(ctx context.Context, id int) (*Issue, error) {
	query := baseIssueSelect + " WHERE i.id = $1"
	var found *Issue

	err := s.db.QueryRows(ctx, query, []any{id}, func(rows pgx.Rows) error {
		it, err := scanIssue(rows)
		if err != nil {
			return err
		}
		found = &it
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("issue: get issue: %w", err)
	}
	if found == nil {
		return nil, db.ErrNotFound
	}
	return found, nil
}

// GetRepositoryID returns the internal repository id an issue belongs to,
// for enqueueing a targeted resync (D16) without exposing that id in the
// public Issue JSON.
func (s *Service) GetRepositoryID(ctx context.Context, issueID int) (int, error) {
	var repoID int
	err := s.db.QueryRow(ctx, "SELECT repository_id FROM issues WHERE id = $1", []any{issueID}, func(row pgx.Row) error {
		return row.Scan(&repoID)
	})
	if err != nil {
		return 0, fmt.Errorf("issue: get repository id: %w", err)
	}
	if repoID == 0 {
		return 0, db.ErrNotFound
	}
	return repoID, nil
}

// savedIssueSelect joins each saved issue against the caller's own latest
// contribution for it (LEFT JOIN LATERAL — at most one row, the most
// recently updated). Saved stays sticky by contract: this never filters on
// issue or contribution state, it only labels what the client already shows.
const savedIssueSelect = `
	SELECT i.id, COALESCE(i.github_id, 0), i.number, i.title,
	       i.url, i.state, i.updated_at::text,
	       i.difficulty_score, i.difficulty,
	       COALESCE(i.required_skills, '{}'),
	       COALESCE(i.technologies, '{}'),
	       COALESCE(i.learning_tags, '{}'),
	       i.comments, COALESCE(i.author, split_part(r.full_name, '/', 1)),
	       r.full_name, r.url, COALESCE(r.description, ''),
	       COALESCE(r.language, ''), r.stars, COALESCE(r.license, ''),
	       COALESCE(c.lifecycle, 'none')
	FROM issues i
	JOIN repositories r ON r.id = i.repository_id
	LEFT JOIN LATERAL (
		SELECT CASE
			WHEN c2.state IN ('PR_MERGED', 'VERIFIED') THEN 'merged'
			WHEN c2.state = 'PR_OPEN' THEN 'submitted'
			WHEN c2.state = 'STARTED' THEN 'working'
			ELSE 'none'
		END AS lifecycle
		FROM contributions c2
		WHERE c2.user_id = $1 AND c2.issue_id = i.id
		ORDER BY c2.updated_at DESC
		LIMIT 1
	) c ON true
`

func scanSavedIssue(rows pgx.Rows) (Issue, error) {
	var it Issue
	err := rows.Scan(
		&it.ID, &it.GitHubID, &it.Number, &it.Title,
		&it.URL, &it.State, &it.UpdatedAt,
		&it.DifficultyScore, &it.Difficulty,
		&it.RequiredSkills, &it.Technologies, &it.LearningTags,
		&it.Comments, &it.Author,
		&it.Repository, &it.RepositoryURL, &it.RepositoryDescription,
		&it.Language, &it.Stars, &it.License,
		&it.Lifecycle,
	)
	return it, err
}

// GetSavedIssues returns all bookmarked issues for a developer, each labeled
// with the caller's lifecycle against it (D2b).
func (s *Service) GetSavedIssues(ctx context.Context, userID int) ([]Issue, error) {
	query := savedIssueSelect + `
		WHERE i.id IN (SELECT issue_id FROM saved_issues WHERE user_id = $1)
		ORDER BY i.difficulty_score, i.updated_at DESC
	`
	var issues []Issue
	err := s.db.QueryRows(ctx, query, []any{userID}, func(rows pgx.Rows) error {
		it, err := scanSavedIssue(rows)
		if err != nil {
			return err
		}
		issues = append(issues, it)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("issue: get saved: %w", err)
	}
	if issues == nil {
		issues = []Issue{}
	}
	return issues, nil
}

// SetState updates an issue's state and GitHub-reported updatedAt (D2a).
// Called only from an on-demand freshness check or a webhook delivery —
// never from the crawl path, which always writes OPEN.
func (s *Service) SetState(ctx context.Context, issueID int, state string, updatedAt time.Time) error {
	return s.db.Exec(ctx,
		`UPDATE issues SET state = $2, updated_at = $3, last_synced_at = now() WHERE id = $1`,
		issueID, state, updatedAt,
	)
}

// SetStateByRepoAndNumber is the webhook path (D2a): GitHub tells us the
// repo full_name and issue number, not our internal id. A no-op if the
// issue was never crawled — that repository/issue simply isn't in scope.
func (s *Service) SetStateByRepoAndNumber(ctx context.Context, repoFullName string, number int, state string, updatedAt time.Time) error {
	return s.db.Exec(ctx,
		`UPDATE issues i SET state = $3, updated_at = $4, last_synced_at = now()
		 FROM repositories r
		 WHERE r.id = i.repository_id AND r.full_name = $1 AND i.number = $2`,
		repoFullName, number, state, updatedAt,
	)
}

// SaveIssue bookmarks an issue for a developer.
func (s *Service) SaveIssue(ctx context.Context, userID, issueID int) error {
	// Verify issue exists
	var count int
	_ = s.db.QueryRow(ctx, "SELECT count(*) FROM issues WHERE id = $1", []any{issueID}, func(row pgx.Row) error {
		return row.Scan(&count)
	})
	if count == 0 {
		return db.ErrNotFound
	}

	return s.db.Exec(ctx,
		`INSERT INTO saved_issues (user_id, issue_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, issueID,
	)
}

// UnsaveIssue removes a bookmark.
func (s *Service) UnsaveIssue(ctx context.Context, userID, issueID int) error {
	return s.db.Exec(ctx,
		`DELETE FROM saved_issues WHERE user_id = $1 AND issue_id = $2`,
		userID, issueID,
	)
}

// GetRecommendationCandidates fetches all open qualified issues for recommendation scoring.
func (s *Service) GetRecommendationCandidates(ctx context.Context) ([]Issue, error) {
	query := baseIssueSelect + `
		WHERE i.state = 'OPEN'
		ORDER BY i.difficulty_score, i.updated_at DESC
	`
	var candidates []Issue
	err := s.db.QueryRows(ctx, query, nil, func(rows pgx.Rows) error {
		it, err := scanIssue(rows)
		if err != nil {
			return err
		}
		candidates = append(candidates, it)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("issue: get recommendation candidates: %w", err)
	}
	return candidates, nil
}
