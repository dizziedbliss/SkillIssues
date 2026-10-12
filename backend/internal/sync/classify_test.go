package sync

import (
	"reflect"
	"testing"
)

func TestClassifyLabels(t *testing.T) {
	tests := []struct {
		name      string
		labels    []string
		wantValid bool
		wantDiff  string
		wantScore int
	}{
		{
			name:      "good first issue",
			labels:    []string{"good first issue", "bug"},
			wantValid: true,
			wantDiff:  "BEGINNER",
			wantScore: 2,
		},
		{
			name:      "difficulty/beginner prefix",
			labels:    []string{"difficulty/beginner", "ui"},
			wantValid: true,
			wantDiff:  "BEGINNER",
			wantScore: 2,
		},
		{
			name:      "help wanted intermediate",
			labels:    []string{"help wanted"},
			wantValid: true,
			wantDiff:  "INTERMEDIATE",
			wantScore: 5,
		},
		{
			name:      "security advanced",
			labels:    []string{"security", "bug"},
			wantValid: true,
			wantDiff:  "ADVANCED",
			wantScore: 8,
		},
		{
			name:      "no matching label is unclassified, never beginner",
			labels:    []string{"triage", "needs-info"},
			wantValid: false,
			wantDiff:  "UNCLASSIFIED",
			wantScore: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			valid, diff, score := ClassifyLabels(tc.labels)
			if valid != tc.wantValid || diff != tc.wantDiff || score != tc.wantScore {
				t.Errorf("got (%v, %q, %d), want (%v, %q, %d)", valid, diff, score, tc.wantValid, tc.wantDiff, tc.wantScore)
			}
		})
	}
}

func TestExtractMetadata(t *testing.T) {
	labels := []string{"good first issue", "documentation", "api"}
	repoLang := "TypeScript"

	score, diff, valid, skills, tech, tags := ExtractMetadata(labels, repoLang)

	if !valid || diff != "BEGINNER" || score != 2 {
		t.Errorf("unexpected classification: valid=%v diff=%v score=%v", valid, diff, score)
	}

	if !reflect.DeepEqual(tech, []string{"TypeScript"}) {
		t.Errorf("tech = %v, want [TypeScript]", tech)
	}

	expectedSkills := []string{"TypeScript", "REST APIs", "Technical Writing"}
	if !reflect.DeepEqual(skills, expectedSkills) {
		t.Errorf("skills = %v, want %v", skills, expectedSkills)
	}

	if len(tags) != 3 {
		t.Errorf("tags len = %d, want 3", len(tags))
	}
}
