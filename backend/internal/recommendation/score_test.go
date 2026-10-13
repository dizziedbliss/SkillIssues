package recommendation

import (
	"testing"

	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/profile"
)

func TestScoreCandidates(t *testing.T) {
	u := &profile.FullProfile{
		Username:           "testdev",
		TargetLevel:        "INTERMEDIATE",
		PreferredLanguages: []string{"Go", "Python"},
		DemonstratedSkills: []string{"Go", "Docker"},
		Interests:          []string{"APIs", "Performance"},
	}

	candidates := []issue.Issue{
		{
			ID:              1,
			Repository:      "golang/go",
			Language:        "Go",
			DifficultyScore: intPtr(3),
			RequiredSkills:  []string{"Go"},
			Technologies:    []string{"Go"},
			LearningTags:    []string{"APIs"},
			Stars:           120000,
		},
		{
			ID:              2,
			Repository:      "vuejs/vue",
			Language:        "JavaScript",
			DifficultyScore: intPtr(3),
			RequiredSkills:  []string{"JavaScript"},
			Technologies:    []string{"JavaScript"},
			LearningTags:    []string{"UI"},
			Stars:           200000,
		},
	}

	results := ScoreCandidates(u, candidates, 10)
	if len(results) == 0 {
		t.Fatal("expected scored candidates")
	}

	// golang/go should be ranked #1 because of language match, skill match, interest match
	if results[0].ID != 1 {
		t.Fatalf("expected issue 1 to rank first, got %d", results[0].ID)
	}

	if results[0].RecommendationScore <= 0 {
		t.Fatalf("expected positive recommendation score, got %d", results[0].RecommendationScore)
	}

	// Verify why_recommended is populated
	if len(results[0].WhyRecommended) == 0 {
		t.Fatal("expected why_recommended tags to be populated")
	}
}

func intPtr(n int) *int { return &n }
