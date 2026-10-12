// Package contribution manages the developer contribution lifecycle, PR linking,
// submit flow, and server-authoritative PR verification.
package contribution

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/cache"
	"github.com/dizziedbliss/skillissues/internal/crypto"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
)

// ContributionState represents the state of a contribution.
type ContributionState string

const (
	StateStarted  ContributionState = "STARTED"
	StatePROpen   ContributionState = "PR_OPEN"
	StatePRClosed ContributionState = "PR_CLOSED"
	StatePRMerged ContributionState = "PR_MERGED"
	StateVerified ContributionState = "VERIFIED"
)

// Contribution represents an active or completed developer challenge attempt.
type Contribution struct {
	ID        int        `json:"id"`
	UserID    int        `json:"user_id"`
	IssueID   int        `json:"issue_id"`
	PRURL     *string    `json:"pr_url"`
	PRNumber  *int       `json:"pr_number"`
	State     string     `json:"state"`
	XPAwarded bool       `json:"xp_awarded"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	MergedAt  *time.Time `json:"merged_at"`
}

// ContributionDetail enriches a contribution with issue and repository fields.
type ContributionDetail struct {
	ID             int      `json:"id"`
	UserID         int      `json:"user_id"`
	IssueID        int      `json:"issue_id"`
	PRURL          string   `json:"pr_url"`
	PRNumber       int      `json:"pr_number"`
	State          string   `json:"state"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
	MergedAt       string   `json:"merged_at"`
	Title          string   `json:"title"`
	Number         int      `json:"number"`
	Difficulty     string   `json:"difficulty"`
	IssueURL       string   `json:"issue_url"`
	RequiredSkills []string `json:"required_skills"`
	Technologies   []string `json:"technologies"`
	IssueUpdatedAt string   `json:"issue_updated_at"`
	Repository     string   `json:"repository"`
	RepositoryURL  string   `json:"repository_url,omitempty"`
	Language       string   `json:"language"`
}

// Service manages contributions and anti-exploit verification.
type Service struct {
	db       *db.Pool
	cache    *cache.Client
	ghClient *github.Client

	// verifyQuota bounds PR verification attempts. Read per call so the limit
	// can be tightened on a running system if it is being abused.
	verifyQuota func() int

	// tokenCipher decrypts users.github_access_token, which auth.Service
	// encrypts before storing. Nil only in tests that don't set it.
	tokenCipher *crypto.Cipher
}

// SetTokenCipher installs the cipher used to decrypt stored GitHub access
// tokens. Safe to call only during startup wiring.
func (s *Service) SetTokenCipher(c *crypto.Cipher) {
	s.tokenCipher = c
}

// decryptToken returns "" if the value can't be decrypted rather than ever
// treating ciphertext as a usable token.
func (s *Service) decryptToken(encrypted string) string {
	if encrypted == "" || s.tokenCipher == nil {
		return encrypted
	}
	plain, err := s.tokenCipher.Decrypt(encrypted)
	if err != nil {
		return ""
	}
	return plain
}

// New creates a new contribution Service. verifyQuota may be nil, in which case
// a built-in limit applies.
func New(db *db.Pool, cache *cache.Client, ghClient *github.Client, verifyQuota func() int) *Service {
	if verifyQuota == nil {
		verifyQuota = func() int { return defaultVerifyQuota }
	}
	return &Service{
		db:          db,
		cache:       cache,
		ghClient:    ghClient,
		verifyQuota: verifyQuota,
	}
}

// defaultVerifyQuota is the fallback hourly verification limit.
const defaultVerifyQuota = 5

// verifyAttemptsPerHour clamps the configured quota so a bad snapshot can never
// disable the anti-abuse limit entirely.
func (s *Service) verifyAttemptsPerHour() int64 {
	n := s.verifyQuota()
	if n < 1 {
		return 1
	}
	return int64(n)
}

// StartContribution creates or touches a contribution record in STARTED state.
func (s *Service) StartContribution(ctx context.Context, userID, issueID int) (*Contribution, error) {
	// Verify issue exists
	var exists bool
	_ = s.db.QueryRow(ctx, "SELECT true FROM issues WHERE id = $1", []any{issueID}, func(row pgx.Row) error {
		return row.Scan(&exists)
	})
	if !exists {
		return nil, db.ErrNotFound
	}

	var c Contribution
	err := s.db.QueryRow(ctx,
		`INSERT INTO contributions (user_id, issue_id, state)
		 VALUES ($1, $2, 'STARTED')
		 ON CONFLICT (user_id, issue_id) WHERE state = 'STARTED' DO UPDATE SET
		     updated_at = now()
		 RETURNING id, user_id, issue_id, pr_url, pr_number, state, xp_awarded, created_at, updated_at, merged_at`,
		[]any{userID, issueID},
		func(row pgx.Row) error {
			return row.Scan(&c.ID, &c.UserID, &c.IssueID, &c.PRURL, &c.PRNumber, &c.State, &c.XPAwarded, &c.CreatedAt, &c.UpdatedAt, &c.MergedAt)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("contribution: start: %w", err)
	}
	return &c, nil
}

// ListContributions returns all contributions for a developer.
func (s *Service) ListContributions(ctx context.Context, userID int) ([]ContributionDetail, error) {
	query := `
		SELECT c.id, c.user_id, c.issue_id, COALESCE(c.pr_url, ''), COALESCE(c.pr_number, 0),
		       c.state, c.created_at::text, c.updated_at::text, COALESCE(c.merged_at::text, ''),
		       i.title, i.number, i.difficulty, i.url AS issue_url,
		       COALESCE(i.required_skills, '{}'), COALESCE(i.technologies, '{}'),
		       i.updated_at::text AS issue_updated_at, r.full_name AS repository, COALESCE(r.language, '')
		FROM contributions c
		JOIN issues i ON i.id = c.issue_id
		JOIN repositories r ON r.id = i.repository_id
		WHERE c.user_id = $1 ORDER BY c.updated_at DESC
	`
	var list []ContributionDetail
	err := s.db.QueryRows(ctx, query, []any{userID}, func(rows pgx.Rows) error {
		var item ContributionDetail
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.IssueID, &item.PRURL, &item.PRNumber,
			&item.State, &item.CreatedAt, &item.UpdatedAt, &item.MergedAt,
			&item.Title, &item.Number, &item.Difficulty, &item.IssueURL,
			&item.RequiredSkills, &item.Technologies, &item.IssueUpdatedAt,
			&item.Repository, &item.Language,
		); err != nil {
			return err
		}
		list = append(list, item)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("contribution: list: %w", err)
	}
	if list == nil {
		list = []ContributionDetail{}
	}
	return list, nil
}

// GetWorking returns active contributions with state = 'STARTED'.
func (s *Service) GetWorking(ctx context.Context, userID int) ([]ContributionDetail, error) {
	query := `
		SELECT c.id, c.user_id, c.issue_id, COALESCE(c.pr_url, ''), COALESCE(c.pr_number, 0),
		       c.state, c.created_at::text, c.updated_at::text, COALESCE(c.merged_at::text, ''),
		       i.title, i.number, i.difficulty, i.url AS issue_url,
		       COALESCE(i.required_skills, '{}'), COALESCE(i.technologies, '{}'),
		       r.full_name AS repository, r.url AS repository_url, COALESCE(r.language, '')
		FROM contributions c
		JOIN issues i ON i.id = c.issue_id
		JOIN repositories r ON r.id = i.repository_id
		WHERE c.user_id = $1 AND c.state = 'STARTED'
		ORDER BY c.updated_at DESC
	`
	var list []ContributionDetail
	err := s.db.QueryRows(ctx, query, []any{userID}, func(rows pgx.Rows) error {
		var item ContributionDetail
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.IssueID, &item.PRURL, &item.PRNumber,
			&item.State, &item.CreatedAt, &item.UpdatedAt, &item.MergedAt,
			&item.Title, &item.Number, &item.Difficulty, &item.IssueURL,
			&item.RequiredSkills, &item.Technologies,
			&item.Repository, &item.RepositoryURL, &item.Language,
		); err != nil {
			return err
		}
		list = append(list, item)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("contribution: get working: %w", err)
	}
	if list == nil {
		list = []ContributionDetail{}
	}
	return list, nil
}

// GetContribution returns a single contribution.
func (s *Service) GetContribution(ctx context.Context, userID, id int) (*Contribution, error) {
	var c Contribution
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id, issue_id, pr_url, pr_number, state, xp_awarded, created_at, updated_at, merged_at
		 FROM contributions WHERE id = $1 AND user_id = $2`,
		[]any{id, userID},
		func(row pgx.Row) error {
			return row.Scan(&c.ID, &c.UserID, &c.IssueID, &c.PRURL, &c.PRNumber, &c.State, &c.XPAwarded, &c.CreatedAt, &c.UpdatedAt, &c.MergedAt)
		},
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

var prURLRegex = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)/pull/(\d+)$`)

// AttachPR updates a contribution with an external PR URL and number after validating via GraphQL.
func (s *Service) AttachPR(ctx context.Context, userID, contribID int, prURL string, prNumber int, fallbackToken string) (*Contribution, error) {
	matches := prURLRegex.FindStringSubmatch(prURL)
	if len(matches) != 4 {
		return nil, fmt.Errorf("invalid PR URL format")
	}
	owner, repo, prNumStr := matches[1], matches[2], matches[3]
	num, _ := strconv.Atoi(prNumStr)
	if num != prNumber {
		return nil, fmt.Errorf("PR number mismatch")
	}

	// Fetch contribution and repository full_name
	var expectedRepo, username, userToken string
	err := s.db.QueryRow(ctx,
		`SELECT r.full_name, u.username, COALESCE(u.github_access_token, '')
		 FROM contributions c
		 JOIN issues i ON i.id = c.issue_id
		 JOIN repositories r ON r.id = i.repository_id
		 JOIN users u ON u.id = c.user_id
		 WHERE c.id = $1 AND c.user_id = $2`,
		[]any{contribID, userID},
		func(row pgx.Row) error {
			return row.Scan(&expectedRepo, &username, &userToken)
		},
	)
	if err != nil {
		return nil, db.ErrNotFound
	}
	userToken = s.decryptToken(userToken)

	// Case-insensitive repository match
	if strings.ToLower(fmt.Sprintf("%s/%s", owner, repo)) != strings.ToLower(expectedRepo) {
		return nil, fmt.Errorf("PR must belong to the challenge repository (%s)", expectedRepo)
	}

	token := userToken
	if token == "" {
		token = fallbackToken
	}

	// Verify PR exists on GitHub via GraphQL
	if token != "" && s.ghClient != nil {
		prInfo, err := s.ghClient.GetPullRequest(ctx, token, owner, repo, prNumber)
		if err != nil {
			return nil, fmt.Errorf("github PR validation failed: %w", err)
		}
		// Anti-exploit check: PR author must match the authenticated user
		if !strings.EqualFold(prInfo.AuthorLogin, username) {
			return nil, fmt.Errorf("PR author (%s) does not match authenticated user (%s)", prInfo.AuthorLogin, username)
		}
	}

	var updated Contribution
	err = s.db.QueryRow(ctx,
		`UPDATE contributions
		 SET pr_url = $1, pr_number = $2, state = 'PR_OPEN', updated_at = now()
		 WHERE id = $3 AND user_id = $4
		 RETURNING id, user_id, issue_id, pr_url, pr_number, state, xp_awarded, created_at, updated_at, merged_at`,
		[]any{prURL, prNumber, contribID, userID},
		func(row pgx.Row) error {
			return row.Scan(&updated.ID, &updated.UserID, &updated.IssueID, &updated.PRURL, &updated.PRNumber, &updated.State, &updated.XPAwarded, &updated.CreatedAt, &updated.UpdatedAt, &updated.MergedAt)
		},
	)
	if err != nil {
		return nil, err
	}

	return &updated, nil
}
