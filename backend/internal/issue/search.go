// Package issue manages the curated open-source issue catalog and saved issues.
package issue

import (
	"context"
	"fmt"
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
	// CreatedAt is when the issue was opened on GitHub; "" when unknown.
	CreatedAt string `json:"created_at"`
	// DifficultyScore is nil for UNCLASSIFIED issues (D6) — never defaulted
	// to a number that would imply BEGINNER or any other difficulty.
	DifficultyScore *int     `json:"difficulty_score"`
	Difficulty      string   `json:"difficulty"`
	RequiredSkills  []string `json:"required_skills"`
	Technologies    []string `json:"technologies"`
	LearningTags    []string `json:"learning_tags"`
	Labels          []string `json:"labels"`    // the issue's GitHub labels, as GitHub has them
	Tech            []string `json:"tech"`      // the repository's three largest languages, by GitHub's names
	Assignees       *int     `json:"assignees"` // people assigned on GitHub; null when not read yet
	// ContributingURL is the repository's contributing guide on GitHub; null
	// when it has none or the crawl hasn't read it yet.
	ContributingURL *string `json:"contributing_url"`
	// Claimed: someone on SkillIssues said they are working on it.
	Claimed bool `json:"claimed"`
	// Health is how the repository's latest merges went; nil until the
	// crawl has read enough of them.
	Health *Health `json:"repository_health"`

	healthSampled, healthOutside, healthMedianHours *int
	Comments                                        int    `json:"comments"`
	Author                                          string `json:"author"`
	Repository                                      string `json:"repository"`
	RepositoryID                                    int    `json:"repository_id"`
	RepositoryURL                                   string `json:"repository_url"`
	RepositoryDescription                           string `json:"repository_description"`
	Language                                        string `json:"language"`
	Stars                                           int    `json:"stars"`
	License                                         string `json:"license"`

	// Enriched fields added by recommendation scoring or search ranking
	RecommendationScore int      `json:"recommendation_score,omitempty"`
	WhyRecommended      []string `json:"why_recommended,omitempty"`
	MatchCategory       string   `json:"match_category,omitempty"`

	// updated and created are updated_at and created_at as timestamps rather
	// than text, for the browse cursors (browse.go): a cursor must name an
	// exact position, to the microsecond. created is zero when unknown.
	updated time.Time
	created time.Time
}

// Filters for issue catalog queries. Within one list any value matches;
// across filters every one must (filters.go).
type Filters struct {
	Search string
	Limit  int
	Page   int
	// Sort orders a browse; "" is the caller's default (the handler picks
	// shuffle for cursor paging, easiest for page numbers). A search orders
	// by match category and ignores it.
	Sort Sort

	Difficulties []string // upper-cased, from Difficulties
	Languages    []string // the repository's main language
	Labels       []string // issue labels, case-insensitively
	Topics       []string // the repository's GitHub topics
	Tech         []string // any language GitHub reports in the repository
	Category     string   // a search's one category (ParseCategory); "" is all of them
	Repos        []string // owner/name, case-insensitively
	Owners       []string // the part of owner/name before the slash, case-insensitively
	Unassigned   bool     // only issues nobody is assigned to (unknown counts as unassigned)
	CommentsMin  *int
	CommentsMax  *int

	// RareLanguageBelow is api.issues.rare_language_threshold for this
	// request: BrowseIssues reads a language with fewer open issues than this
	// repository-first and sorts it, rather than walking the catalog index.
	// 0 always walks the index.
	RareLanguageBelow int
	// LanguageCountTTL is api.issues.language_count_ttl_seconds: how long a
	// language's counted issues are reused before it is counted again.
	LanguageCountTTL time.Duration
	// RareProbe is api.issues.rare_probe_ms: how long isRare may spend
	// counting a combination of filters. One whose matches take longer to
	// count than this has too many to sort, so it is read in index order.
	RareProbe time.Duration
	// ShuffleWalkMaxRepos is api.issues.shuffle_walk_max_repos: how many
	// repositories the shuffled feed walks for one page before reading the
	// rest of it from the matching issues instead (shuffle.go).
	ShuffleWalkMaxRepos int
	// ShuffleWalkBudget is api.issues.shuffle_walk_budget_ms: how long one
	// page of the shuffled feed may look before it is returned short, with
	// a cursor to carry on from.
	ShuffleWalkBudget time.Duration
	// SearchSample is api.search.sample_size: how many title or author
	// matches a search reads before ordering them by stars
	// (search_categories.go).
	SearchSample int
	// SearchPerRepo is api.search.per_repo_cap: the most issues one
	// repository contributes to a repository-led search category.
	SearchPerRepo int
}

// Service manages issue queries and bookmarks.
type Service struct {
	db *db.Pool

	// languages remembers how many open issues each language has, for
	// BrowseIssues to choose how to read it.
	languages languageCounts

	// hidden is what reports have taken out of lists (hidden.go); nil hides nothing.
	hidden  func() Hidden
	claimed func() map[int]bool
	// healthRules decides Health's flags (health.go).
	healthRules func() HealthRules

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

// repoTechSelect is a repository's three largest languages by name, from the
// dataset's languages (a JSON array of {name, size}); empty when unknown.
const repoTechSelect = `ARRAY(
	         SELECT e->>'name'
	         FROM jsonb_array_elements(CASE WHEN jsonb_typeof(r.languages) = 'array' THEN r.languages ELSE '[]'::jsonb END) e
	         WHERE COALESCE(e->>'name', '') <> ''
	         ORDER BY CASE WHEN jsonb_typeof(e->'size') = 'number' THEN (e->>'size')::numeric END DESC NULLS LAST
	         LIMIT 3)`

// No body here: bodies are large, attacker-controlled and fetched on their own
// (GET /issues/{id}/body, issue_bodies).
const baseIssueSelect = `
	SELECT i.id, COALESCE(i.github_id, 0), i.number, i.title,
	       i.url, i.state, i.updated_at::text,
	       i.difficulty_score, i.difficulty,
	       COALESCE(i.required_skills, '{}'),
	       COALESCE(i.technologies, '{}'),
	       COALESCE(i.learning_tags, '{}'),
	       COALESCE(i.labels, '{}'),
	       ` + repoTechSelect + `,
	       i.comments, COALESCE(i.author, split_part(r.full_name, '/', 1)),
	       r.full_name, r.url, COALESCE(r.description, ''),
	       COALESCE(r.language, ''), r.stars, COALESCE(r.license, ''),
	       i.updated_at, COALESCE(i.created_at::text, ''), i.created_at, i.repository_id, i.assignees,
	       r.contributing_url, r.health_sampled, r.health_outside, r.health_median_merge_hours
	FROM issues i
	JOIN repositories r ON r.id = i.repository_id
`

func scanIssue(rows pgx.Rows) (Issue, error) {
	var it Issue
	var created *time.Time
	err := rows.Scan(
		&it.ID, &it.GitHubID, &it.Number, &it.Title,
		&it.URL, &it.State, &it.UpdatedAt,
		&it.DifficultyScore, &it.Difficulty,
		&it.RequiredSkills, &it.Technologies, &it.LearningTags,
		&it.Labels, &it.Tech,
		&it.Comments, &it.Author,
		&it.Repository, &it.RepositoryURL, &it.RepositoryDescription,
		&it.Language, &it.Stars, &it.License,
		&it.updated, &it.CreatedAt, &created, &it.RepositoryID, &it.Assignees,
		&it.ContributingURL, &it.healthSampled, &it.healthOutside, &it.healthMedianHours,
	)
	if created != nil {
		it.created = *created
	}
	return it, err
}

// SearchIssues queries the catalog with optional filters and user language preferences.
func (s *Service) SearchIssues(ctx context.Context, f Filters, prefLangs []string, userName string) ([]Issue, error) {
	found, err := s.searchIssues(ctx, f, prefLangs, userName)
	return s.visible(found, f.Unassigned), err
}

func (s *Service) searchIssues(ctx context.Context, f Filters, prefLangs []string, userName string) ([]Issue, error) {
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

	if f.Search != "" {
		return s.searchCategories(ctx, f, limit, offset)
	}

	// Browsing by page number: an OFFSET into the easiest order. Kept for
	// callers that page with ?page; BrowseIssues serves the first page and
	// every cursor page without the cost of skipping rows.
	var q browseQuery
	q.sb.WriteString(baseIssueSelect)
	q.sb.WriteString(" WHERE i.state = 'OPEN'")
	q.sb.WriteString(f.conds(q.arg))

	// As in easiestPage, a language filter makes the preference moot.
	langs := normaliseLanguages(prefLangs)
	if len(langs) > 0 && len(f.Languages) == 0 {
		q.sb.WriteString(` ORDER BY
			CASE WHEN lower(r.language) = ANY(` + q.arg(langs) + `) THEN 0 ELSE 1 END ASC, ` + catalogOrder)
	} else {
		q.sb.WriteString(" ORDER BY " + catalogOrder)
	}
	q.sb.WriteString(" LIMIT " + q.arg(limit) + " OFFSET " + q.arg(offset))

	all, err := s.queryIssues(ctx, q.sb.String(), q.args)
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
	s.annotate(found, s.claimedNow(), s.rulesNow())
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

// savedIssueSelect is the column list for a saved issue. Saved stays sticky by
// contract: nothing here filters on issue state, so a closed issue stays listed.
const savedIssueSelect = `
	SELECT i.id, COALESCE(i.github_id, 0), i.number, i.title,
	       i.url, i.state, i.updated_at::text,
	       i.difficulty_score, i.difficulty,
	       COALESCE(i.required_skills, '{}'),
	       COALESCE(i.technologies, '{}'),
	       COALESCE(i.learning_tags, '{}'),
	       COALESCE(i.labels, '{}'),
	       ` + repoTechSelect + `,
	       i.comments, COALESCE(i.author, split_part(r.full_name, '/', 1)),
	       r.full_name, r.url, COALESCE(r.description, ''),
	       COALESCE(r.language, ''), r.stars, COALESCE(r.license, ''), i.repository_id, i.assignees,
	       r.contributing_url, r.health_sampled, r.health_outside, r.health_median_merge_hours
	FROM issues i
	JOIN repositories r ON r.id = i.repository_id
`

func scanSavedIssue(rows pgx.Rows) (Issue, error) {
	var it Issue
	err := rows.Scan(
		&it.ID, &it.GitHubID, &it.Number, &it.Title,
		&it.URL, &it.State, &it.UpdatedAt,
		&it.DifficultyScore, &it.Difficulty,
		&it.RequiredSkills, &it.Technologies, &it.LearningTags,
		&it.Labels, &it.Tech,
		&it.Comments, &it.Author,
		&it.Repository, &it.RepositoryURL, &it.RepositoryDescription,
		&it.Language, &it.Stars, &it.License, &it.RepositoryID, &it.Assignees,
		&it.ContributingURL, &it.healthSampled, &it.healthOutside, &it.healthMedianHours,
	)
	return it, err
}

// GetSavedIssues returns all bookmarked issues for a developer.
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
	s.markClaimed(issues)
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
//
// An event whose updated_at is not newer than the stored one is ignored:
// webhooks are redelivered and arrive out of order, and a replayed "closed"
// must not undo a later reopen the crawl has already recorded.
func (s *Service) SetStateByRepoAndNumber(ctx context.Context, repoFullName string, number int, state string, updatedAt time.Time) error {
	return s.db.Exec(ctx,
		`UPDATE issues i SET state = $3, updated_at = $4, last_synced_at = now()
		 FROM repositories r
		 WHERE r.id = i.repository_id AND r.full_name = $1 AND i.number = $2
		   AND i.updated_at < $4`,
		repoFullName, number, state, updatedAt,
	)
}

// SaveIssue bookmarks an issue for a developer.
func (s *Service) SaveIssue(ctx context.Context, userID, issueID int) error {
	// Verify issue exists
	var count int
	if err := s.db.QueryRow(ctx, "SELECT count(*) FROM issues WHERE id = $1", []any{issueID}, func(row pgx.Row) error {
		return row.Scan(&count)
	}); err != nil {
		return err
	}
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
