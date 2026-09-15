// GitHub OAuth PKCE and PAT validation for SkillIssues.
// Uses GitHub GraphQL exclusively — no REST API calls.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// GitHubConfig holds OAuth app credentials and endpoint configuration.
type GitHubConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	GraphQLURL   string // default: https://api.github.com/graphql
}

// GitHubUser holds the data returned from GitHub for the authenticated user.
type GitHubUser struct {
	GitHubID   string // numeric database ID, stored as TEXT in users
	Username   string
	AvatarURL  string
}

// GitHubOAuth handles GitHub OAuth PKCE flow.
type GitHubOAuth struct {
	cfg GitHubConfig
	db  *Service
}

// NewGitHubOAuth creates a GitHubOAuth handler.
func NewGitHubOAuth(cfg GitHubConfig, authSvc *Service) *GitHubOAuth {
	return &GitHubOAuth{cfg: cfg, db: authSvc}
}

// ---------------------------------------------------------------------------
// PKCE OAuth flow
// ---------------------------------------------------------------------------

// BeginOAuth generates the OAuth authorization URL with PKCE S256 challenge.
// Stores the state and code_verifier in oauth_states table (expires in 10 minutes).
// Returns: authorizeURL, oauthState.
func (g *GitHubOAuth) BeginOAuth(ctx context.Context, redirectURI string) (string, string, error) {
	// Generate random state
	stateBuf := make([]byte, 32)
	if _, err := rand.Read(stateBuf); err != nil {
		return "", "", fmt.Errorf("github oauth: generate state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(stateBuf)

	// Generate PKCE code_verifier (random 32 bytes, base64url encoded)
	verifierBuf := make([]byte, 32)
	if _, err := rand.Read(verifierBuf); err != nil {
		return "", "", fmt.Errorf("github oauth: generate verifier: %w", err)
	}
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBuf)

	// Compute S256 code_challenge = base64url(sha256(code_verifier))
	h := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(h[:])

	// Store state + verifier in oauth_states (expires 10 min)
	err := g.db.db.Exec(ctx,
		`INSERT INTO oauth_states (state, code_verifier, redirect_uri, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		state, codeVerifier, redirectURI, time.Now().Add(10*time.Minute),
	)
	if err != nil {
		return "", "", fmt.Errorf("github oauth: store state: %w", err)
	}

	// Build GitHub authorize URL
	params := url.Values{
		"client_id":             {g.cfg.ClientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {"user:email,repo"},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	authorizeURL := "https://github.com/login/oauth/authorize?" + params.Encode()

	return authorizeURL, state, nil
}

// ExchangeOAuthCode exchanges a GitHub OAuth code for an access token using PKCE.
// Validates the state parameter against the stored value.
// Returns the GitHub access token and user info on success.
func (g *GitHubOAuth) ExchangeOAuthCode(ctx context.Context, code, state string) (string, *GitHubUser, error) {
	// Look up state and code_verifier
	var codeVerifier, storedRedirectURI string
	err := g.db.db.QueryRow(ctx,
		`DELETE FROM oauth_states
		 WHERE state = $1 AND expires_at > now()
		 RETURNING code_verifier, redirect_uri`,
		[]any{state},
		func(row pgx.Row) error {
			return row.Scan(&codeVerifier, &storedRedirectURI)
		},
	)
	if err != nil {
		return "", nil, fmt.Errorf("github oauth: invalid or expired state: %w", ErrUnauthorized)
	}

	// Exchange code for access token
	token, err := g.exchangeCode(ctx, code, codeVerifier, storedRedirectURI)
	if err != nil {
		return "", nil, fmt.Errorf("github oauth: exchange code: %w", err)
	}

	// Fetch user info via GraphQL
	user, err := g.GetAuthenticatedUser(ctx, token)
	if err != nil {
		return "", nil, fmt.Errorf("github oauth: get user: %w", err)
	}

	return token, user, nil
}

// exchangeCode sends the token exchange request to GitHub.
func (g *GitHubOAuth) exchangeCode(ctx context.Context, code, codeVerifier, redirectURI string) (string, error) {
	body := url.Values{
		"client_id":     {g.cfg.ClientID},
		"client_secret": {g.cfg.ClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {codeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://github.com/login/oauth/access_token",
		strings.NewReader(body.Encode()),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("github: %s", result.Error)
	}
	return result.AccessToken, nil
}

// ---------------------------------------------------------------------------
// PAT (Personal Access Token) validation
// ---------------------------------------------------------------------------

// ValidatePAT validates a GitHub Personal Access Token via GraphQL and returns user info.
// Used by POST /auth/github/token (Electron PAT login flow).
func (g *GitHubOAuth) ValidatePAT(ctx context.Context, pat string) (*GitHubUser, error) {
	return g.GetAuthenticatedUser(ctx, pat)
}

// GetAuthenticatedUser fetches GitHub user info using a token (PAT or OAuth).
// Uses GitHub GraphQL: { viewer { databaseId login avatarUrl } }
func (g *GitHubOAuth) GetAuthenticatedUser(ctx context.Context, token string) (*GitHubUser, error) {
	query := `{
		"query": "{ viewer { databaseId login avatarUrl } }"
	}`

	req, err := http.NewRequestWithContext(ctx, "POST", g.cfg.GraphQLURL,
		strings.NewReader(query))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github graphql: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("github: invalid token (HTTP %d)", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			Viewer struct {
				DatabaseID int    `json:"databaseId"`
				Login      string `json:"login"`
				AvatarURL  string `json:"avatarUrl"`
			} `json:"viewer"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("github graphql: parse response: %w", err)
	}
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("github graphql: %s", result.Errors[0].Message)
	}
	if result.Data.Viewer.Login == "" {
		return nil, fmt.Errorf("github graphql: empty viewer response")
	}

	return &GitHubUser{
		GitHubID:  fmt.Sprintf("%d", result.Data.Viewer.DatabaseID),
		Username:  result.Data.Viewer.Login,
		AvatarURL: result.Data.Viewer.AvatarURL,
	}, nil
}
