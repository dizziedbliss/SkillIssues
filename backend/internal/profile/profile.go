// Package profile handles developer profiles, skill inventories, and preferences.
package profile

import (
	"context"
	"fmt"
	"strings"

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
	err := s.db.QueryRow(ctx,
		`SELECT u.id, COALESCE(u.username, ''), COALESCE(u.name, u.username, ''), COALESCE(u.avatar_url, ''),
		        p.experience_level, p.target_level, COALESCE(p.bio, ''), p.demonstrated_skills,
		        COALESCE(pref.interests, '{}'), COALESCE(pref.preferred_languages, '{}')
		 FROM users u
		 JOIN developer_profiles p ON p.user_id = u.id
		 LEFT JOIN developer_preferences pref ON pref.user_id = u.id
		 WHERE u.id = $1`,
		[]any{userID},
		func(row pgx.Row) error {
			return row.Scan(
				&p.ID, &p.Username, &p.Name, &p.AvatarURL,
				&p.ExperienceLevel, &p.TargetLevel, &p.Bio, &p.DemonstratedSkills,
				&p.Interests, &p.PreferredLanguages,
			)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("profile: fetch full profile: %w", err)
	}

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

// UpdateProfile updates bio, experience level, target level, interests, and preferred languages.
func (s *Service) UpdateProfile(ctx context.Context, userID int, update ProfileUpdate) (*FullProfile, error) {
	expLevel := strings.ToUpper(strings.TrimSpace(update.ExperienceLevel))
	if expLevel != "BEGINNER" && expLevel != "INTERMEDIATE" && expLevel != "ADVANCED" {
		expLevel = "BEGINNER"
	}
	targetLevel := strings.ToUpper(strings.TrimSpace(update.TargetLevel))
	if targetLevel != "BEGINNER" && targetLevel != "INTERMEDIATE" && targetLevel != "ADVANCED" {
		targetLevel = "INTERMEDIATE"
	}

	if update.Interests == nil {
		update.Interests = []string{}
	}
	if update.PreferredLanguages == nil {
		update.PreferredLanguages = []string{}
	}

	err := s.db.Exec(ctx,
		`UPDATE developer_profiles SET bio = $1, experience_level = $2, target_level = $3
		 WHERE user_id = $4`,
		update.Bio, expLevel, targetLevel, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("profile: update profile: %w", err)
	}

	err = s.db.Exec(ctx,
		`UPDATE developer_preferences SET interests = $1, preferred_languages = $2
		 WHERE user_id = $3`,
		update.Interests, update.PreferredLanguages, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("profile: update preferences: %w", err)
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
