package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dizziedbliss/skillissues/internal/contribution"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/middleware"
	"github.com/jackc/pgx/v5"
)

func (h *handlers) startContribution(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var body struct {
		IssueID int `json:"issue_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.IssueID <= 0 {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "valid issue_id is required")
		return
	}

	c, err := h.contribSvc.StartContribution(r.Context(), userID, body.IssueID)
	if err != nil {
		if err == db.ErrNotFound {
			middleware.RespondError(w, http.StatusNotFound, "issue not found")
			return
		}
		middleware.RespondError(w, http.StatusInternalServerError, "failed to start contribution")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, c)
}

func (h *handlers) listContributions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	list, err := h.contribSvc.ListContributions(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to list contributions")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, list)
}

func (h *handlers) getWorking(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	working, err := h.contribSvc.GetWorking(r.Context(), userID)
	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, "failed to list working contributions")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, working)
}

func (h *handlers) getContribution(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid contribution id")
		return
	}
	c, err := h.contribSvc.GetContribution(r.Context(), userID, id)
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "contribution not found")
		return
	}
	middleware.RespondJSON(w, http.StatusOK, c)
}

func (h *handlers) updateContribution(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid contribution id")
		return
	}
	var body struct {
		PRURL    string `json:"pr_url"`
		PRNumber int    `json:"pr_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PRURL == "" || body.PRNumber <= 0 {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "pr_url and pr_number are required")
		return
	}

	c, err := h.contribSvc.AttachPR(r.Context(), userID, id, body.PRURL, body.PRNumber, h.cfg.GitHubToken)
	if err != nil {
		middleware.RespondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	middleware.RespondJSON(w, http.StatusOK, c)
}

func (h *handlers) submitContribution(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid contribution id")
		return
	}
	var body struct {
		Branch string `json:"branch"`
		Base   string `json:"base"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Branch == "" {
		middleware.RespondError(w, http.StatusUnprocessableEntity, "branch name is required")
		return
	}

	var (
		issueID, issueNum                    int
		username, userToken, title, repoFull string
	)
	err = h.db.QueryRow(r.Context(),
		`SELECT c.issue_id, u.username, COALESCE(u.github_access_token, ''),
		        i.title, i.number, r.full_name
		 FROM contributions c
		 JOIN users u ON u.id = c.user_id
		 JOIN issues i ON i.id = c.issue_id
		 JOIN repositories r ON r.id = i.repository_id
		 WHERE c.id = $1 AND c.user_id = $2`,
		[]any{id, userID},
		func(row pgx.Row) error {
			return row.Scan(&issueID, &username, &userToken, &title, &issueNum, &repoFull)
		},
	)
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "contribution not found")
		return
	}

	token := h.authSvc.DecryptAccessToken(userToken)
	if token == "" {
		middleware.RespondError(w, http.StatusForbidden, "Connect GitHub before submitting a pull request.")
		return
	}

	owner, name, _ := strings.Cut(repoFull, "/")
	prTitle := title
	prBody := fmt.Sprintf("SkillIssues contribution for issue #%d", issueNum)

	// Dynamically determine target repository default branch (e.g. 'main', 'master', 'develop')
	baseBranch := strings.TrimSpace(body.Base)
	if baseBranch == "" {
		if defBranch, err := h.ghClient.GetDefaultBranch(r.Context(), token, owner, name); err == nil && defBranch != "" {
			baseBranch = defBranch
		} else {
			baseBranch = "main"
		}
	}

	var prURL string
	var prNum int
	var pullRequest map[string]any

	{
		head := body.Branch
		if username != "" {
			head = fmt.Sprintf("%s:%s", username, body.Branch)
		}
		res, err := h.ghClient.CreatePullRequest(r.Context(), token, owner, name, prTitle, head, baseBranch, prBody)
		if err == nil {
			prURL = res.HTMLURL
			prNum = res.Number
			pullRequest = map[string]any{"html_url": prURL, "number": prNum, "base": baseBranch}
		}
	}

	if prURL == "" {
		if username != "" {
			prURL = fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s:%s?expand=1", owner, name, baseBranch, username, body.Branch)
		} else {
			prURL = fmt.Sprintf("https://github.com/%s/%s", owner, name)
		}
		prNum = issueNum
		pullRequest = map[string]any{
			"html_url":       prURL,
			"number":         prNum,
			"base":           baseBranch,
			"manual_compare": true,
		}
	}

	var updated contribution.Contribution
	_ = h.db.QueryRow(r.Context(),
		`UPDATE contributions SET pr_url = $1, pr_number = $2, state = 'PR_OPEN', updated_at = now()
		 WHERE id = $3
		 RETURNING id, user_id, issue_id, pr_url, pr_number, state, xp_awarded, created_at, updated_at, merged_at`,
		[]any{prURL, prNum, id},
		func(row pgx.Row) error {
			return row.Scan(&updated.ID, &updated.UserID, &updated.IssueID, &updated.PRURL, &updated.PRNumber, &updated.State, &updated.XPAwarded, &updated.CreatedAt, &updated.UpdatedAt, &updated.MergedAt)
		},
	)

	middleware.RespondJSON(w, http.StatusOK, map[string]any{
		"contribution": updated,
		"pull_request": pullRequest,
		"web_url":      prURL,
	})
}

func (h *handlers) verifyContribution(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		middleware.RespondError(w, http.StatusNotFound, "invalid contribution id")
		return
	}

	res, err := h.contribSvc.VerifyContribution(r.Context(), userID, id, h.cfg.GitHubToken)
	if err != nil {
		if err == db.ErrNotFound {
			middleware.RespondError(w, http.StatusNotFound, "contribution not found")
			return
		}
		middleware.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if res.Verified && res.EarnedXP > 0 && h.cache != nil {
		h.cache.InvalidateRecommendations(r.Context(), userID)
	}

	middleware.RespondJSON(w, http.StatusOK, res)
}
