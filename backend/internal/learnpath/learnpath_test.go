package learnpath

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/issue"
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

func TestPathStartSwapAbandon(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	n := time.Now().UnixNano()
	lang := fmt.Sprintf("Lang%d", n)
	role := Role{Key: fmt.Sprintf("test-%d", n), Tech: []string{lang}}
	Roles = append(Roles, role)
	t.Cleanup(func() { Roles = Roles[:len(Roles)-1] })

	scan := func(dst *int) func(pgx.Row) error { return func(r pgx.Row) error { return r.Scan(dst) } }
	var userID int
	if err := pool.QueryRow(ctx, `INSERT INTO users (github_id, username) VALUES ($1, $1) RETURNING id`,
		[]any{fmt.Sprintf("path-%d", n)}, scan(&userID)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	// Six repositories of the role's language, each with a beginner and an
	// intermediate issue: the shuffled feed shows one per repository.
	for r := 0; r < 6; r++ {
		var repo int
		if err := pool.QueryRow(ctx,
			`INSERT INTO repositories (full_name, url, language, languages, shuffle_key) VALUES ($1, 'u', $2, $3, $4) RETURNING id`,
			[]any{fmt.Sprintf("path-test/%d-%d", n, r), lang, fmt.Sprintf(`[{"name":%q,"size":10}]`, lang), r},
			scan(&repo)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, repo); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		})
		if err := pool.Exec(ctx,
			`INSERT INTO issues (repository_id, number, title, url, difficulty, difficulty_score, updated_at) VALUES
			   ($1, 1, 't', 'u', 'BEGINNER', 2, now()), ($1, 2, 't', 'u', 'INTERMEDIATE', 5, now())`, repo); err != nil {
			t.Fatal(err)
		}
	}
	s := New(pool, issue.New(pool, nil))
	sizes := Sizes{Beginner: 3, Intermediate: 2, PickPages: 5}
	base := issue.Filters{ShuffleWalkMaxRepos: 100, ShuffleWalkBudget: 5 * time.Second, RareProbe: time.Second}

	if _, err := s.Start(ctx, userID, "no-such-role", sizes, base); !errors.Is(err, ErrUnknownRole) {
		t.Errorf("unknown role: %v", err)
	}
	p, err := s.Start(ctx, userID, role.Key, sizes, base)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"BEGINNER", "BEGINNER", "BEGINNER", "INTERMEDIATE", "INTERMEDIATE"}
	if len(p.Steps) != len(want) {
		t.Fatalf("steps = %d, want %d", len(p.Steps), len(want))
	}
	seen := map[int]bool{}
	for i, st := range p.Steps {
		if st.Difficulty != want[i] || st.Issue == nil || st.Issue.Difficulty != want[i] || seen[st.Issue.ID] {
			t.Errorf("step %d = %+v", i+1, st)
		}
		seen[st.Issue.ID] = true
	}
	if _, err := s.Start(ctx, userID, role.Key, sizes, base); !errors.Is(err, ErrHasPath) {
		t.Errorf("a second path: %v", err)
	}

	cur, err := s.Current(ctx, userID)
	if err != nil || cur.ID != p.ID || len(cur.Steps) != 5 || cur.Steps[0].Issue == nil {
		t.Fatalf("current = %+v, %v", cur, err)
	}
	first := cur.Steps[0].Issue.ID
	swapped, err := s.Swap(ctx, userID, 1, 5, base)
	if err != nil {
		t.Fatal(err)
	}
	if got := swapped.Steps[0].Issue; got == nil || got.ID == first || seen[got.ID] || got.Difficulty != "BEGINNER" {
		t.Errorf("swapped step 1 to %+v, want another beginner issue not on the path", got)
	}
	// Swapping again walks on: six beginner issues, three on the path, so two
	// more swaps of step 1 find new ones and the third finds none. One swapped
	// out is never offered again.
	seen[swapped.Steps[0].Issue.ID] = true
	for i := 0; i < 2; i++ {
		again, err := s.Swap(ctx, userID, 1, 5, base)
		if err != nil {
			t.Fatalf("swap %d: %v", i+2, err)
		}
		if got := again.Steps[0].Issue; got == nil || seen[got.ID] {
			t.Fatalf("swap %d gave %+v, an issue already offered", i+2, got)
		}
		seen[again.Steps[0].Issue.ID] = true
	}
	if _, err := s.Swap(ctx, userID, 1, 5, base); !errors.Is(err, ErrNoIssues) {
		t.Errorf("swapping with every other issue skipped: %v, want ErrNoIssues", err)
	}
	if err := pool.Exec(ctx, `UPDATE learning_path_steps SET done_at = now() WHERE path_id = $1 AND position = 2`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Swap(ctx, userID, 2, 5, base); !errors.Is(err, ErrStepDone) {
		t.Errorf("swapping a done step: %v", err)
	}
	if _, err := s.Swap(ctx, userID, 9, 5, base); !errors.Is(err, ErrNoStep) {
		t.Errorf("swapping a missing step: %v", err)
	}

	if err := s.Abandon(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Current(ctx, userID); !errors.Is(err, ErrNoPath) {
		t.Errorf("after abandoning: %v", err)
	}
	if err := s.Abandon(ctx, userID); !errors.Is(err, ErrNoPath) {
		t.Errorf("abandoning twice: %v", err)
	}
}
