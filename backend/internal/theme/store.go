package theme

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
)

// Theme is a stored theme as the API returns it.
type Theme struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	TOML      string    `json:"toml"`
	CSS       string    `json:"css"`
	Public    bool      `json:"public"`
	Author    string    `json:"author"`
	Mine      bool      `json:"mine"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Input is a theme as an author saves it.
type Input struct {
	TOML   string `json:"toml"`
	CSS    string `json:"css"`
	Public bool   `json:"public"`
}

// Limits are the per-request values of api.themes.*.
type Limits struct {
	MaxPerUser int
	MaxBytes   int
}

var (
	// ErrNotFound is a theme that does not exist or that the caller may not
	// see: a private theme is indistinguishable from a missing one.
	ErrNotFound = errors.New("theme: not found")
	// ErrTooMany is a create past api.themes.max_per_user.
	ErrTooMany = errors.New("theme: too many themes")
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Service stores themes.
type Service struct{ db *db.Pool }

// NewService returns a theme store over pool.
func NewService(pool *db.Pool) *Service { return &Service{db: pool} }

// Check validates a theme without storing it (POST /themes/validate, and
// every save) and returns its name: meta.name, or "Untitled". Problems in
// the file and in the stylesheet come back together, so an author fixes
// both in one pass.
func Check(in Input, lim Limits) (string, error) {
	if size := len(in.TOML) + len(in.CSS); size > lim.MaxBytes {
		return "", Problems{fmt.Sprintf("the theme is %d bytes; the limit is %d", size, lim.MaxBytes)}
	}
	tok, tomlErr := ParseTOML(in.TOML)
	cssErr := CheckCSS(in.CSS)
	var all Problems
	for _, err := range []error{tomlErr, cssErr} {
		var p Problems
		if errors.As(err, &p) {
			all = append(all, p...)
		} else if err != nil {
			return "", err
		}
	}
	if len(all) > 0 {
		return "", all
	}
	name := tok.Name()
	if name == "" {
		name = "Untitled"
	}
	return name, nil
}

const themeSelect = `
	SELECT t.id::text, t.name, t.toml, t.css, t.public, COALESCE(u.username, ''), t.user_id = $1,
	       t.created_at, t.updated_at
	FROM user_themes t JOIN users u ON u.id = t.user_id`

func scanTheme(row pgx.Row) (Theme, error) {
	var t Theme
	err := row.Scan(&t.ID, &t.Name, &t.TOML, &t.CSS, &t.Public, &t.Author, &t.Mine, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

// List returns the caller's themes, most recently changed first.
func (s *Service) List(ctx context.Context, userID int) ([]Theme, error) {
	out := []Theme{}
	err := s.db.QueryRows(ctx, themeSelect+` WHERE t.user_id = $1 ORDER BY t.updated_at DESC`, []any{userID},
		func(rows pgx.Rows) error {
			t, err := scanTheme(rows)
			out = append(out, t)
			return err
		})
	if err != nil {
		return nil, fmt.Errorf("theme: list: %w", err)
	}
	return out, nil
}

// Get returns a theme the viewer owns, or any public one. viewerID is 0 for
// a signed-out caller.
func (s *Service) Get(ctx context.Context, viewerID int, id string) (Theme, error) {
	if !uuidRe.MatchString(id) {
		return Theme{}, ErrNotFound
	}
	var t Theme
	err := s.db.QueryRow(ctx, themeSelect+` WHERE t.id = $2 AND (t.public OR t.user_id = $1)`, []any{viewerID, id},
		func(row pgx.Row) (err error) { t, err = scanTheme(row); return err })
	if errors.Is(err, db.ErrNotFound) {
		return Theme{}, ErrNotFound
	}
	if err != nil {
		return Theme{}, fmt.Errorf("theme: get: %w", err)
	}
	return t, nil
}

// Create validates and stores a theme for the caller.
func (s *Service) Create(ctx context.Context, userID int, in Input, lim Limits) (Theme, error) {
	name, err := Check(in, lim)
	if err != nil {
		return Theme{}, err
	}
	var id string
	err = s.db.InTx(ctx, func(tx pgx.Tx) error {
		// Serialise one account's creates, so two at once cannot both pass
		// the count.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, userID); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM user_themes WHERE user_id = $1`, userID).Scan(&n); err != nil {
			return err
		}
		if n >= lim.MaxPerUser {
			return ErrTooMany
		}
		return tx.QueryRow(ctx,
			`INSERT INTO user_themes (user_id, name, toml, css, public) VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
			userID, name, in.TOML, in.CSS, in.Public).Scan(&id)
	})
	if errors.Is(err, ErrTooMany) {
		return Theme{}, err
	}
	if err != nil {
		return Theme{}, fmt.Errorf("theme: create: %w", err)
	}
	return s.Get(ctx, userID, id)
}

// Update replaces one of the caller's themes. Making a theme private takes
// it off every other account that had applied it.
func (s *Service) Update(ctx context.Context, userID int, id string, in Input, lim Limits) (Theme, error) {
	if !uuidRe.MatchString(id) {
		return Theme{}, ErrNotFound
	}
	name, err := Check(in, lim)
	if err != nil {
		return Theme{}, err
	}
	err = s.db.InTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE user_themes SET name = $3, toml = $4, css = $5, public = $6, updated_at = now()
			 WHERE id = $2 AND user_id = $1`, userID, id, name, in.TOML, in.CSS, in.Public)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if !in.Public {
			_, err = tx.Exec(ctx, `UPDATE users SET active_theme_id = NULL WHERE active_theme_id = $1 AND id <> $2`, id, userID)
		}
		return err
	})
	if errors.Is(err, ErrNotFound) {
		return Theme{}, err
	}
	if err != nil {
		return Theme{}, fmt.Errorf("theme: update: %w", err)
	}
	return s.Get(ctx, userID, id)
}

// Delete removes one of the caller's themes; accounts that had applied it
// fall back to the default (ON DELETE SET NULL).
func (s *Service) Delete(ctx context.Context, userID int, id string) error {
	if !uuidRe.MatchString(id) {
		return ErrNotFound
	}
	var deleted bool
	err := s.db.QueryRow(ctx,
		`WITH d AS (DELETE FROM user_themes WHERE id = $2 AND user_id = $1 RETURNING 1) SELECT count(*) > 0 FROM d`,
		[]any{userID, id}, func(row pgx.Row) error { return row.Scan(&deleted) })
	if err != nil {
		return fmt.Errorf("theme: delete: %w", err)
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

// SetActive applies a theme to the caller's account: their own or a public
// one. An empty id goes back to the default.
func (s *Service) SetActive(ctx context.Context, userID int, id string) error {
	if id == "" {
		if err := s.db.Exec(ctx, `UPDATE users SET active_theme_id = NULL, active_preset = NULL WHERE id = $1`, userID); err != nil {
			return fmt.Errorf("theme: clear active: %w", err)
		}
		return nil
	}
	if !uuidRe.MatchString(id) {
		return ErrNotFound
	}
	var set bool
	err := s.db.QueryRow(ctx, `
		WITH u AS (
		  UPDATE users SET active_theme_id = t.id, active_preset = NULL FROM user_themes t
		  WHERE users.id = $1 AND t.id = $2 AND (t.public OR t.user_id = $1)
		  RETURNING 1
		) SELECT count(*) > 0 FROM u`, []any{userID, id}, func(row pgx.Row) error { return row.Scan(&set) })
	if err != nil {
		return fmt.Errorf("theme: set active: %w", err)
	}
	if !set {
		return ErrNotFound
	}
	return nil
}

// SetActivePreset applies a built-in theme (one of Presets) as it is.
func (s *Service) SetActivePreset(ctx context.Context, userID int, preset string) error {
	if !slices.Contains(Presets, preset) {
		return ErrNotFound
	}
	var n int64
	err := s.db.QueryRow(ctx, `WITH u AS (UPDATE users SET active_theme_id = NULL, active_preset = $2 WHERE id = $1 RETURNING 1)
		SELECT count(*) FROM u`, []any{userID, preset}, func(row pgx.Row) error { return row.Scan(&n) })
	if err != nil {
		return fmt.Errorf("theme: set active preset: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("theme: set active preset: no user %d", userID)
	}
	return nil
}

// ActivePreset is the built-in the caller has applied as it is, or "".
func (s *Service) ActivePreset(ctx context.Context, userID int) (string, error) {
	var preset *string
	err := s.db.QueryRow(ctx, `SELECT active_preset FROM users WHERE id = $1`, []any{userID},
		func(row pgx.Row) error { return row.Scan(&preset) })
	if err != nil {
		return "", fmt.Errorf("theme: active preset: %w", err)
	}
	if preset == nil {
		return "", nil
	}
	return *preset, nil
}

// Active returns the theme the caller has applied, or nil for the default.
func (s *Service) Active(ctx context.Context, userID int) (*Theme, error) {
	var t Theme
	err := s.db.QueryRow(ctx, themeSelect+` WHERE t.id = (SELECT active_theme_id FROM users WHERE id = $1)`,
		[]any{userID}, func(row pgx.Row) (err error) { t, err = scanTheme(row); return err })
	if errors.Is(err, db.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("theme: active: %w", err)
	}
	return &t, nil
}
