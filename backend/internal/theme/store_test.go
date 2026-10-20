package theme

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
)

func storePool(t *testing.T) *db.Pool {
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
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func testUser(t *testing.T, pool *db.Pool) int {
	t.Helper()
	login := fmt.Sprintf("theme-test-%d", time.Now().UnixNano())
	var id int
	if err := pool.QueryRow(context.Background(), `INSERT INTO users (github_id, username) VALUES ($1, $1) RETURNING id`,
		[]any{login}, func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return id
}

const okTheme = "[meta]\nname = \"Paper\"\n[colors]\naccent = \"#ff0000\"\n"

func TestThemeStore(t *testing.T) {
	pool := storePool(t)
	svc := NewService(pool)
	ctx := context.Background()
	lim := Limits{MaxPerUser: 2, MaxBytes: 4096}
	author, other := testUser(t, pool), testUser(t, pool)

	// Invalid input is refused with its problems, before anything is stored.
	if _, err := svc.Create(ctx, author, Input{TOML: "[colors]\naccent = \"url(x)\"\n"}, lim); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid TOML: %v", err)
	}
	if _, err := svc.Create(ctx, author, Input{TOML: okTheme, CSS: "@import url(https://evil.example/x);"}, lim); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid CSS: %v", err)
	}
	if _, err := svc.Create(ctx, author, Input{TOML: okTheme, CSS: string(make([]byte, 5000))}, lim); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized: %v", err)
	}

	priv, err := svc.Create(ctx, author, Input{TOML: okTheme}, lim)
	if err != nil || priv.Name != "Paper" || !priv.Mine || priv.Public {
		t.Fatalf("create: %+v, %v", priv, err)
	}
	pub, err := svc.Create(ctx, author, Input{TOML: "[colors]\naccent = \"#000\"\n", Public: true}, lim)
	if err != nil || pub.Name != "Untitled" {
		t.Fatalf("create public: %+v, %v", pub, err)
	}
	if _, err := svc.Create(ctx, author, Input{TOML: okTheme}, lim); !errors.Is(err, ErrTooMany) {
		t.Errorf("third theme past a cap of 2: %v", err)
	}

	// A private theme is invisible to anyone else; a public one is not.
	if _, err := svc.Get(ctx, other, priv.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("other user read a private theme: %v", err)
	}
	if _, err := svc.Get(ctx, 0, priv.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("signed out read a private theme: %v", err)
	}
	if got, err := svc.Get(ctx, 0, pub.ID); err != nil || got.Mine {
		t.Errorf("signed out read of a public theme: %+v, %v", got, err)
	}
	if _, err := svc.Get(ctx, other, "not-a-uuid"); !errors.Is(err, ErrNotFound) {
		t.Errorf("malformed id: %v", err)
	}

	// Applying: own or public only.
	if err := svc.SetActive(ctx, other, priv.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("applied someone's private theme: %v", err)
	}
	if err := svc.SetActive(ctx, other, pub.ID); err != nil {
		t.Fatal(err)
	}
	if a, err := svc.Active(ctx, other); err != nil || a == nil || a.ID != pub.ID {
		t.Fatalf("active = %+v, %v", a, err)
	}
	// Only the author edits, and making it private takes it off others.
	if _, err := svc.Update(ctx, other, pub.ID, Input{TOML: okTheme, Public: true}, lim); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-author update: %v", err)
	}
	if _, err := svc.Update(ctx, author, pub.ID, Input{TOML: okTheme}, lim); err != nil {
		t.Fatal(err)
	}
	if a, err := svc.Active(ctx, other); err != nil || a != nil {
		t.Errorf("a theme made private stayed applied for another account: %+v, %v", a, err)
	}

	if err := svc.SetActive(ctx, author, priv.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, other, priv.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-author delete: %v", err)
	}
	if err := svc.Delete(ctx, author, priv.ID); err != nil {
		t.Fatal(err)
	}
	if a, err := svc.Active(ctx, author); err != nil || a != nil {
		t.Errorf("deleted theme still applied: %+v, %v", a, err)
	}
	if list, err := svc.List(ctx, author); err != nil || len(list) != 1 {
		t.Errorf("list after delete: %d themes, %v", len(list), err)
	}
}
