package issue

// Health is how an issue's repository treats contributions, from its latest
// merged pull requests (read by the crawl, internal/github.RepositoryHealth).
type Health struct {
	Sampled          int `json:"sampled"`
	Outside          int `json:"outside"`
	MedianMergeHours int `json:"median_merge_hours"`
	// Welcoming: enough of those merges came from outside the project.
	Welcoming bool `json:"welcoming"`
	// Slow: its median merge takes longer than api.health.slow_hours.
	Slow bool `json:"slow"`
}

// HealthRules are the api.health.* thresholds.
type HealthRules struct {
	MinSample, WelcomingMin, SlowHours int
}

// SetHealthRules gives the service the thresholds that decide Welcoming and
// Slow; fn is called per list, so it should read live settings cheaply.
func (s *Service) SetHealthRules(fn func() HealthRules) { s.healthRules = fn }

// annotate fills what is decided per request rather than stored: whether an
// issue is claimed, and its repository's health against today's thresholds.
func (s *Service) annotate(it *Issue, claimed map[int]bool, rules *HealthRules) {
	it.Claimed = claimed[it.ID]
	it.Health = nil
	if rules == nil || it.healthSampled == nil || it.healthOutside == nil || it.healthMedianHours == nil ||
		*it.healthSampled < rules.MinSample {
		return
	}
	it.Health = &Health{
		Sampled: *it.healthSampled, Outside: *it.healthOutside, MedianMergeHours: *it.healthMedianHours,
		Welcoming: *it.healthOutside >= rules.WelcomingMin,
		Slow:      *it.healthMedianHours >= rules.SlowHours,
	}
}

func (s *Service) rulesNow() *HealthRules {
	if s.healthRules == nil {
		return nil
	}
	r := s.healthRules()
	return &r
}
