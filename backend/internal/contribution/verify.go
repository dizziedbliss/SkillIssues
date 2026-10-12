package contribution

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/progression"
	"github.com/jackc/pgx/v5"
)

// VerifyResult contains the outcome of a PR verification attempt.
type VerifyResult struct {
	Verified       bool                   `json:"verified"`
	State          string                 `json:"state"`
	AlreadyAwarded bool                   `json:"already_awarded,omitempty"`
	EarnedXP       int                    `json:"earned_xp"`
	TotalXP        int                    `json:"total_xp"`
	OldLevel       int                    `json:"old_level"`
	Level          int                    `json:"level"`
	LevelUp        bool                   `json:"level_up"`
	UnlockedBadges []string               `json:"unlocked_badges"`
	AllBadges      []string               `json:"all_badges"`
	Badges         []string               `json:"badges"`
	NextLevelXP    int                    `json:"next_level_xp"`
	ProgressInLvl  int                    `json:"progress_in_level"`
	ProgressPct    int                    `json:"progress_percent"`
	Message        string                 `json:"message"`
	XPInfo         map[string]any         `json:"xp_info,omitempty"`
}

// VerifyContribution verifies PR merge status on GitHub using GraphQL and awards XP idempotently.
func (s *Service) VerifyContribution(ctx context.Context, userID, contribID int, fallbackToken string) (*VerifyResult, error) {
	// 1. Anti-exploit: cap verification attempts per user per contribution per hour.
	if s.cache != nil {
		quota := s.verifyAttemptsPerHour()
		rlKey := fmt.Sprintf("verify:%d:%d", userID, contribID)
		res := s.cache.CheckRateLimit(ctx, rlKey, quota, time.Hour)
		if !res.Allowed {
			return &VerifyResult{
				Verified: false,
				Message: fmt.Sprintf(
					"Rate limit exceeded: maximum %d verification attempts per hour for this challenge.", quota),
			}, nil
		}
	}

	// 2. Fetch contribution and repository info
	var (
		cID, issueID, prNum, repoID int
		prURL, state, difficulty, username, userToken string
		skills, techs []string
		xpAwarded bool
	)

	err := s.db.QueryRow(ctx,
		`SELECT c.id, c.issue_id, COALESCE(c.pr_url, ''), COALESCE(c.pr_number, 0),
		        c.state, c.xp_awarded, i.difficulty, COALESCE(i.required_skills, '{}'),
		        COALESCE(i.technologies, '{}'), i.repository_id, u.username,
		        COALESCE(u.github_access_token, '')
		 FROM contributions c
		 JOIN issues i ON i.id = c.issue_id
		 JOIN users u ON u.id = c.user_id
		 WHERE c.id = $1 AND c.user_id = $2`,
		[]any{contribID, userID},
		func(row pgx.Row) error {
			return row.Scan(
				&cID, &issueID, &prURL, &prNum, &state, &xpAwarded,
				&difficulty, &skills, &techs, &repoID, &username, &userToken,
			)
		},
	)
	if err != nil {
		return nil, db.ErrNotFound
	}
	userToken = s.decryptToken(userToken)

	if prURL == "" {
		return &VerifyResult{
			Verified: false,
			Message:  "Connect GitHub and submit a valid pull request URL to verify status.",
		}, nil
	}

	// Parse PR URL
	u, err := url.Parse(prURL)
	if err != nil {
		return &VerifyResult{Verified: false, Message: "Invalid pull request URL format."}, nil
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "pull" {
		return &VerifyResult{Verified: false, Message: "Invalid pull request URL structure."}, nil
	}
	owner, repo := parts[0], parts[1]
	number, _ := strconv.Atoi(parts[3])

	token := userToken
	if token == "" {
		token = fallbackToken
	}
	if token == "" {
		return &VerifyResult{
			Verified: false,
			Message:  "Connect GitHub before verifying pull request status.",
		}, nil
	}

	// 3. Query GitHub GraphQL for PR details
	var merged bool
	var prState string
	var prAuthor string

	if s.ghClient != nil && token != "" {
		prInfo, err := s.ghClient.GetPullRequest(ctx, token, owner, repo, number)
		if err != nil {
			return nil, fmt.Errorf("github PR lookup failed: %w", err)
		}
		merged = prInfo.Merged
		prState = strings.ToLower(prInfo.State)
		prAuthor = prInfo.AuthorLogin
	}

	// Anti-exploit check: PR author match (case-insensitive)
	if !strings.EqualFold(prAuthor, username) {
		return &VerifyResult{
			Verified: false,
			Message:  fmt.Sprintf("PR author (@%s) does not match your GitHub login (@%s).", prAuthor, username),
		}, nil
	}

	if !merged {
		newState := "PR_OPEN"
		if prState != "open" {
			newState = "PR_CLOSED"
		}
		_ = s.db.Exec(ctx, "UPDATE contributions SET state = $1, updated_at = now() WHERE id = $2", newState, contribID)
		return &VerifyResult{
			Verified: false,
			State:    prState,
			Message:  "GitHub confirms the pull request is not merged yet.",
		}, nil
	}

	// 4. Server-Authoritative Progression Transaction
	var res VerifyResult
	err = s.db.InTx(ctx, func(tx pgx.Tx) error {
		// Lock contribution row to prevent race conditions
		var currentXPAwarded bool
		err := tx.QueryRow(ctx,
			"SELECT xp_awarded FROM contributions WHERE id = $1 FOR UPDATE",
			cID,
		).Scan(&currentXPAwarded)
		if err != nil {
			return err
		}

		// Fetch current profile stats
		var currentXP, currentLvl int
		var currentBadges []string
		_ = tx.QueryRow(ctx,
			`SELECT COALESCE(total_xp, 0), COALESCE(level, 1), COALESCE(badges, '{}')
			 FROM developer_profiles WHERE user_id = $1 FOR UPDATE`,
			userID,
		).Scan(&currentXP, &currentLvl, &currentBadges)

		if currentXPAwarded {
			// Idempotent: already rewarded, return current level stats
			lvlInfo := progression.CalculateLevel(currentXP)
			res = VerifyResult{
				Verified:       true,
				State:          "PR_MERGED",
				AlreadyAwarded: true,
				EarnedXP:       0,
				TotalXP:        currentXP,
				OldLevel:       currentLvl,
				Level:          lvlInfo.Level,
				LevelUp:        false,
				UnlockedBadges: []string{},
				AllBadges:      currentBadges,
				Badges:         currentBadges,
				NextLevelXP:    lvlInfo.NextLevelXP,
				ProgressInLvl:  lvlInfo.ProgressInLevel,
				ProgressPct:    lvlInfo.ProgressPercent,
				Message:        fmt.Sprintf("PR is merged! Current Level: %d (%d XP)", lvlInfo.Level, currentXP),
			}
			return nil
		}

		// Calculate novelty count: prior merged PRs by user in this repository
		var priorRepoMerged int
		_ = tx.QueryRow(ctx,
			`SELECT count(*) FROM contributions c
			 JOIN issues i ON i.id = c.issue_id
			 WHERE c.user_id = $1 AND c.state = 'PR_MERGED' AND i.repository_id = $2`,
			userID, repoID,
		).Scan(&priorRepoMerged)

		_, _, earnedXP := progression.CalculateEarnedXP(difficulty, priorRepoMerged, len(skills), len(techs))
		newXP := currentXP + earnedXP
		newLvlInfo := progression.CalculateLevel(newXP)
		newLevel := newLvlInfo.Level
		levelUp := newLevel > currentLvl

		// Update contribution to PR_MERGED and mark awarded
		_, err = tx.Exec(ctx,
			`UPDATE contributions
			 SET state = 'PR_MERGED', merged_at = COALESCE(merged_at, now()),
			     xp_awarded = true, updated_at = now()
			 WHERE id = $1`,
			cID,
		)
		if err != nil {
			return err
		}

		// Re-evaluate badges
		var totalMerged, totalRepos, totalLangs int
		_ = tx.QueryRow(ctx,
			`SELECT 
				count(c.id) AS total_merged,
				count(DISTINCT r.id) AS total_repos,
				count(DISTINCT lower(r.language)) AS total_languages
			FROM contributions c
			JOIN issues i ON i.id = c.issue_id
			JOIN repositories r ON r.id = i.repository_id
			WHERE c.user_id = $1 AND c.state = 'PR_MERGED'`,
			userID,
		).Scan(&totalMerged, &totalRepos, &totalLangs)

		earnedBadges := progression.EvaluateBadges(progression.StatsForBadges{
			TotalMerged:    totalMerged,
			TotalRepos:     totalRepos,
			TotalLanguages: totalLangs,
		})

		oldBadgeMap := make(map[string]bool)
		for _, b := range currentBadges {
			oldBadgeMap[b] = true
		}
		var unlocked []string
		for _, b := range earnedBadges {
			if !oldBadgeMap[b] {
				unlocked = append(unlocked, b)
			}
		}

		// Update developer profile
		_, err = tx.Exec(ctx,
			`UPDATE developer_profiles SET
			   completed_count = $1,
			   progress_score = $2,
			   total_xp = $3,
			   level = $4,
			   badges = $5
			 WHERE user_id = $6`,
			totalMerged, newLvlInfo.ProgressPercent, newXP, newLevel, earnedBadges, userID,
		)
		if err != nil {
			return err
		}

		// 5. Anti-exploit audit log entry
		_, _ = tx.Exec(ctx,
			`INSERT INTO contribution_audit_log (contribution_id, user_id, from_state, to_state, event, metadata)
			 VALUES ($1, $2, $3, 'PR_MERGED', 'VERIFIED_MERGED', $4)`,
			cID, userID, state,
			fmt.Sprintf(`{"earned_xp": %d, "total_xp": %d, "pr_url": "%s"}`, earnedXP, newXP, prURL),
		)

		msg := fmt.Sprintf("+%d XP!", earnedXP)
		if levelUp {
			msg += " LEVEL UP!"
		}

		res = VerifyResult{
			Verified:       true,
			State:          "PR_MERGED",
			AlreadyAwarded: false,
			EarnedXP:       earnedXP,
			TotalXP:        newXP,
			OldLevel:       currentLvl,
			Level:          newLevel,
			LevelUp:        levelUp,
			UnlockedBadges: unlocked,
			AllBadges:      earnedBadges,
			Badges:         earnedBadges,
			NextLevelXP:    newLvlInfo.NextLevelXP,
			ProgressInLvl:  newLvlInfo.ProgressInLevel,
			ProgressPct:    newLvlInfo.ProgressPercent,
			Message:        msg,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("verify contribution tx: %w", err)
	}

	res.XPInfo = map[string]any{
		"merged":             true,
		"earned_xp":          res.EarnedXP,
		"total_xp":           res.TotalXP,
		"level":              res.Level,
		"level_up":           res.LevelUp,
		"unlocked_badges":    res.UnlockedBadges,
		"message":            res.Message,
	}

	return &res, nil
}

// ApplyMergedPRByUrl applies merge XP when triggered via GitHub Webhook.
func (s *Service) ApplyMergedPRByUrl(ctx context.Context, prURL string, fallbackToken string) (*VerifyResult, error) {
	var cID, userID int
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id FROM contributions
		 WHERE pr_url = $1 OR lower(rtrim(pr_url, '/')) = lower(rtrim($1, '/'))`,
		[]any{prURL},
		func(row pgx.Row) error {
			return row.Scan(&cID, &userID)
		},
	)
	if err != nil {
		return nil, db.ErrNotFound
	}
	return s.VerifyContribution(ctx, userID, cID, fallbackToken)
}
