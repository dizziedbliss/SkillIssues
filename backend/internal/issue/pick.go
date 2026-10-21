package issue

import (
	"context"
	"math/rand/v2"
)

// Pick returns up to n open issues matching f, walking the shuffled feed (one
// issue per repository, so a pick spans projects), unassigned and unclaimed,
// skipping the ids in exclude. It reads at most maxPages pages.
//
// The walk starts at a random point of the shuffled order and wraps round
// once, so two picks with the same filters don't both start from the top: a
// swap gets a different issue, not the next one down a fixed list.
func (s *Service) Pick(ctx context.Context, f Filters, n, maxPages int, exclude map[int]bool) ([]Issue, error) {
	return s.pickFrom(ctx, f, n, maxPages, exclude, rand.Int32())
}

func (s *Service) pickFrom(ctx context.Context, f Filters, n, maxPages int, exclude map[int]bool, start int32) ([]Issue, error) {
	f.Sort = SortShuffle
	f.Unassigned = true
	var out []Issue
	// Just before the start: repository ids are positive, so (start, 0) sorts
	// before every repository at that key.
	after := &Cursor{Sort: SortShuffle, Key: int(start), ID: 0}
	wrapped := false
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
		switch {
		case next == nil && wrapped:
			return out, nil
		case next == nil:
			// The end of the order: go round from the top, once.
			wrapped, after = true, nil
		case wrapped && next.Key >= int(start):
			return out, nil // back where the walk began
		default:
			after = next
		}
	}
	return out, nil
}
