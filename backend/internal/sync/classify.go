package sync

import (
	"strings"
)

var beginnerLabels = map[string]struct{}{
	"good first issue": {}, "beginner-friendly": {}, "easy": {}, "starter": {}, "starter-bug": {},
	"first-timers-only": {}, "newbie": {}, "up-for-grabs": {},
	"low-hanging-fruit": {}, "bitesize": {}, "trivial": {}, "easy-fix": {}, "good-for-beginner": {},
	"documentation": {}, "docs": {}, "typo": {}, "good-first-pr": {},
}

var intermediateLabels = map[string]struct{}{
	"help wanted": {}, "contributions-welcome": {}, "seeking-contributors": {},
	"medium": {}, "intermediate": {}, "size/m": {}, "effort/medium": {}, "difficulty/medium": {},
	"enhancement": {}, "feature": {}, "refactoring": {}, "unit-test": {},
}

var advancedLabels = map[string]struct{}{
	"hard": {}, "advanced": {}, "complex": {}, "size/l": {}, "size/xl": {}, "difficulty/hard": {},
	"breaking-change": {}, "architecture": {}, "security": {}, "performance": {}, "optimization": {},
	"critical": {}, "high-priority": {}, "blocker": {}, "p1": {},
}

// ClassifyLabels evaluates GitHub issue label names and returns:
// hasValidDifficultyLabel, difficulty ("BEGINNER"|"INTERMEDIATE"|"ADVANCED"), and difficultyScore (1-10).
func ClassifyLabels(labels []string) (bool, string, int) {
	normalized := make([]string, 0, len(labels))
	for _, l := range labels {
		trimmed := strings.TrimSpace(strings.ToLower(l))
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}

	for _, l := range normalized {
		if _, ok := beginnerLabels[l]; ok || startsWithAny(l, "difficulty/easy", "difficulty/beginner", "size/small", "size/xs", "effort/low", "exp/beginner") {
			return true, "BEGINNER", 2
		}
	}

	for _, l := range normalized {
		if _, ok := intermediateLabels[l]; ok || startsWithAny(l, "difficulty/medium", "size/m", "effort/medium", "exp/intermediate") {
			return true, "INTERMEDIATE", 5
		}
	}

	for _, l := range normalized {
		if _, ok := advancedLabels[l]; ok || startsWithAny(l, "difficulty/hard", "difficulty/expert", "size/l", "size/xl", "effort/high", "exp/expert") {
			return true, "ADVANCED", 8
		}
	}

	return false, "BEGINNER", 3
}

func startsWithAny(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// ExtractMetadata extracts difficulty, score, valid label flag, skills, technologies, and tags
// from issue details and repository primary language.
func ExtractMetadata(labels []string, body string, repoLanguage string) (score int, difficulty string, hasValid bool, skills []string, technologies []string, tags []string) {
	hasValid, difficulty, score = ClassifyLabels(labels)

	var labelNames []string
	for _, l := range labels {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			labelNames = append(labelNames, trimmed)
		}
	}

	labelText := strings.ToLower(strings.Join(labelNames, " "))
	bodyLower := strings.ToLower(body)

	seenSkills := make(map[string]struct{})
	var skillList []string

	addSkill := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, exists := seenSkills[s]; !exists {
			seenSkills[s] = struct{}{}
			skillList = append(skillList, s)
		}
	}

	if repoLanguage != "" {
		addSkill(repoLanguage)
		technologies = []string{repoLanguage}
	} else {
		technologies = []string{}
	}

	if containsAny(labelText, "api", "http", "rest") || containsAny(bodyLower, "api", "http", "rest") {
		addSkill("REST APIs")
	}

	if containsAny(labelText, "docs", "documentation") || containsAny(bodyLower, "docs", "documentation") {
		addSkill("Technical Writing")
	}

	if len(labelNames) > 0 {
		limit := len(labelNames)
		if limit > 5 {
			limit = 5
		}
		tags = labelNames[:limit]
	} else {
		tags = []string{"open source", "software development"}
	}

	skills = skillList
	if skills == nil {
		skills = []string{}
	}

	return score, difficulty, hasValid, skills, technologies, tags
}

func containsAny(target string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(target, kw) {
			return true
		}
	}
	return false
}
