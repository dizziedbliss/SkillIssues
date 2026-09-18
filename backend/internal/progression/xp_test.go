package progression

import (
	"testing"
)

func TestCalculateLevel(t *testing.T) {
	tests := []struct {
		xp       int
		wantLvl  int
		wantMin  int
		wantNext int
	}{
		{0, 1, 0, 300},
		{100, 1, 0, 300},
		{300, 2, 300, 750},
		{749, 2, 300, 750},
		{750, 3, 750, 1350},
		{1350, 4, 1350, 2100},
	}

	for _, tt := range tests {
		info := CalculateLevel(tt.xp)
		if info.Level != tt.wantLvl {
			t.Errorf("CalculateLevel(%d).Level = %d, want %d", tt.xp, info.Level, tt.wantLvl)
		}
		if info.CurrentLevelMinXP != tt.wantMin {
			t.Errorf("CalculateLevel(%d).CurrentLevelMinXP = %d, want %d", tt.xp, info.CurrentLevelMinXP, tt.wantMin)
		}
		if info.NextLevelXP != tt.wantNext {
			t.Errorf("CalculateLevel(%d).NextLevelXP = %d, want %d", tt.xp, info.NextLevelXP, tt.wantNext)
		}
	}
}

func TestCalculateEarnedXP(t *testing.T) {
	// Beginner, first time in repo, 1 skill, 1 tech: 75 + (25 + 15) = 115
	base, bonus, total := CalculateEarnedXP("BEGINNER", 0, 1, 1)
	if base != 75 || bonus != 40 || total != 115 {
		t.Fatalf("expected 75, 40, 115; got %d, %d, %d", base, bonus, total)
	}

	// Intermediate, 3rd time in repo (prior=2), 2 skills: 140 + (25 + 0 + 10) = 175
	base, bonus, total = CalculateEarnedXP("INTERMEDIATE", 2, 2, 1)
	if base != 140 || bonus != 35 || total != 175 {
		t.Fatalf("expected 140, 35, 175; got %d, %d, %d", base, bonus, total)
	}

	// Advanced, 1st time (prior=0), 2 skills: 250 + (25 + 15 + 10) = 300
	base, bonus, total = CalculateEarnedXP("ADVANCED", 0, 2, 2)
	if base != 250 || bonus != 50 || total != 300 {
		t.Fatalf("expected 250, 50, 300; got %d, %d, %d", base, bonus, total)
	}
}

func TestEvaluateBadges(t *testing.T) {
	badges := EvaluateBadges(StatsForBadges{
		TotalMerged:    3,
		TotalRepos:     2,
		TotalLanguages: 2,
	})
	expected := []string{"First Blood", "Bug Wrangler", "Polyglot", "World Traveller"}
	if len(badges) != len(expected) {
		t.Fatalf("expected %d badges, got %d", len(expected), len(badges))
	}
}
