// Package recommendation provides personalized challenge scoring and ranking
// matching developer skills, interests, and target levels with open-source issues.
package recommendation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/profile"
)

// TargetDifficultyMap maps target level strings to numerical difficulty targets.
var TargetDifficultyMap = map[string]int{
	"BEGINNER":     1,
	"INTERMEDIATE": 3,
	"ADVANCED":     5,
}

// ScoreCandidates ranks candidate issues for a given user profile.
//
// Formula:
//
//	Score = LangBoost (15) + TechBoost (8) + (SkillMatch * 2) + InterestMatch + DifficultyFit
//
// Factors:
//   - LangBoost: +15 if repository primary language matches user's preferred languages
//   - TechBoost: +8 if user preferred languages match issue technologies, skills, or tags
//   - SkillMatch: Number of overlapping skills between user and issue (*2 weighting)
//   - InterestMatch: Number of overlapping tags between user interests and issue learning tags
//   - DifficultyFit: Proximity to user's target level: max(0, 5 - |difficulty_score - target|)
//
// Stars never enter the score (the same rule as For you, and what the public
// docs promise): ties keep the candidates' order, which comes off the
// shuffled feed, so popular repositories get no head start.
func ScoreCandidates(u *profile.FullProfile, candidates []issue.Issue, limit int) []issue.Issue {
	if limit <= 0 {
		limit = 3
	}

	preferredSet := make(map[string]bool)
	for _, l := range u.PreferredLanguages {
		preferredSet[strings.ToLower(strings.TrimSpace(l))] = true
	}

	userSkills := make(map[string]bool)
	for _, s := range u.DemonstratedSkills {
		userSkills[strings.ToLower(strings.TrimSpace(s))] = true
	}

	interests := make(map[string]bool)
	for _, in := range u.Interests {
		interests[strings.ToLower(strings.TrimSpace(in))] = true
	}

	target := 3 // default INTERMEDIATE
	if t, ok := TargetDifficultyMap[strings.ToUpper(strings.TrimSpace(u.TargetLevel))]; ok {
		target = t
	}

	userNameLower := strings.ToLower(u.Username)

	// Filter candidate pool if preferred languages are defined
	pool := candidates
	if len(preferredSet) > 0 {
		var filtered []issue.Issue
		for _, item := range candidates {
			repoLower := strings.ToLower(item.Repository)
			isUserRepo := userNameLower != "" && strings.HasPrefix(repoLower, userNameLower+"/")

			matchesLang := item.Language != "" && preferredSet[strings.ToLower(item.Language)]
			matchesTech := false
			if !matchesLang {
				for _, t := range item.Technologies {
					if preferredSet[strings.ToLower(t)] {
						matchesTech = true
						break
					}
				}
			}
			if !matchesLang && !matchesTech {
				for _, s := range item.RequiredSkills {
					if preferredSet[strings.ToLower(s)] {
						matchesTech = true
						break
					}
				}
			}
			if !matchesLang && !matchesTech {
				for _, tag := range item.LearningTags {
					if preferredSet[strings.ToLower(tag)] {
						matchesTech = true
						break
					}
				}
			}

			if isUserRepo || matchesLang || matchesTech {
				filtered = append(filtered, item)
			}
		}
		if len(filtered) > 0 {
			pool = filtered
		}
	}

	scored := make([]issue.Issue, len(pool))
	copy(scored, pool)

	for i := range scored {
		item := &scored[i]

		itemSkills := make(map[string]bool)
		for _, s := range item.RequiredSkills {
			itemSkills[strings.ToLower(strings.TrimSpace(s))] = true
		}

		itemTags := make(map[string]bool)
		for _, tag := range item.LearningTags {
			itemTags[strings.ToLower(strings.TrimSpace(tag))] = true
		}

		itemTechs := make(map[string]bool)
		for _, tech := range item.Technologies {
			itemTechs[strings.ToLower(strings.TrimSpace(tech))] = true
		}

		languageMatch := item.Language != "" && preferredSet[strings.ToLower(item.Language)]
		techMatch := false
		for k := range preferredSet {
			if itemTechs[k] || itemSkills[k] || itemTags[k] {
				techMatch = true
				break
			}
		}

		langBoost := 0
		if languageMatch {
			langBoost = 15
		}

		techBoost := 0
		if techMatch {
			techBoost = 8
		}

		skillOverlap := 0
		for k := range userSkills {
			if itemSkills[k] {
				skillOverlap++
			}
		}
		skillMatch := skillOverlap * 2

		interestOverlap := 0
		for k := range interests {
			if itemTags[k] {
				interestOverlap++
			}
		}
		interestMatch := interestOverlap

		diffScore := 0
		if item.DifficultyScore != nil {
			diffScore = *item.DifficultyScore
		}
		if diffScore <= 0 {
			diffScore = 1
		}
		diffDelta := diffScore - target
		if diffDelta < 0 {
			diffDelta = -diffDelta
		}
		difficultyFit := 5 - diffDelta
		if difficultyFit < 0 {
			difficultyFit = 0
		}

		item.RecommendationScore = langBoost + techBoost + skillMatch + interestMatch + difficultyFit

		var why []string
		if languageMatch {
			why = append(why, fmt.Sprintf("Uses your preferred language: %s", item.Language))
		}
		if techMatch && !languageMatch {
			why = append(why, "Matches your preferred technologies")
		}
		if skillMatch > 0 {
			why = append(why, "Builds on your current skills")
		}
		if interestMatch > 0 {
			why = append(why, "Matches your interests")
		}
		if diffScore >= target {
			why = append(why, "Slightly stretches your current level")
		}
		if len(why) == 0 {
			why = append(why, "A strong open-source starting point")
		}
		item.WhyRecommended = why
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].RecommendationScore > scored[j].RecommendationScore
	})

	if len(scored) > limit {
		return scored[:limit]
	}
	return scored
}
