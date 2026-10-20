// Package profile handles developer profiles, skill inventories, and preferences.
package profile

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

// ProfileUpdate contains the fields that developers can update via PUT /profile.
type ProfileUpdate struct {
	Bio                string   `json:"bio"`
	ExperienceLevel    string   `json:"experience_level"`
	TargetLevel        string   `json:"target_level"`
	Interests          []string `json:"interests"`
	PreferredLanguages []string `json:"preferred_languages"`
}

// FullProfile represents the complete developer profile returned by GET /me.
// XP, levels and badges are absent: they were computed from contributions,
// which are deferred to the gamification redesign.
type FullProfile struct {
	ID                 int      `json:"id"`
	Username           string   `json:"username"`
	Name               string   `json:"name"`
	AvatarURL          string   `json:"avatar_url"`
	ExperienceLevel    string   `json:"experience_level"`
	TargetLevel        string   `json:"target_level"`
	Bio                string   `json:"bio"`
	DemonstratedSkills []string `json:"demonstrated_skills"`
	Interests          []string `json:"interests"`
	PreferredLanguages []string `json:"preferred_languages"`
	JoinedAt           string   `json:"joined_at"` // when the account was created here, RFC 3339
}

// Service manages developer profile operations.
type Service struct {
	db *db.Pool
}

// New creates a new profile Service.
func New(db *db.Pool) *Service {
	return &Service{db: db}
}

// GetFullProfile returns the user's profile, creating the profile row if it
// does not exist yet.
func (s *Service) GetFullProfile(ctx context.Context, userID int) (*FullProfile, error) {
	if err := s.db.Exec(ctx,
		`INSERT INTO developer_profiles (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`,
		userID,
	); err != nil {
		return nil, fmt.Errorf("profile: ensure profile row: %w", err)
	}

	var p FullProfile
	var joined time.Time
	err := s.db.QueryRow(ctx,
		`SELECT u.id, COALESCE(u.username, ''), COALESCE(u.name, u.username, ''), COALESCE(u.avatar_url, ''),
		        p.experience_level, p.target_level, COALESCE(p.bio, ''), p.demonstrated_skills,
		        COALESCE(pref.interests, '{}'), COALESCE(pref.preferred_languages, '{}'), u.created_at
		 FROM users u
		 JOIN developer_profiles p ON p.user_id = u.id
		 LEFT JOIN developer_preferences pref ON pref.user_id = u.id
		 WHERE u.id = $1`,
		[]any{userID},
		func(row pgx.Row) error {
			return row.Scan(
				&p.ID, &p.Username, &p.Name, &p.AvatarURL,
				&p.ExperienceLevel, &p.TargetLevel, &p.Bio, &p.DemonstratedSkills,
				&p.Interests, &p.PreferredLanguages, &joined,
			)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("profile: fetch full profile: %w", err)
	}

	p.JoinedAt = joined.UTC().Format(time.RFC3339)
	if p.Interests == nil {
		p.Interests = []string{}
	}
	if p.PreferredLanguages == nil {
		p.PreferredLanguages = []string{}
	}
	if p.DemonstratedSkills == nil {
		p.DemonstratedSkills = []string{}
	}
	return &p, nil
}

// Limits bound what UpdateProfile stores (api.profile.* settings).
type Limits struct {
	BioMaxChars  int
	MaxLanguages int
	MaxInterests int
}

// ErrInvalid wraps a profile update that is refused, with the reason.
var ErrInvalid = errors.New("profile: invalid update")

var levels = []string{"BEGINNER", "INTERMEDIATE", "ADVANCED"}

// maxNameRunes is the longest one interest or language name may be: a format
// rule (a name, not a sentence), like the theme schema's text lengths.
const maxNameRunes = 60

// cleanList trims a list, drops empty and repeated values (case-insensitively,
// keeping the first spelling) and refuses one longer than max.
func cleanList(name string, vals []string, max int) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range vals {
		v = strings.TrimSpace(v)
		k := strings.ToLower(v)
		if v == "" || seen[k] {
			continue
		}
		if utf8.RuneCountInString(v) > maxNameRunes || strings.ContainsAny(v, "<>\x00") {
			return nil, fmt.Errorf("%w: %s value %q is not a name", ErrInvalid, name, v)
		}
		seen[k] = true
		out = append(out, v)
	}
	if len(out) > max {
		return nil, fmt.Errorf("%w: at most %d %s", ErrInvalid, max, name)
	}
	return out, nil
}

// UpdateProfile replaces bio, experience level, target level, interests and
// preferred languages. Anything out of bounds is refused with ErrInvalid,
// never trimmed or defaulted.
func (s *Service) UpdateProfile(ctx context.Context, userID int, update ProfileUpdate, lim Limits) (*FullProfile, error) {
	expLevel := strings.ToUpper(strings.TrimSpace(update.ExperienceLevel))
	if !slices.Contains(levels, expLevel) {
		return nil, fmt.Errorf("%w: experience_level must be BEGINNER, INTERMEDIATE or ADVANCED", ErrInvalid)
	}
	targetLevel := strings.ToUpper(strings.TrimSpace(update.TargetLevel))
	if !slices.Contains(levels, targetLevel) {
		return nil, fmt.Errorf("%w: target_level must be BEGINNER, INTERMEDIATE or ADVANCED", ErrInvalid)
	}
	bio := strings.TrimSpace(update.Bio)
	if n := utf8.RuneCountInString(bio); n > lim.BioMaxChars {
		return nil, fmt.Errorf("%w: the bio is %d characters, more than %d", ErrInvalid, n, lim.BioMaxChars)
	}
	interests, err := cleanList("interests", update.Interests, lim.MaxInterests)
	if err != nil {
		return nil, err
	}
	langs, err := cleanList("preferred_languages", update.PreferredLanguages, lim.MaxLanguages)
	if err != nil {
		return nil, err
	}

	// Both rows in one transaction, each an upsert: a missing row is created,
	// never silently left unchanged.
	err = s.db.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO developer_profiles (user_id, bio, experience_level, target_level) VALUES ($1, $2, $3, $4)
			 ON CONFLICT (user_id) DO UPDATE SET bio = EXCLUDED.bio,
			   experience_level = EXCLUDED.experience_level, target_level = EXCLUDED.target_level`,
			userID, bio, expLevel, targetLevel); err != nil {
			return fmt.Errorf("update profile: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO developer_preferences (user_id, interests, preferred_languages) VALUES ($1, $2, $3)
			 ON CONFLICT (user_id) DO UPDATE SET interests = EXCLUDED.interests,
			   preferred_languages = EXCLUDED.preferred_languages`,
			userID, interests, langs); err != nil {
			return fmt.Errorf("update preferences: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("profile: %w", err)
	}
	return s.GetFullProfile(ctx, userID)
}

// LinguistLanguagesAndFrameworks contains the standard catalog list.
var LinguistLanguagesAndFrameworks = []string{
	"Python", "TypeScript", "JavaScript", "Go", "Rust", "C++", "C", "C#",
	"Java", "Ruby", "PHP", "Swift", "Kotlin", "Dart", "HTML", "CSS", "Shell",
	"Scala", "Elixir", "Haskell", "Lua", "React", "Vue", "Angular", "Svelte",
	"Next.js", "Node.js", "Express", "FastAPI", "Django", "Flask", "Docker",
	"Kubernetes", "PostgreSQL", "Redis", "GraphQL", "PyTorch", "TensorFlow", "Tailwind",
}

// GetPreferenceOptions returns available language and framework options.
func (s *Service) GetPreferenceOptions(ctx context.Context) ([]string, error) {
	var dbLangs []string
	err := s.db.QueryRows(ctx,
		`SELECT DISTINCT language FROM repositories WHERE language IS NOT NULL AND language != ''`,
		nil,
		func(rows pgx.Rows) error {
			var lang string
			if err := rows.Scan(&lang); err != nil {
				return err
			}
			dbLangs = append(dbLangs, lang)
			return nil
		},
	)
	if err != nil {
		return LinguistLanguagesAndFrameworks, nil // fallback to base list on DB error
	}

	seen := make(map[string]bool)
	var combined []string
	for _, l := range LinguistLanguagesAndFrameworks {
		if !seen[l] {
			seen[l] = true
			combined = append(combined, l)
		}
	}
	for _, l := range dbLangs {
		if !seen[l] {
			seen[l] = true
			combined = append(combined, l)
		}
	}
	return combined, nil
}
