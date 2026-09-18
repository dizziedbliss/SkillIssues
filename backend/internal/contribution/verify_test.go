package contribution

import (
	"testing"
)

func TestPRURLRegex(t *testing.T) {
	valid := []string{
		"https://github.com/tiangolo/fastapi/pull/123",
		"https://github.com/dizziedbliss/SkillIssues/pull/1",
		"https://github.com/k8s-team/repo.sub/pull/9999",
	}
	for _, u := range valid {
		if !prURLRegex.MatchString(u) {
			t.Errorf("expected %s to match PR URL regex", u)
		}
	}

	invalid := []string{
		"http://github.com/a/b/pull/123", // non-https
		"https://github.com/a/b/issues/123", // issue not pull
		"https://gitlab.com/a/b/pull/123", // gitlab
		"https://github.com/a/pull/123", // missing repo
		"https://github.com/a/b/pull/abc", // non-numeric
	}
	for _, u := range invalid {
		if prURLRegex.MatchString(u) {
			t.Errorf("expected %s to NOT match PR URL regex", u)
		}
	}
}
