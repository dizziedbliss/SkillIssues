// Package progression provides server-authoritative XP, level, and badge calculation
// for developer achievements and gamification.
package progression

import (
	"strings"
)

// BaseXP maps issue difficulty tiers to base XP rewards.
var BaseXP = map[string]int{
	"BEGINNER":     75,
	"INTERMEDIATE": 140,
	"ADVANCED":     250,
}

// BadgeIcons maps badge names to their emoji icons.
var BadgeIcons = map[string]string{
	"First Blood":     "🩸",
	"Bug Wrangler":    "🩹",
	"Polyglot":        "🧪",
	"World Traveller": "🗺️",
	"Touch Grass":     "🌱",
}

// LevelInfo contains complete progression status for a given XP amount.
type LevelInfo struct {
	Level             int `json:"level"`
	TotalXP           int `json:"total_xp"`
	CurrentLevelMinXP int `json:"current_level_min_xp"`
	NextLevelXP       int `json:"next_level_xp"`
	ProgressInLevel   int `json:"progress_in_level"`
	LevelXPRequired   int `json:"level_xp_required"`
	ProgressPercent   int `json:"progress_percent"`
}

// BadgeDetail represents a badge with its icon.
type BadgeDetail struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// CalculateLevel computes the developer's level and progress in the current level.
// Thresholds follow the quadratic progression:
//
//	Level 1 starts at 0 XP
//	Level 2 requires 300 XP
//	Level 3 requires 750 XP (+450)
//	Level 4 requires 1350 XP (+600)
//	Formula: each level cost increases by 150 * (l + 1)
func CalculateLevel(totalXP int) LevelInfo {
	if totalXP < 0 {
		totalXP = 0
	}

	thresholds := make([]int, 0, 100)
	thresholds = append(thresholds, 0)
	curr := 0
	for l := 1; l < 100; l++ {
		curr += 150 + (l * 150)
		thresholds = append(thresholds, curr)
	}

	level := 1
	for i := 0; i < len(thresholds)-1; i++ {
		if totalXP >= thresholds[i] {
			level = i + 1
		} else {
			break
		}
	}

	minXP := thresholds[level-1]
	maxXP := thresholds[level]
	progressInLevel := totalXP - minXP
	levelXPRequired := maxXP - minXP
	if levelXPRequired < 1 {
		levelXPRequired = 1
	}

	progressPercent := (progressInLevel * 100) / levelXPRequired
	if progressPercent > 100 {
		progressPercent = 100
	}
	if progressPercent < 0 {
		progressPercent = 0
	}

	return LevelInfo{
		Level:             level,
		TotalXP:           totalXP,
		CurrentLevelMinXP: minXP,
		NextLevelXP:       maxXP,
		ProgressInLevel:   progressInLevel,
		LevelXPRequired:   levelXPRequired,
		ProgressPercent:   progressPercent,
	}
}

// CalculateEarnedXP calculates the XP reward for a merged pull request.
//
//	Base XP: BEGINNER=75, INTERMEDIATE=140, ADVANCED=250
//	Merged bonus: +25 (always awarded for merged PRs)
//	Repository novelty: +15 if this is user's 1st or 2nd merged PR in this repository
//	Cross-skill bonus: +10 if the issue requires multiple skills or technologies
func CalculateEarnedXP(difficulty string, priorRepoMergedCount int, skillCount, techCount int) (int, int, int) {
	upperDiff := strings.ToUpper(strings.TrimSpace(difficulty))
	baseXP, ok := BaseXP[upperDiff]
	if !ok {
		baseXP = 75 // fallback to beginner XP
	}

	bonusXP := 25 // Merged bonus (+25)

	if priorRepoMergedCount <= 1 {
		bonusXP += 15 // Novelty bonus (+15)
	}

	if skillCount > 1 || techCount > 1 {
		bonusXP += 10 // Multi-discipline bonus (+10)
	}

	return baseXP, bonusXP, baseXP + bonusXP
}

// StatsForBadges holds aggregated contribution counts needed to evaluate badges.
type StatsForBadges struct {
	TotalMerged    int
	TotalRepos     int
	TotalLanguages int
}

// EvaluateBadges computes earned badges based on contribution statistics.
func EvaluateBadges(stats StatsForBadges) []string {
	var earned []string
	if stats.TotalMerged >= 1 {
		earned = append(earned, "First Blood")
	}
	if stats.TotalMerged >= 3 {
		earned = append(earned, "Bug Wrangler")
	}
	if stats.TotalLanguages >= 2 {
		earned = append(earned, "Polyglot")
	}
	if stats.TotalRepos >= 2 {
		earned = append(earned, "World Traveller")
	}
	if stats.TotalMerged >= 5 {
		earned = append(earned, "Touch Grass")
	}
	return earned
}
