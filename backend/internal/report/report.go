// Package report stores what signed-in people flag (an issue, a repository or
// a public theme), works out which targets enough people have reported to
// take out of lists until someone looks, and resolves them.
//
// Reports are private. Only the admin endpoints read them; the person who
// reported never sees anyone else's report, and nothing about a report is
// shown on the target.
package report

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

// Types of target, and the reasons each may be reported for.
const (
	Issue      = "issue"
	Repository = "repository"
	Theme      = "theme"
)

// Reasons per target type, as the API takes them.
var Reasons = map[string][]string{
	Issue:      {"spam", "abuse", "already-fixed", "not-an-issue", "wrong-difficulty", "other"},
	Repository: {"spam", "malware", "abandoned", "other"},
	Theme:      {"abuse", "misleading", "broken", "other"},
}

// maxNoteRunes is the longest note a report carries: a sentence or two.
const maxNoteRunes = 500

var (
	// ErrInvalid wraps a refused report (type, reason, note), with the reason.
	ErrInvalid = errors.New("report: invalid")
	// ErrNotFound is a target that doesn't exist, or a theme that isn't public.
	ErrNotFound = errors.New("report: no such target")
	// ErrDuplicate is a second open report of the same target by the same person.
	ErrDuplicate = errors.New("report: already reported")
	// ErrTooMany is past api.reports.per_user_per_day.
	ErrTooMany = errors.New("report: too many reports today")
)

// Target is what a report is about.
type Target struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Service stores and resolves reports.
type Service struct{ db *db.Pool }

// NewService returns a report store over pool.
func NewService(pool *db.Pool) *Service { return &Service{db: pool} }

// exists says whether a target can be reported: a stored issue or
// repository, or a public theme.
func (s *Service) exists(ctx context.Context, t Target) (bool, error) {
	var q string
	switch t.Type {
	case Issue:
		q = `SELECT EXISTS (SELECT 1 FROM issues WHERE id = $1::int)`
	case Repository:
		q = `SELECT EXISTS (SELECT 1 FROM repositories WHERE id = $1::int)`
	case Theme:
		q = `SELECT EXISTS (SELECT 1 FROM user_themes WHERE id = $1::uuid AND public)`
	}
	var ok bool
	err := s.db.QueryRow(ctx, q, []any{t.ID}, func(row pgx.Row) error { return row.Scan(&ok) })
	return ok, err
}

// validTarget checks a target's shape before it reaches SQL: a positive
// number for an issue or repository, a UUID for a theme.
func validTarget(t Target) bool {
	switch t.Type {
	case Issue, Repository:
		n, err := strconv.Atoi(t.ID)
		return err == nil && n > 0
	case Theme:
		return uuidLike(t.ID)
	}
	return false
}

func uuidLike(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
				return false
			}
		}
	}
	return true
}

// Create records a report. perDay is api.reports.per_user_per_day.
func (s *Service) Create(ctx context.Context, userID int, t Target, reason, note string, perDay int) error {
	if _, ok := Reasons[t.Type]; !ok {
		return fmt.Errorf("%w: type must be issue, repository or theme", ErrInvalid)
	}
	if !slices.Contains(Reasons[t.Type], reason) {
		return fmt.Errorf("%w: a %s can be reported for %s", ErrInvalid, t.Type, strings.Join(Reasons[t.Type], ", "))
	}
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > maxNoteRunes {
		return fmt.Errorf("%w: the note is longer than %d characters", ErrInvalid, maxNoteRunes)
	}
	if strings.ContainsRune(note, 0) {
		return fmt.Errorf("%w: the note holds a null character", ErrInvalid)
	}
	if !validTarget(t) {
		return ErrNotFound
	}
	ok, err := s.exists(ctx, t)
	if err != nil {
		return fmt.Errorf("report: look up target: %w", err)
	}
	if !ok {
		return ErrNotFound
	}

	return s.db.InTx(ctx, func(tx pgx.Tx) error {
		// Serialise one person's reports so the daily count can't be raced.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('report:' || $1::int::text))`, userID); err != nil {
			return fmt.Errorf("report: lock: %w", err)
		}
		var today int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM reports WHERE user_id = $1 AND created_at > now() - interval '24 hours'`, userID).Scan(&today); err != nil {
			return fmt.Errorf("report: count today: %w", err)
		}
		if today >= perDay {
			return ErrTooMany
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO reports (user_id, target_type, target_id, reason, note) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (user_id, target_type, target_id) WHERE status = 'open' DO NOTHING`,
			userID, t.Type, t.ID, reason, note)
		if err != nil {
			return fmt.Errorf("report: insert: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrDuplicate
		}
		return nil
	})
}

// Hidden is what lists leave out: targets reported by at least the threshold
// of different people and not yet reviewed, and targets whose reports were
// upheld.
type Hidden struct {
	Issues       map[int]bool
	Repositories map[int]bool
	Themes       map[string]bool
}

// Hidden reads the hidden targets. hideAfter is api.reports.hide_after.
func (s *Service) Hidden(ctx context.Context, hideAfter int) (Hidden, error) {
	h := Hidden{Issues: map[int]bool{}, Repositories: map[int]bool{}, Themes: map[string]bool{}}
	err := s.db.QueryRows(ctx, `
		SELECT target_type, target_id FROM reports
		WHERE status IN ('open', 'upheld')
		GROUP BY target_type, target_id
		HAVING bool_or(status = 'upheld') OR count(DISTINCT user_id) FILTER (WHERE status = 'open') >= $1`,
		[]any{hideAfter}, func(rows pgx.Rows) error {
			var typ, id string
			if err := rows.Scan(&typ, &id); err != nil {
				return err
			}
			switch typ {
			case Issue, Repository:
				n, err := strconv.Atoi(id)
				if err != nil {
					return fmt.Errorf("report: stored %s id %q is not a number", typ, id)
				}
				if typ == Issue {
					h.Issues[n] = true
				} else {
					h.Repositories[n] = true
				}
			case Theme:
				h.Themes[id] = true
			}
			return nil
		})
	if err != nil {
		return Hidden{}, fmt.Errorf("report: hidden: %w", err)
	}
	return h, nil
}

// Summary is one reported target, for review.
type Summary struct {
	Target
	Label     string         `json:"label"`
	Reporters int            `json:"reporters"`
	Reasons   map[string]int `json:"reasons"`
	Notes     []string       `json:"notes"`
	First     time.Time      `json:"first_reported"`
	Last      time.Time      `json:"last_reported"`
}

// Open lists targets with open reports, most reported first.
func (s *Service) Open(ctx context.Context) ([]Summary, error) {
	out := []Summary{}
	err := s.db.QueryRows(ctx, `
		SELECT r.target_type, r.target_id,
		       COALESCE(CASE r.target_type
		         WHEN 'issue' THEN (SELECT rp.full_name || '#' || i.number || ' ' || i.title FROM issues i JOIN repositories rp ON rp.id = i.repository_id WHERE i.id::text = r.target_id)
		         WHEN 'repository' THEN (SELECT full_name FROM repositories WHERE id::text = r.target_id)
		         WHEN 'theme' THEN (SELECT name FROM user_themes WHERE id::text = r.target_id)
		       END, '(gone)'),
		       count(DISTINCT r.user_id),
		       array_agg(r.reason), array_remove(array_agg(NULLIF(r.note, '')), NULL),
		       min(r.created_at), max(r.created_at)
		FROM reports r WHERE r.status = 'open'
		GROUP BY r.target_type, r.target_id
		ORDER BY count(DISTINCT r.user_id) DESC, max(r.created_at) DESC`, nil,
		func(rows pgx.Rows) error {
			var s Summary
			var reasons []string
			if err := rows.Scan(&s.Type, &s.ID, &s.Label, &s.Reporters, &reasons, &s.Notes, &s.First, &s.Last); err != nil {
				return err
			}
			s.Reasons = map[string]int{}
			for _, r := range reasons {
				s.Reasons[r]++
			}
			if s.Notes == nil {
				s.Notes = []string{}
			}
			out = append(out, s)
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("report: open: %w", err)
	}
	return out, nil
}

// Resolve closes every open report of a target. Upholding acts on it: a
// repository is excluded from the crawl, a theme made private (and so taken
// off every other account), an issue kept hidden. Dismissing brings it back.
// It reports ErrNotFound when the target has no open reports.
func (s *Service) Resolve(ctx context.Context, t Target, uphold bool) error {
	status := "dismissed"
	if uphold {
		status = "upheld"
	}
	return s.db.InTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE reports SET status = $3, resolved_at = now()
			WHERE target_type = $1 AND target_id = $2 AND status = 'open'`, t.Type, t.ID, status)
		if err != nil {
			return fmt.Errorf("report: resolve: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if !uphold {
			// A target dismissed now isn't held hidden by an older uphold either.
			if _, err := tx.Exec(ctx, `UPDATE reports SET status = 'dismissed' WHERE target_type = $1 AND target_id = $2 AND status = 'upheld'`, t.Type, t.ID); err != nil {
				return fmt.Errorf("report: dismiss earlier: %w", err)
			}
			return nil
		}
		switch t.Type {
		case Repository:
			if _, err := tx.Exec(ctx, `INSERT INTO excluded_repositories (full_name, reason)
				SELECT full_name, 'reported, upheld on review' FROM repositories WHERE id::text = $1
				ON CONFLICT ((lower(full_name))) DO NOTHING`, t.ID); err != nil {
				return fmt.Errorf("report: exclude repository: %w", err)
			}
		case Theme:
			if _, err := tx.Exec(ctx, `UPDATE user_themes SET public = false, updated_at = now() WHERE id::text = $1`, t.ID); err != nil {
				return fmt.Errorf("report: make theme private: %w", err)
			}
			// As a private theme is everywhere else: off every account but its author's.
			if _, err := tx.Exec(ctx, `UPDATE users SET active_theme_id = NULL
				WHERE active_theme_id::text = $1 AND id <> (SELECT user_id FROM user_themes WHERE id::text = $1)`, t.ID); err != nil {
				return fmt.Errorf("report: take theme off accounts: %w", err)
			}
		}
		return nil
	})
}
