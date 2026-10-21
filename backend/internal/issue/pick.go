package issue

import "context"

// Pick returns up to n open issues matching f, walking the shuffled feed (one
// issue per repository, so a pick spans projects), unassigned and unclaimed,
// skipping the ids in exclude. It reads at most maxPages pages.
func (s *Service) Pick(ctx context.Context, f Filters, n, maxPages int, exclude map[int]bool) ([]Issue, error) {
	f.Sort = SortShuffle
	f.Unassigned = true
	var out []Issue
	var after *Cursor
	for page := 0; page < maxPages && len(out) < n; page++ {
		f.Limit = max(n*2, 10)
		got, next, err := s.BrowseIssues(ctx, f, after, nil)
		if err != nil {
			return nil, err
		}
		for _, it := range got {
			if len(out) < n && !exclude[it.ID] {
				exclude[it.ID] = true
				out = append(out, it)
			}
		}
		if next == nil {
			break
		}
		after = next
	}
	return out, nil
}
