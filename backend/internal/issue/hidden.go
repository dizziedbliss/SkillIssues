package issue

// Hidden is what reports have taken out of the catalog's lists until someone
// reviews them (internal/report): issues, and every issue of a repository.
// An issue's own page, a saved issue and a search for its id still find it;
// only browsing, search results and recommendations leave it out.
type Hidden struct {
	Issues       map[int]bool
	Repositories map[int]bool
}

// SetHidden gives the service what to leave out; fn is called per list, so
// it should answer from memory (go-api refreshes it on a timer).
func (s *Service) SetHidden(fn func() Hidden) { s.hidden = fn }

// visible drops hidden issues from a list, keeping its order.
func (s *Service) visible(list []Issue) []Issue {
	if s.hidden == nil || len(list) == 0 {
		return list
	}
	h := s.hidden()
	if len(h.Issues) == 0 && len(h.Repositories) == 0 {
		return list
	}
	out := list[:0:0]
	for _, it := range list {
		if !h.Issues[it.ID] && !h.Repositories[it.RepositoryID] {
			out = append(out, it)
		}
	}
	return out
}
