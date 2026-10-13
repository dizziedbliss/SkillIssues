// Package profile handles developer profiles, skill inventories, and preferences.
package profile

import (
	"context"
	"fmt"
	"strings"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/progression"
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

// FullProfile represents the complete developer profile returned by GET /me and GET /profile.
type FullProfile struct {
	ID                 int                       `json:"id"`
	Username           string                    `json:"username"`
	Name               string                    `json:"name"`
	AvatarURL          string                    `json:"avatar_url"`
	ExperienceLevel    string                    `json:"experience_level"`
	TargetLevel        string                    `json:"target_level"`
	ProgressScore      int                       `json:"progress_score"`
	CompletedCount     int                       `json:"completed_count"`
	Bio                string                    `json:"bio"`
	DemonstratedSkills []string                  `json:"demonstrated_skills"`
	TotalXP            int                       `json:"total_xp"`
	Level              int                       `json:"level"`
	Badges             []string                  `json:"badges"`
	Interests          []string                  `json:"interests"`
	PreferredLanguages []string                  `json:"preferred_languages"`
	CurrentLevelMinXP  int                       `json:"current_level_min_xp"`
	NextLevelXP        int                       `json:"next_level_xp"`
	ProgressInLevel    int                       `json:"progress_in_level"`
	LevelXPRequired    int                       `json:"level_xp_required"`
	ProgressPercent    int                       `json:"progress_percent"`
	BadgeDetails       []progression.BadgeDetail `json:"badge_details"`
}

// Service manages developer profile operations.
type Service struct {
	db *db.Pool
}

// New creates a new profile Service.
func New(db *db.Pool) *Service {
	return &Service{db: db}
}

// GetFullProfile retrieves or initializes the profile, calculates live XP and badges,
// and persists updated progression metrics atomically.
func (s *Service) GetFullProfile(ctx context.Context, userID int) (*FullProfile, error) {
	// Ensure developer_profiles row exists
	_ = s.db.Exec(ctx,
		`INSERT INTO developer_profiles (user_id, experience_level, target_level, progress_score, completed_count, total_xp, level, badges)
		 VALUES ($1, 'BEGINNER', 'INTERMEDIATE', 0, 0, 0, 1, '{}')
		 ON CONFLICT (user_id) DO NOTHING`,
		userID,
	)

	// Calculate actual XP & completed count from merged contributions
	var completedCount int
	var calcXP int
	_ = s.db.QueryRow(ctx,
		`SELECT count(*) AS completed, COALESCE(SUM(
			CASE 
				WHEN UPPER(i.difficulty) = 'BEGINNER' THEN 75
				WHEN UPPER(i.difficulty) = 'INTERMEDIATE' THEN 140
				WHEN UPPER(i.difficulty) = 'ADVANCED' THEN 250
				ELSE 75
			END
		), 0) AS calc_xp
		FROM contributions c
		JOIN issues i ON i.id = c.issue_id
		WHERE c.user_id = $1 AND c.state = 'PR_MERGED'`,
		[]any{userID},
		func(row pgx.Row) error {
			return row.Scan(&completedCount, &calcXP)
		},
	)

	// Fetch stored total_xp
	var storedXP int
	_ = s.db.QueryRow(ctx,
		`SELECT total_xp FROM developer_profiles WHERE user_id = $1`,
		[]any{userID},
		func(row pgx.Row) error {
			return row.Scan(&storedXP)
		},
	)

	totalXP := storedXP
	if calcXP > totalXP {
		totalXP = calcXP
	}

	lvlInfo := progression.CalculateLevel(totalXP)

	// Evaluate badges based on user's merged PR statistics
	var totalMerged, totalRepos, totalLangs int
	_ = s.db.QueryRow(ctx,
		`SELECT 
			count(c.id) AS total_merged,
			count(DISTINCT r.id) AS total_repos,
			count(DISTINCT lower(r.language)) AS total_languages
		FROM contributions c
		JOIN issues i ON i.id = c.issue_id
		JOIN repositories r ON r.id = i.repository_id
		WHERE c.user_id = $1 AND c.state = 'PR_MERGED'`,
		[]any{userID},
		func(row pgx.Row) error {
			return row.Scan(&totalMerged, &totalRepos, &totalLangs)
		},
	)

	earnedBadges := progression.EvaluateBadges(progression.StatsForBadges{
		TotalMerged:    totalMerged,
		TotalRepos:     totalRepos,
		TotalLanguages: totalLangs,
	})
	if earnedBadges == nil {
		earnedBadges = []string{}
	}

	// Persist computed progression stats
	_ = s.db.Exec(ctx,
		`UPDATE developer_profiles SET
		  total_xp = $1,
		  level = $2,
		  progress_score = $3,
		  completed_count = $4,
		  badges = $5
		WHERE user_id = $6`,
		totalXP, lvlInfo.Level, lvlInfo.ProgressPercent, completedCount, earnedBadges, userID,
	)

	// Fetch full combined data
	var p FullProfile
	var name *string
	var avatarURL *string
	var bio *string

	err := s.db.QueryRow(ctx,
		`SELECT u.id, u.username, COALESCE(u.name, u.username), COALESCE(u.avatar_url, ''),
		        p.experience_level, p.target_level, p.progress_score, p.completed_count,
		        COALESCE(p.bio, ''), p.demonstrated_skills, p.total_xp, p.level, p.badges,
		        COALESCE(pref.interests, '{}'), COALESCE(pref.preferred_languages, '{}')
		 FROM users u
		 JOIN developer_profiles p ON p.user_id = u.id
		 LEFT JOIN developer_preferences pref ON pref.user_id = u.id
		 WHERE u.id = $1`,
		[]any{userID},
		func(row pgx.Row) error {
			return row.Scan(
				&p.ID, &p.Username, &name, &avatarURL,
				&p.ExperienceLevel, &p.TargetLevel, &p.ProgressScore, &p.CompletedCount,
				&bio, &p.DemonstratedSkills, &p.TotalXP, &p.Level, &p.Badges,
				&p.Interests, &p.PreferredLanguages,
			)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("profile: fetch full profile: %w", err)
	}

	if name != nil {
		p.Name = *name
	}
	if avatarURL != nil {
		p.AvatarURL = *avatarURL
	}
	if bio != nil {
		p.Bio = *bio
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
	if p.Badges == nil {
		p.Badges = []string{}
	}

	p.CurrentLevelMinXP = lvlInfo.CurrentLevelMinXP
	p.NextLevelXP = lvlInfo.NextLevelXP
	p.ProgressInLevel = lvlInfo.ProgressInLevel
	p.LevelXPRequired = lvlInfo.LevelXPRequired
	p.ProgressPercent = lvlInfo.ProgressPercent

	badgeDetails := make([]progression.BadgeDetail, 0, len(p.Badges))
	for _, b := range p.Badges {
		icon, ok := progression.BadgeIcons[b]
		if !ok {
			icon = "🏅"
		}
		badgeDetails = append(badgeDetails, progression.BadgeDetail{Name: b, Icon: icon})
	}
	p.BadgeDetails = badgeDetails

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
