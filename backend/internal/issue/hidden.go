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

// SetClaimed gives the service the issues someone on SkillIssues has claimed
// (internal/claim); like SetHidden, fn should answer from memory.
func (s *Service) SetClaimed(fn func() map[int]bool) { s.claimed = fn }

// visible drops hidden issues from a list, keeping its order, and marks the
// claimed ones; with dropClaimed (the catalog's "nobody is on it yet"), it
// drops those too.
func (s *Service) visible(list []Issue, dropClaimed bool) []Issue {
	if len(list) == 0 {
		return list
	}
	var h Hidden
	if s.hidden != nil {
		h = s.hidden()
	}
	claimed, rules := s.claimedNow(), s.rulesNow()
	out := list[:0:0]
	for _, it := range list {
		if h.Issues[it.ID] || h.Repositories[it.RepositoryID] || (dropClaimed && claimed[it.ID]) {
			continue
		}
		s.annotate(&it, claimed, rules)
		out = append(out, it)
	}
	return out
}

// markClaimed annotates issues shown whatever their claim: an issue's own
// page and saved issues.
func (s *Service) markClaimed(list []Issue) {
	claimed, rules := s.claimedNow(), s.rulesNow()
	for i := range list {
		s.annotate(&list[i], claimed, rules)
	}
}

func (s *Service) claimedNow() map[int]bool {
	if s.claimed == nil {
		return nil
	}
	return s.claimed()
}
