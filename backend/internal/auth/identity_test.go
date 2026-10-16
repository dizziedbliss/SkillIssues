package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

func testService(t *testing.T) (*Service, *db.Pool) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := db.New(context.Background(), db.DefaultConfig(url))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return New(pool), pool
}

func userRow(t *testing.T, pool *db.Pool, id int) (githubID string, username *string) {
	t.Helper()
	if err := pool.QueryRow(context.Background(),
		`SELECT github_id, username FROM users WHERE id = $1`, []any{id},
		func(row pgx.Row) error { return row.Scan(&githubID, &username) }); err != nil {
		t.Fatalf("read user %d: %v", id, err)
	}
	return githubID, username
}

// Two accounts swapping usernames must never merge: the second sign-in with
// a login the first account used to hold cannot take that account over.
func TestUpsertUserNeverMatchesByUsername(t *testing.T) {
	svc, pool := testService(t)
	ctx := context.Background()
	stamp := time.Now().UnixNano()
	idA, idB := fmt.Sprintf("910%d", stamp), fmt.Sprintf("920%d", stamp)
	loginA, loginB := fmt.Sprintf("alice-%d", stamp), fmt.Sprintf("bob-%d", stamp)
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM users WHERE github_id = ANY($1)`, []string{idA, idB}); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	userA, err := svc.UpsertUser(ctx, idA, loginA, "")
	if err != nil {
		t.Fatal(err)
	}
	// Alice renames; Bob takes her old login. Bob signs in.
	if _, err := svc.UpsertUser(ctx, idA, loginA+"-renamed", ""); err != nil {
		t.Fatal(err)
	}
	userB, err := svc.UpsertUser(ctx, idB, loginA, "")
	if err != nil {
		t.Fatal(err)
	}
	if userB == userA {
		t.Fatal("bob signed in as alice's account")
	}
	if gid, name := userRow(t, pool, userB); gid != idB || name == nil || *name != loginA {
		t.Errorf("bob row = %s %v", gid, name)
	}

	// A login collision with a *different* live row clears that row's
	// username instead of failing or merging.
	if _, err := svc.UpsertUser(ctx, idA, loginB, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpsertUser(ctx, idB, loginB, ""); err != nil {
		t.Fatal(err)
	}
	if gid, name := userRow(t, pool, userA); gid != idA || name != nil {
		t.Errorf("alice row after losing her login = %s %v, want id kept and username NULL", gid, name)
	}
	if _, name := userRow(t, pool, userB); name == nil || *name != loginB {
		t.Errorf("bob username = %v, want %s", name, loginB)
	}
}

func TestUpsertUserRejectsMissingIdentity(t *testing.T) {
	svc, _ := testService(t)
	for _, id := range []string{"", "0"} {
		if _, err := svc.UpsertUser(context.Background(), id, "x", ""); !errors.Is(err, ErrInvalidGitHubIdentity) {
			t.Errorf("github id %q: err = %v, want ErrInvalidGitHubIdentity", id, err)
		}
	}
}

func fakeGitHub(t *testing.T, handler http.HandlerFunc) *GitHubOAuth {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewGitHubOAuth(GitHubConfig{ClientID: "login-app", ClientSecret: "s3cret", APIURL: srv.URL}, New(nil))
}

func TestVerifyDeviceToken(t *testing.T) {
	g := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if r.Method != http.MethodPost || r.URL.Path != "/applications/login-app/token" || !ok || user != "login-app" || pass != "s3cret" {
			t.Errorf("unexpected request %s %s basic=%v/%s", r.Method, r.URL.Path, ok, user)
		}
		var body struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		switch body.AccessToken {
		case "good":
			fmt.Fprint(w, `{"app":{"client_id":"login-app"},"user":{"id":42,"login":"octo","avatar_url":"a"}}`)
		case "other-app":
			fmt.Fprint(w, `{"app":{"client_id":"someone-else"},"user":{"id":42,"login":"octo"}}`)
		case "no-id":
			fmt.Fprint(w, `{"app":{"client_id":"login-app"},"user":{"id":0,"login":"octo"}}`)
		default:
			http.NotFound(w, r)
		}
	})

	u, err := g.VerifyDeviceToken(context.Background(), "good")
	if err != nil || u.GitHubID != "42" || u.Username != "octo" {
		t.Fatalf("good token: %+v %v", u, err)
	}
	if _, err := g.VerifyDeviceToken(context.Background(), "unknown"); !errors.Is(err, ErrInvalidDeviceToken) {
		t.Errorf("unknown token: %v, want ErrInvalidDeviceToken", err)
	}
	if _, err := g.VerifyDeviceToken(context.Background(), "other-app"); !errors.Is(err, ErrInvalidDeviceToken) {
		t.Errorf("token from another app: %v, want ErrInvalidDeviceToken", err)
	}
	if _, err := g.VerifyDeviceToken(context.Background(), "no-id"); !errors.Is(err, ErrInvalidGitHubIdentity) {
		t.Errorf("token with no id: %v, want ErrInvalidGitHubIdentity", err)
	}
	if _, err := g.VerifyDeviceToken(context.Background(), ""); !errors.Is(err, ErrInvalidDeviceToken) {
		t.Errorf("empty token: %v, want ErrInvalidDeviceToken", err)
	}
}

func TestVerifyDeviceTokenSurfacesGitHubOutage(t *testing.T) {
	g := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) })
	_, err := g.VerifyDeviceToken(context.Background(), "t")
	if err == nil || errors.Is(err, ErrInvalidDeviceToken) {
		t.Errorf("err = %v; an outage must not read as an invalid token", err)
	}
}

func TestStateBindingDiffersPerState(t *testing.T) {
	if StateBinding("a") == StateBinding("b") || StateBinding("a") != StateBinding("a") {
		t.Fatal("StateBinding must be deterministic and distinct per state")
	}
}
