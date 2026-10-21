// GitHub sign-in for SkillIssues, through the Login app: an OAuth App that
// requests no scopes. It can only tell the server who the user is.
//
// The server reads the numeric GitHub id, login and avatar from a token it is
// handed and then discards that token. It never stores one, and nothing it
// holds can act on a user's GitHub account.
package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/httpx"
	"github.com/jackc/pgx/v5"
)

const defaultGitHubAPIURL = "https://api.github.com"

// GitHubConfig holds the Login app's credentials and endpoint configuration.
type GitHubConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	APIURL       string // default: https://api.github.com

	// HTTP sends every call to GitHub. Required: it carries the timeout, and
	// http.DefaultClient has none.
	HTTP httpx.Doer
}

// GitHubUser holds the data GitHub returns for the signed-in user.
type GitHubUser struct {
	GitHubID  string // numeric database ID, stored as TEXT in users
	Username  string
	// Name is the display name on the GitHub profile; empty when unset.
	Name      string
	AvatarURL string
}

// ErrInvalidDeviceToken means a device-flow token was not issued to the Login
// app, or GitHub no longer accepts it.
var ErrInvalidDeviceToken = errors.New("auth: token was not issued to the Login app")

// GitHubOAuth handles sign-in through the Login app.
type GitHubOAuth struct {
	cfg GitHubConfig
	db  *Service
}

// NewGitHubOAuth creates a GitHubOAuth handler.
func NewGitHubOAuth(cfg GitHubConfig, authSvc *Service) *GitHubOAuth {
	if cfg.APIURL == "" {
		cfg.APIURL = defaultGitHubAPIURL
	}
	if cfg.HTTP == nil {
		// Every call then fails with httpx.ErrNoTimeout instead of running
		// with no deadline.
		cfg.HTTP = httpx.New(nil, nil)
	}
	return &GitHubOAuth{cfg: cfg, db: authSvc}
}

// StateBinding derives the value of the browser-bound cookie that must
// accompany an OAuth callback. The callback is refused unless the cookie
// matches the state in its query string, so a callback URL replayed in
// another browser — or planted in a victim's — completes nothing.
func StateBinding(state string) string {
	h := sha256.Sum256([]byte(state))
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// PKCE OAuth flow
// ---------------------------------------------------------------------------

// BeginOAuth generates the OAuth authorization URL with a PKCE S256
// challenge and stores the state and code_verifier in oauth_states (expires in
// 10 minutes). It requests no scopes. Returns: authorizeURL, oauthState.
func (g *GitHubOAuth) BeginOAuth(ctx context.Context, redirectURI string) (string, string, error) {
	stateBuf := make([]byte, 32)
	if _, err := rand.Read(stateBuf); err != nil {
		return "", "", fmt.Errorf("github oauth: generate state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(stateBuf)

	verifierBuf := make([]byte, 32)
	if _, err := rand.Read(verifierBuf); err != nil {
		return "", "", fmt.Errorf("github oauth: generate verifier: %w", err)
	}
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBuf)

	h := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(h[:])

	err := g.db.db.Exec(ctx,
		`INSERT INTO oauth_states (state, code_verifier, redirect_uri, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		state, codeVerifier, redirectURI, time.Now().Add(10*time.Minute),
	)
	if err != nil {
		return "", "", fmt.Errorf("github oauth: store state: %w", err)
	}

	// No "scope": the default grants read access to public information only,
	// which is all identifying a user needs.
	params := url.Values{
		"client_id":             {g.cfg.ClientID},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	return "https://github.com/login/oauth/authorize?" + params.Encode(), state, nil
}

// ExchangeOAuthCode exchanges a GitHub OAuth code for the signed-in user's
// identity, validating the state parameter against the stored value. The
// access token GitHub returns is used once, to read the identity, and dropped.
func (g *GitHubOAuth) ExchangeOAuthCode(ctx context.Context, code, state string) (*GitHubUser, error) {
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
		return nil, fmt.Errorf("github oauth: invalid or expired state: %w", ErrUnauthorized)
	}

	token, err := g.exchangeCode(ctx, code, codeVerifier, storedRedirectURI)
	if err != nil {
		return nil, fmt.Errorf("github oauth: exchange code: %w", err)
	}

	user, err := g.viewer(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("github oauth: get user: %w", err)
	}
	return user, nil
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

	resp, err := g.cfg.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		if result.ErrorDescription != "" {
			return "", fmt.Errorf("github oauth: %s (%s)", result.Error, result.ErrorDescription)
		}
		return "", fmt.Errorf("github oauth: %s", result.Error)
	}
	if result.AccessToken == "" {
		return "", errors.New("github oauth: no access token in response")
	}
	return result.AccessToken, nil
}

// viewer reads the identity a token belongs to from GET /user.
func (g *GitHubOAuth) viewer(ctx context.Context, token string) (*GitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", g.cfg.APIURL+"/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := g.cfg.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: GET /user returned HTTP %d", resp.StatusCode)
	}
	var u struct {
		ID        int64   `json:"id"`
		Login     string  `json:"login"`
		Name      *string `json:"name"`
		AvatarURL string  `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("github: parse /user: %w", err)
	}
	return userFromGitHub(u.ID, u.Login, u.Name, u.AvatarURL)
}

func userFromGitHub(id int64, login string, name *string, avatarURL string) (*GitHubUser, error) {
	if id <= 0 || login == "" {
		return nil, ErrInvalidGitHubIdentity
	}
	u := &GitHubUser{GitHubID: strconv.FormatInt(id, 10), Username: login, AvatarURL: avatarURL}
	if name != nil {
		u.Name = strings.TrimSpace(*name)
	}
	return u, nil
}

// ---------------------------------------------------------------------------
// Device flow
// ---------------------------------------------------------------------------

// VerifyDeviceToken turns a token minted by the Login app's device flow into
// an identity. It asks GitHub to check the token against this app's client id
// (POST /applications/{client_id}/token, authenticated with the app's own
// credentials): GitHub answers only for a token this app issued, so a token
// from any other app, or a personal access token, is refused. That is what
// stops one app's token being presented here as a login. The token is not
// kept.
func (g *GitHubOAuth) VerifyDeviceToken(ctx context.Context, accessToken string) (*GitHubUser, error) {
	if accessToken == "" {
		return nil, ErrInvalidDeviceToken
	}
	payload, err := json.Marshal(map[string]string{"access_token": accessToken})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST",
		g.cfg.APIURL+"/applications/"+url.PathEscape(g.cfg.ClientID)+"/token",
		bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(g.cfg.ClientID, g.cfg.ClientSecret)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.cfg.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: check token: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound, http.StatusUnprocessableEntity:
		return nil, ErrInvalidDeviceToken
	default:
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("github: check token returned HTTP %d", resp.StatusCode)
	}

	var out struct {
		App struct {
			ClientID string `json:"client_id"`
		} `json:"app"`
		User struct {
			ID        int64   `json:"id"`
			Login     string  `json:"login"`
			Name      *string `json:"name"`
			AvatarURL string  `json:"avatar_url"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("github: parse token check: %w", err)
	}
	// Belt and braces: the endpoint is already scoped to our client id.
	if out.App.ClientID != g.cfg.ClientID {
		return nil, ErrInvalidDeviceToken
	}
	return userFromGitHub(out.User.ID, out.User.Login, out.User.Name, out.User.AvatarURL)
}
