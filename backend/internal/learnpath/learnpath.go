// Package learnpath is learning paths: pick a role, get a short run of catalog
// issues from beginner to intermediate, and finish them one merge at a time.
// go-api starts, swaps and abandons paths; ingestd marks steps done when a
// merged pull request of the person's closes one, and pays the finished path
// (internal/ingest/prs.go).
package learnpath

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/issue"
)

// Role is a kind of work a path is made of: repositories using any of Tech,
// or issues carrying any of Labels.
type Role struct {
	Key    string   `json:"key"`
	Tech   []string `json:"tech,omitempty"`
	Labels []string `json:"labels,omitempty"`
}

// Roles are the paths on offer.
var Roles = []Role{
	{Key: "frontend", Tech: []string{"TypeScript", "JavaScript", "Vue", "Svelte", "CSS", "HTML"}},
	{Key: "backend", Tech: []string{"Go", "Python", "Java", "Rust", "Ruby", "PHP", "C#", "Kotlin", "Elixir"}},
	{Key: "docs", Labels: []string{"documentation", "docs"}},
	{Key: "devops", Tech: []string{"Shell", "Dockerfile", "HCL", "Nix", "Makefile"}},
	{Key: "data", Tech: []string{"Python", "Jupyter Notebook", "R", "SQL", "Julia"}},
	{Key: "mobile", Tech: []string{"Kotlin", "Swift", "Dart", "Objective-C"}},
}

func roleOf(key string) (Role, bool) {
	for _, r := range Roles {
		if r.Key == key {
			return r, true
		}
	}
	return Role{}, false
}

// Errors a path request can be refused with.
var (
	ErrUnknownRole = errors.New("learnpath: no such role")
	ErrHasPath     = errors.New("learnpath: you already have a path in progress")
	ErrNoPath      = errors.New("learnpath: no path in progress")
	ErrNoIssues    = errors.New("learnpath: no issues fit right now")
	ErrStepDone    = errors.New("learnpath: that step is done")
	ErrNoStep      = errors.New("learnpath: no such step")
)

// Path is a person's path in progress.
type Path struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"`
	StartedAt time.Time `json:"started_at"`
	Steps     []Step    `json:"steps"`
}

// Step is one issue of a path.
type Step struct {
	Position   int          `json:"position"`
	Difficulty string       `json:"difficulty"`
	DoneAt     *time.Time   `json:"done_at"`
	Issue      *issue.Issue `json:"issue"` // nil once the catalog dropped it
}

// Sizes is how many issues of each difficulty a new path takes.
type Sizes struct {
	Beginner, Intermediate int
	PickPages              int // shuffled-feed pages read per difficulty
}

// Service reads and writes paths.
type Service struct {
	db     *db.Pool
	issues *issue.Service
}

// New builds a Service.
func New(pool *db.Pool, issues *issue.Service) *Service { return &Service{db: pool, issues: issues} }

func (s *Service) pick(ctx context.Context, base issue.Filters, r Role, difficulty string, n, pages int, exclude map[int]bool) ([]issue.Issue, error) {
	f := base
	f.Tech, f.Labels, f.Difficulties = r.Tech, r.Labels, []string{difficulty}
	return s.issues.Pick(ctx, f, n, pages, exclude)
}

// Start begins a path of role for userID. base carries the catalog's walk
// settings.
func (s *Service) Start(ctx context.Context, userID int, roleKey string, sz Sizes, base issue.Filters) (Path, error) {
	r, ok := roleOf(roleKey)
	if !ok {
		return Path{}, ErrUnknownRole
	}
	if _, err := s.Current(ctx, userID); err == nil {
		return Path{}, ErrHasPath
	} else if !errors.Is(err, ErrNoPath) {
		return Path{}, err
	}
	exclude := map[int]bool{}
	var steps []Step
	for _, part := range []struct {
		difficulty string
		n          int
	}{{"BEGINNER", sz.Beginner}, {"INTERMEDIATE", sz.Intermediate}} {
		found, err := s.pick(ctx, base, r, part.difficulty, part.n, sz.PickPages, exclude)
		if err != nil {
			return Path{}, fmt.Errorf("learnpath: pick %s issues: %w", part.difficulty, err)
		}
		for i := range found {
			steps = append(steps, Step{Position: len(steps) + 1, Difficulty: part.difficulty, Issue: &found[i]})
		}
	}
	if len(steps) == 0 {
		return Path{}, ErrNoIssues
	}
	var p Path
	err := s.db.InTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `INSERT INTO learning_paths (user_id, role) VALUES ($1, $2) RETURNING id, role, started_at`,
			userID, r.Key).Scan(&p.ID, &p.Role, &p.StartedAt)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrHasPath // started twice at once
		}
		if err != nil {
			return fmt.Errorf("learnpath: create: %w", err)
		}
		for _, st := range steps {
			if _, err := tx.Exec(ctx, `INSERT INTO learning_path_steps (path_id, position, issue_id, difficulty) VALUES ($1, $2, $3, $4)`,
				p.ID, st.Position, st.Issue.ID, st.Difficulty); err != nil {
				return fmt.Errorf("learnpath: add step: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return Path{}, err
	}
	p.Steps = steps
	return p, nil
}

// Current is userID's path in progress, with its issues as they stand.
func (s *Service) Current(ctx context.Context, userID int) (Path, error) {
	var p Path
	err := s.db.QueryRow(ctx,
		`SELECT id, role, started_at FROM learning_paths WHERE user_id = $1 AND finished_at IS NULL AND abandoned_at IS NULL`,
		[]any{userID}, func(r pgx.Row) error { return r.Scan(&p.ID, &p.Role, &p.StartedAt) })
	if errors.Is(err, db.ErrNotFound) {
		return Path{}, ErrNoPath
	}
	if err != nil {
		return Path{}, fmt.Errorf("learnpath: read: %w", err)
	}
	type row struct {
		pos     int
		diff    string
		done    *time.Time
		issueID *int
	}
	var rows []row
	err = s.db.QueryRows(ctx,
		`SELECT position, difficulty, done_at, issue_id FROM learning_path_steps WHERE path_id = $1 ORDER BY position`,
		[]any{p.ID}, func(r pgx.Rows) error {
			var x row
			if err := r.Scan(&x.pos, &x.diff, &x.done, &x.issueID); err != nil {
				return err
			}
			rows = append(rows, x)
			return nil
		})
	if err != nil {
		return Path{}, fmt.Errorf("learnpath: read steps: %w", err)
	}
	p.Steps = []Step{}
	for _, x := range rows {
		st := Step{Position: x.pos, Difficulty: x.diff, DoneAt: x.done}
		if x.issueID != nil {
			it, err := s.issues.GetIssue(ctx, *x.issueID)
			if err != nil && !errors.Is(err, db.ErrNotFound) {
				return Path{}, fmt.Errorf("learnpath: read step issue: %w", err)
			}
			st.Issue = it
		}
		p.Steps = append(p.Steps, st)
	}
	return p, nil
}

// Swap replaces a step that isn't done with another issue of its difficulty.
func (s *Service) Swap(ctx context.Context, userID, position, pickPages int, base issue.Filters) (Path, error) {
	p, err := s.Current(ctx, userID)
	if err != nil {
		return Path{}, err
	}
	r, ok := roleOf(p.Role)
	if !ok {
		return Path{}, ErrUnknownRole
	}
	var step *Step
	// Everything on the path now, and everything swapped out of it before.
	exclude := map[int]bool{}
	err = s.db.QueryRows(ctx, `SELECT issue_id FROM learning_path_skips WHERE path_id = $1`, []any{p.ID},
		func(r pgx.Rows) error {
			var id int
			if err := r.Scan(&id); err != nil {
				return err
			}
			exclude[id] = true
			return nil
		})
	if err != nil {
		return Path{}, fmt.Errorf("learnpath: read skipped issues: %w", err)
	}
	for i := range p.Steps {
		if p.Steps[i].Issue != nil {
			exclude[p.Steps[i].Issue.ID] = true
		}
		if p.Steps[i].Position == position {
			step = &p.Steps[i]
		}
	}
	if step == nil {
		return Path{}, ErrNoStep
	}
	if step.DoneAt != nil {
		return Path{}, ErrStepDone
	}
	found, err := s.pick(ctx, base, r, step.Difficulty, 1, pickPages, exclude)
	if err != nil {
		return Path{}, fmt.Errorf("learnpath: pick a replacement: %w", err)
	}
	if len(found) == 0 {
		return Path{}, ErrNoIssues
	}
	err = s.db.InTx(ctx, func(tx pgx.Tx) error {
		if step.Issue != nil {
			if _, err := tx.Exec(ctx, `INSERT INTO learning_path_skips (path_id, issue_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				p.ID, step.Issue.ID); err != nil {
				return fmt.Errorf("learnpath: remember the skipped issue: %w", err)
			}
		}
		tag, err := tx.Exec(ctx, `UPDATE learning_path_steps SET issue_id = $3 WHERE path_id = $1 AND position = $2 AND done_at IS NULL`,
			p.ID, position, found[0].ID)
		if err != nil {
			return fmt.Errorf("learnpath: swap: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrStepDone // finished between the read and now
		}
		return nil
	})
	if err != nil {
		return Path{}, err
	}
	step.Issue = &found[0]
	return p, nil
}

// Abandon gives up userID's path in progress.
func (s *Service) Abandon(ctx context.Context, userID int) error {
	var done bool
	err := s.db.QueryRow(ctx,
		`WITH a AS (UPDATE learning_paths SET abandoned_at = now()
		            WHERE user_id = $1 AND finished_at IS NULL AND abandoned_at IS NULL RETURNING 1)
		 SELECT EXISTS (SELECT 1 FROM a)`, []any{userID}, func(r pgx.Row) error { return r.Scan(&done) })
	if err != nil {
		return fmt.Errorf("learnpath: abandon: %w", err)
	}
	if !done {
		return ErrNoPath
	}
	return nil
}
