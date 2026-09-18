// Package issue manages the curated open-source issue catalog and saved issues.
package issue

import (
	"context"
	"fmt"
	"strings"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

// Issue represents an open-source challenge issue joined with repository metadata.
type Issue struct {
	ID                    int      `json:"id"`
	GitHubID              int64    `json:"github_id"`
	Number                int      `json:"number"`
	Title                 string   `json:"title"`
	Body                  string   `json:"body"`
	URL                   string   `json:"url"`
	State                 string   `json:"state"`
	UpdatedAt             string   `json:"updated_at"`
	DifficultyScore       int      `json:"difficulty_score"`
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
}

// New creates a new issue Service.
func New(db *db.Pool) *Service {
	return &Service{db: db}
}

const baseIssueSelect = `
	SELECT i.id, COALESCE(i.github_id, 0), i.number, i.title, COALESCE(i.body, ''),
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
		&it.ID, &it.GitHubID, &it.Number, &it.Title, &it.Body,
		&it.URL, &it.State, &it.UpdatedAt,
		&it.DifficultyScore, &it.Difficulty,
		&it.RequiredSkills, &it.Technologies, &it.LearningTags,
		&it.Comments, &it.Author,
		&it.Repository, &it.RepositoryURL, &it.RepositoryDescription,
		&it.Language, &it.Stars, &it.License,
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

	var sb strings.Builder
	sb.WriteString(baseIssueSelect)
	sb.WriteString(" WHERE i.state = 'OPEN'")

	var args []any
	argIdx := 1

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

	if f.Search != "" {
		term := "%" + f.Search + "%"
		sb.WriteString(fmt.Sprintf(` AND (
			i.title ILIKE $%d OR i.body ILIKE $%d OR r.full_name ILIKE $%d OR r.language ILIKE $%d
			OR $%d = ANY(i.technologies) OR $%d = ANY(i.learning_tags) OR $%d = ANY(i.required_skills)
			OR i.author ILIKE $%d
		)`, argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4, argIdx+5, argIdx+6, argIdx+7))
		args = append(args, term, term, term, term, f.Search, f.Search, f.Search, term)
		argIdx += 8

		// Priority 1: Languages, 2: Repo names, 3: Username / author, 4: Issue title, 5: Issue description
		sb.WriteString(fmt.Sprintf(` ORDER BY
			CASE
				WHEN lower(r.language) = lower($%d) OR lower(r.language) ILIKE $%d THEN 1
				WHEN lower(r.full_name) ILIKE $%d THEN 2
				WHEN lower(COALESCE(i.author, split_part(r.full_name, '/', 1))) ILIKE $%d THEN 3
				WHEN lower(i.title) ILIKE $%d THEN 4
				WHEN lower(i.body) ILIKE $%d THEN 5
				ELSE 6
			END ASC,
			i.difficulty_score ASC,
			i.updated_at DESC`,
			argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4, argIdx+5))
		args = append(args, f.Search, term, term, term, term, term)
		argIdx += 6
	} else if len(prefLangs) > 0 {
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

		if f.Search != "" {
			sLower := strings.ToLower(f.Search)
			if strings.EqualFold(it.Language, f.Search) || strings.Contains(strings.ToLower(it.Language), sLower) {
				it.MatchCategory = "Language"
			} else if strings.Contains(strings.ToLower(it.Repository), sLower) {
				it.MatchCategory = "Repository"
			} else if strings.Contains(strings.ToLower(it.Author), sLower) {
				it.MatchCategory = "Author"
			} else if strings.Contains(strings.ToLower(it.Title), sLower) {
				it.MatchCategory = "Title"
			} else if strings.Contains(strings.ToLower(it.Body), sLower) {
				it.MatchCategory = "Description"
			}
		}

		all = append(all, it)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("issue: search issues: %w", err)
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

// GetSavedIssues returns all bookmarked issues for a developer.
func (s *Service) GetSavedIssues(ctx context.Context, userID int) ([]Issue, error) {
	query := baseIssueSelect + `
		WHERE i.id IN (SELECT issue_id FROM saved_issues WHERE user_id = $1)
		ORDER BY i.difficulty_score, i.updated_at DESC
	`
	var issues []Issue
	err := s.db.QueryRows(ctx, query, []any{userID}, func(rows pgx.Rows) error {
		it, err := scanIssue(rows)
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
