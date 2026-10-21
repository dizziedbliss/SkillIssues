package issue

import (
	"context"
	"slices"
	"strings"
)

// RecommendationCandidates returns up to pool open issues for recommendation
// scoring: the shuffled feed (one issue per repository) narrowed to
// repositories using one of langs and to issues at level, read page by page.
// It never reads the whole catalog. When those filters match fewer than want
// issues it widens them — level dropped first, then languages — so a caller
// with a rare language still gets something to score.
//
// base carries the per-request settings (RareLanguageBelow, LanguageCountTTL);
// its filters are ignored.
func (s *Service) RecommendationCandidates(ctx context.Context, base Filters, langs []string, level string, want, pool int) ([]Issue, error) {
	found, err := s.recommendationCandidates(ctx, base, langs, level, want, pool)
	return s.visible(found), err
}

func (s *Service) recommendationCandidates(ctx context.Context, base Filters, langs []string, level string, want, pool int) ([]Issue, error) {
	var tech []string
	for _, l := range langs {
		if l = strings.TrimSpace(l); l != "" {
			tech = append(tech, l)
		}
	}
	var levels []string
	if level = strings.ToUpper(strings.TrimSpace(level)); slices.Contains(Difficulties, level) {
		levels = []string{level}
	}

	tries := []Filters{{Tech: tech, Difficulties: levels}}
	if levels != nil {
		tries = append(tries, Filters{Tech: tech})
	}
	if tech != nil {
		tries = append(tries, Filters{})
	}

	var out []Issue
	seen := map[int]bool{}
	for _, f := range tries {
		f.Sort = SortShuffle
		f.RareLanguageBelow, f.LanguageCountTTL = base.RareLanguageBelow, base.LanguageCountTTL
		f.RareProbe, f.ShuffleWalkMaxRepos, f.ShuffleWalkBudget = base.RareProbe, base.ShuffleWalkMaxRepos, base.ShuffleWalkBudget
		var after *Cursor
		for len(out) < pool {
			f.Limit = pageLimit(pool - len(out))
			page, next, err := s.BrowseIssues(ctx, f, after, nil)
			if err != nil {
				return nil, err
			}
			for _, it := range page {
				if !seen[it.ID] {
					seen[it.ID] = true
					out = append(out, it)
				}
			}
			if next == nil {
				break
			}
			after = next
		}
		if len(out) >= want {
			break
		}
	}
	return out, nil
}
