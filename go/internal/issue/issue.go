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

	// Enriched fields added by recommendation scoring
	RecommendationScore int      `json:"recommendation_score,omitempty"`
	WhyRecommended      []string `json:"why_recommended,omitempty"`
}

// Filters for issue catalog queries.
type Filters struct {
	Difficulty string
	Language   string
	Search     string
	Limit      int
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
	if limit > 50 {
		limit = 50
	}

	var sb strings.Builder
	sb.WriteString(baseIssueSelect)
	sb.WriteString(" WHERE i.state = 'OPEN' AND (i.has_difficulty_label = true OR r.full_name = 'dizziedbliss/listtty')")

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
		)`, argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4, argIdx+5, argIdx+6))
		args = append(args, term, term, term, term, f.Search, f.Search, f.Search)
		argIdx += 7
	}

	sb.WriteString(" ORDER BY i.difficulty_score, i.updated_at DESC")

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
		return nil, fmt.Errorf("issue: search issues: %w", err)
	}

	// If preferred languages are specified and not an explicit keyword search,
	// prioritize preferred language matching (mirroring Python logic)
	if len(prefLangs) > 0 && f.Search == "" {
		prefSet := make(map[string]bool)
		for _, l := range prefLangs {
			prefSet[strings.ToLower(strings.TrimSpace(l))] = true
		}
		userLower := strings.ToLower(userName)

		var filtered []Issue
		for _, item := range all {
			repoLower := strings.ToLower(item.Repository)
			isUserRepo := repoLower == "dizziedbliss/listtty" || (userLower != "" && strings.HasPrefix(repoLower, userLower+"/"))

			itemLang := strings.ToLower(item.Language)
			matchesPref := prefSet[itemLang]
			if !matchesPref {
				for _, t := range item.Technologies {
					if prefSet[strings.ToLower(t)] {
						matchesPref = true
						break
					}
				}
			}
			if !matchesPref {
				for _, s := range item.RequiredSkills {
					if prefSet[strings.ToLower(s)] {
						matchesPref = true
						break
					}
				}
			}
			if !matchesPref {
				for _, tag := range item.LearningTags {
					if prefSet[strings.ToLower(tag)] {
						matchesPref = true
						break
					}
				}
			}

			if isUserRepo || matchesPref {
				filtered = append(filtered, item)
			}
		}

		if len(filtered) > limit {
			return filtered[:limit], nil
		}
		if len(filtered) > 0 {
			return filtered, nil
		}
	}

	if len(all) > limit {
		return all[:limit], nil
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
		WHERE i.state = 'OPEN' AND (i.has_difficulty_label = true OR r.full_name = 'dizziedbliss/listtty')
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
