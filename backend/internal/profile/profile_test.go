package profile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

func testPool(t *testing.T) *db.Pool {
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
	return pool
}

// GET /me says when the account was created, for the profile's "joined on".
func TestFullProfileJoinedAt(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	login := fmt.Sprintf("profile-test-%d", time.Now().UnixNano())
	created := time.Date(2026, 9, 20, 8, 30, 0, 0, time.UTC)
	var id int
	if err := pool.QueryRow(ctx, `INSERT INTO users (github_id, username, created_at) VALUES ($1, $1, $2) RETURNING id`,
		[]any{login, created}, func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	p, err := New(pool).GetFullProfile(ctx, id)
	if err != nil {
		t.Fatalf("GetFullProfile: %v", err)
	}
	if p.JoinedAt != "2026-09-20T08:30:00Z" {
		t.Errorf("joined_at = %q, want 2026-09-20T08:30:00Z", p.JoinedAt)
	}
}

// An update is stored whole, creating a missing preferences row, or refused
// whole with ErrInvalid: never trimmed or defaulted.
func TestUpdateProfile(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	login := fmt.Sprintf("profile-update-%d", time.Now().UnixNano())
	var id int
	if err := pool.QueryRow(ctx, `INSERT INTO users (github_id, username) VALUES ($1, $1) RETURNING id`,
		[]any{login}, func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	svc := New(pool)
	lim := Limits{BioMaxChars: 10, MaxLanguages: 2, MaxInterests: 2}

	// No preferences row yet: the update creates it.
	p, err := svc.UpdateProfile(ctx, id, ProfileUpdate{
		Bio: " hi ", ExperienceLevel: "advanced", TargetLevel: "ADVANCED",
		PreferredLanguages: []string{"Go", " go ", "", "Rust"}, Interests: []string{"cli"},
	}, lim)
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if p.Bio != "hi" || p.ExperienceLevel != "ADVANCED" || len(p.PreferredLanguages) != 2 || p.PreferredLanguages[1] != "Rust" || len(p.Interests) != 1 {
		t.Errorf("stored %+v", p)
	}

	for name, bad := range map[string]ProfileUpdate{
		"unknown level":    {ExperienceLevel: "guru", TargetLevel: "ADVANCED"},
		"long bio":         {Bio: "far too long a bio", ExperienceLevel: "BEGINNER", TargetLevel: "ADVANCED"},
		"many languages":   {ExperienceLevel: "BEGINNER", TargetLevel: "ADVANCED", PreferredLanguages: []string{"a", "b", "c"}},
		"markup in a name": {ExperienceLevel: "BEGINNER", TargetLevel: "ADVANCED", Interests: []string{"<b>"}},
	} {
		if _, err := svc.UpdateProfile(ctx, id, bad, lim); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
	if p, err := svc.GetFullProfile(ctx, id); err != nil || p.Bio != "hi" {
		t.Errorf("a refused update changed the profile: %+v, %v", p, err)
	}
}

// Only an operator makes an account a moderator, in the database; the flag
// is false until then, and an unknown account isn't one.
func TestModerator(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	login := fmt.Sprintf("mod-test-%d", time.Now().UnixNano())
	var id int
	if err := pool.QueryRow(ctx, `INSERT INTO users (github_id, username) VALUES ($1, $1) RETURNING id`,
		[]any{login}, func(row pgx.Row) error { return row.Scan(&id) }); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	svc := New(pool)
	if mod, err := svc.IsModerator(ctx, id); err != nil || mod {
		t.Fatalf("a new account: moderator %v, %v", mod, err)
	}
	if err := pool.Exec(ctx, `UPDATE users SET moderator = true WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if mod, err := svc.IsModerator(ctx, id); err != nil || !mod {
		t.Errorf("after the operator's update: moderator %v, %v", mod, err)
	}
	if p, err := svc.GetFullProfile(ctx, id); err != nil || !p.Moderator {
		t.Errorf("GET /me: moderator %v, %v", p != nil && p.Moderator, err)
	}
	if mod, err := svc.IsModerator(ctx, -1); err != nil || mod {
		t.Errorf("an unknown account: moderator %v, %v", mod, err)
	}
}
