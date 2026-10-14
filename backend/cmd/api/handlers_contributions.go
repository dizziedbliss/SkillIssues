package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
		issueID, issueNum         int
		username, title, repoFull string
	)
	err = h.db.QueryRow(r.Context(),
		`SELECT c.issue_id, u.username,
		        i.title, i.number, r.full_name
		 FROM contributions c
		 JOIN users u ON u.id = c.user_id
		 JOIN issues i ON i.id = c.issue_id
		 JOIN repositories r ON r.id = i.repository_id
		 WHERE c.id = $1 AND c.user_id = $2`,
		[]any{id, userID},
		func(row pgx.Row) error {
			return row.Scan(&issueID, &username, &title, &issueNum, &repoFull)
		},
	)
	if errors.Is(err, db.ErrNotFound) {
		middleware.RespondError(w, http.StatusNotFound, "contribution not found")
		return
	}
	if err != nil {
		slog.Error("submit contribution: load contribution", "contribution_id", id, "user_id", userID, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "failed to load contribution")
		return
	}

	// GetAccessToken, not a raw decrypt: a GitHub App user token expires
	// after 8 hours and only GetAccessToken refreshes it.
	token, ok := h.userGitHubToken(w, r, userID, "Connect GitHub before submitting a pull request.")
	if !ok {
		return
	}

	owner, name, _ := strings.Cut(repoFull, "/")
	prTitle := title
	prBody := fmt.Sprintf("SkillIssues contribution for issue #%d", issueNum)

	// The PR's base is the repository's default branch unless the client
	// named one. A failed lookup is an error, not a guess: a wrong base makes
	// the PR (or the compare page) target a branch that may not exist.
	baseBranch := strings.TrimSpace(body.Base)
	if baseBranch == "" {
		defBranch, err := h.ghClient.GetDefaultBranch(r.Context(), token, owner, name)
		if err != nil || defBranch == "" {
			slog.Error("submit contribution: look up default branch", "contribution_id", id, "repository", repoFull, "error", err)
			middleware.RespondError(w, http.StatusBadGateway, "Could not read the repository's default branch from GitHub. Please try again.")
			return
		}
		baseBranch = defBranch
	}

	head := body.Branch
	if username != "" {
		head = fmt.Sprintf("%s:%s", username, body.Branch)
	}
	res, err := h.ghClient.CreatePullRequest(r.Context(), token, owner, name, prTitle, head, baseBranch, prBody)
	if err != nil {
		// Typically a token without permission to open PRs on the upstream
		// (classic PATs). Hand the user GitHub's compare page so they can
		// open it themselves. The contribution is not marked PR_OPEN: there
		// is no PR yet, and a compare URL could never be verified or matched
		// by the merge webhook. They attach the real PR with
		// PUT /contributions/{id} once it exists.
		slog.Warn("submit contribution: create pull request failed; returning compare page", "contribution_id", id, "repository", repoFull, "error", err)
		compareURL := fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s?expand=1", owner, name, baseBranch, head)
		middleware.RespondJSON(w, http.StatusOK, map[string]any{
			"pull_request": map[string]any{
				"html_url":       compareURL,
				"base":           baseBranch,
				"manual_compare": true,
			},
			"web_url": compareURL,
		})
		return
	}

	var updated contribution.Contribution
	err = h.db.QueryRow(r.Context(),
		`UPDATE contributions
		 SET pr_url = $1, pr_number = $2, state = 'PR_OPEN', updated_at = now(),
		     submitted_at = COALESCE(submitted_at, now())
		 WHERE id = $3
		 RETURNING id, user_id, issue_id, pr_url, pr_number, state, xp_awarded, created_at, updated_at, merged_at, submitted_at`,
		[]any{res.HTMLURL, res.Number, id},
		func(row pgx.Row) error {
			return row.Scan(&updated.ID, &updated.UserID, &updated.IssueID, &updated.PRURL, &updated.PRNumber, &updated.State, &updated.XPAwarded, &updated.CreatedAt, &updated.UpdatedAt, &updated.MergedAt, &updated.SubmittedAt)
		},
	)
	if err != nil {
		// The PR exists on GitHub; only our record of it failed.
		slog.Error("submit contribution: record pull request", "contribution_id", id, "pr_url", res.HTMLURL, "error", err)
		middleware.RespondError(w, http.StatusInternalServerError, "The pull request was opened, but recording it failed: "+res.HTMLURL)
		return
	}

	prURL := res.HTMLURL
	pullRequest := map[string]any{"html_url": prURL, "number": res.Number, "base": baseBranch}
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
