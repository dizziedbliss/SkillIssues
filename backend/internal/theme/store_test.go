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

// A built-in applies as it is, and an account has one of a built-in or a
// stored theme at a time: choosing either clears the other.
func TestActivePreset(t *testing.T) {
	pool := storePool(t)
	ctx := context.Background()
	svc := NewService(pool)
	user := testUser(t, pool)
	lim := Limits{MaxPerUser: 5, MaxBytes: 4096}

	if err := svc.SetActivePreset(ctx, user, "not-a-preset"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown preset: err = %v, want ErrNotFound", err)
	}
	if err := svc.SetActivePreset(ctx, user, "nord"); err != nil {
		t.Fatal(err)
	}
	if p, err := svc.ActivePreset(ctx, user); err != nil || p != "nord" {
		t.Errorf("ActivePreset = %q, %v; want nord", p, err)
	}

	mine, err := svc.Create(ctx, user, Input{TOML: "[colors]\naccent = \"#123456\"\n"}, lim)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetActive(ctx, user, mine.ID); err != nil {
		t.Fatal(err)
	}
	if p, err := svc.ActivePreset(ctx, user); err != nil || p != "" {
		t.Errorf("after a stored theme, ActivePreset = %q, %v; want none", p, err)
	}
	if err := svc.SetActivePreset(ctx, user, "dracula"); err != nil {
		t.Fatal(err)
	}
	if a, err := svc.Active(ctx, user); err != nil || a != nil {
		t.Errorf("after a built-in, Active = %+v, %v; want none", a, err)
	}
	if err := svc.SetActive(ctx, user, ""); err != nil {
		t.Fatal(err)
	}
	if p, _ := svc.ActivePreset(ctx, user); p != "" {
		t.Errorf("the default left the built-in %q applied", p)
	}
}

// Check reports the file's and the stylesheet's problems together.
func TestCheckReportsEverything(t *testing.T) {
	_, err := Check(Input{TOML: "[colors]\nacent = \"#000\"\n", CSS: "@import \"x\";"}, Limits{MaxBytes: 4096})
	var p Problems
	if !errors.As(err, &p) || len(p) != 2 {
		t.Errorf("Check = %v, want a TOML and a CSS problem", err)
	}
}

// The community listing: public themes only, most used first, searchable,
// without the ones reports have hidden.
func TestListPublic(t *testing.T) {
	pool := storePool(t)
	ctx := context.Background()
	svc := NewService(pool)
	lim := Limits{MaxPerUser: 5, MaxBytes: 4096}
	author, fan := testUser(t, pool), testUser(t, pool)
	word := fmt.Sprintf("zq%d", time.Now().UnixNano())
	mk := func(name string, public bool) Theme {
		th, err := svc.Create(ctx, author, Input{TOML: fmt.Sprintf("[meta]\nname = %q\n", name+" "+word), Public: public}, lim)
		if err != nil {
			t.Fatal(err)
		}
		return th
	}
	quiet, loved, _ := mk("Quiet", true), mk("Loved", true), mk("Hidden away", false)
	if err := svc.SetActive(ctx, fan, loved.ID); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ListPublic(ctx, 0, "popular", word, 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != loved.ID || got[0].Uses != 1 || got[1].ID != quiet.ID {
		t.Errorf("popular = %+v, want Loved (1 use) then Quiet, and no private theme", got)
	}
	got, err = svc.ListPublic(ctx, 0, "popular", word, 10, 0, map[string]bool{loved.ID: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != quiet.ID {
		t.Errorf("with Loved hidden = %+v, want Quiet alone", got)
	}
	if got, err := svc.ListPublic(ctx, 0, "newest", "50%_"+word, 10, 0, nil); err != nil || len(got) != 0 {
		t.Errorf("a search's %% and _ are literal: %+v, %v", got, err)
	}
}
