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
	claimed := s.claimedNow()
	if len(h.Issues) == 0 && len(h.Repositories) == 0 && len(claimed) == 0 {
		return list
	}
	out := list[:0:0]
	for _, it := range list {
		if h.Issues[it.ID] || h.Repositories[it.RepositoryID] {
			continue
		}
		if claimed[it.ID] {
			if dropClaimed {
				continue
			}
			it.Claimed = true
		}
		out = append(out, it)
	}
	return out
}

// markClaimed sets Claimed on issues shown whatever their claim: an issue's
// own page and saved issues.
func (s *Service) markClaimed(list []Issue) {
	claimed := s.claimedNow()
	for i := range list {
		list[i].Claimed = claimed[list[i].ID]
	}
}

func (s *Service) claimedNow() map[int]bool {
	if s.claimed == nil {
		return nil
	}
	return s.claimed()
}
