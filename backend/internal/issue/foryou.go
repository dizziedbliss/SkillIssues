package issue

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// For you is the default feed: personal for a signed-in person at any level,
// and fair to the whole backlog.
//
//   - Candidates come from the shuffled walk (one issue per repository, the
//     filters applied) starting at a point seeded per person per day, so two
//     people start in different parts of the catalog instead of both at the top.
//   - The walk is read a chunk at a time (Rules.Pool repositories), and each
//     chunk is ordered by a weighted random draw (Gumbel-top-k, the Plackett-
//     Luce model): an issue's chance of coming first grows with its score,
//     Fit × Doable × Need, but a good issue is not shown to everyone first.
//   - Fit is how well it suits the person (their languages, level, interests,
//     what they've merged before); Doable is how likely it is to go somewhere
//     (labelled, a short discussion, recent, a welcoming repository); Need is
//     how long it has waited and how few people have been shown it lately.
//
// Pages are slices of the drawn chunks; the cursor carries the seed, so every
// page of one walk agrees, and the next day draws afresh.

// ForYouViewer is who the feed is for. The zero value is someone signed out:
// no Fit, and one draw a day shared by everyone (so pages stay cacheable).
type ForYouViewer struct {
	ID        int
	Languages []string // preferred, any case
	Interests []string
	Level     string // experience: BEGINNER, INTERMEDIATE or ADVANCED
	Target    string // the level they're aiming for
	// From their merged pull requests: merges per language (lower case) and
	// the repositories merged in.
	MergedLanguages map[string]int
	MergedRepos     map[int]bool
}

// ForYouRules are the api.foryou.* settings for one request.
type ForYouRules struct {
	Pool         int     // repositories walked per chunk
	Randomness   float64 // scale of the draw's noise; 0 is a plain ranking
	WeightFit    float64 // exponents on the three parts
	WeightDoable float64
	WeightNeed   float64
	ExposureHalf float64       // times shown at which Need's exposure part halves
	ChunkTTL     time.Duration // how long a drawn chunk is kept
	Now          time.Time
}

// ForYouStore keeps what the feed shares between requests: drawn chunks, and
// how often each issue has been shown. Every method fails open: a nil result
// or a no-op when the store can't be reached.
type ForYouStore interface {
	GetChunk(ctx context.Context, key string) []byte
	SetChunk(ctx context.Context, key string, data []byte, ttl time.Duration)
	// ShownCounts is how many times each issue was served lately.
	ShownCounts(ctx context.Context, ids []int) map[int]int
	RecordShown(ctx context.Context, ids []int)
}

// SetForYouStore sets the store; without one, chunks are drawn every time and
// nothing counts as shown.
func (s *Service) SetForYouStore(store ForYouStore) { s.forYou = store }

// forYouSeed is the draw's seed: one per person per UTC day (person 0 is
// everyone signed out).
func forYouSeed(viewer int, now time.Time) int64 {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "foryou:%d:%s", viewer, now.UTC().Format("2006-01-02"))
	return int64(h.Sum64() >> 2) // non-negative, fits the cursor's decimal form
}

// forYouUniform is the draw's uniform number in (0, 1) for one issue.
func forYouUniform(seed int64, issueID int) float64 {
	h := fnv.New64a()
	var b [16]byte
	binary.LittleEndian.PutUint64(b[:8], uint64(seed))
	binary.LittleEndian.PutUint64(b[8:], uint64(issueID))
	_, _ = h.Write(b[:])
	return (float64(h.Sum64()>>11) + 0.5) / float64(1<<53)
}

// levelRank places a difficulty: 1 to 3, 0 for UNCLASSIFIED or unknown.
func levelRank(d string) int {
	switch strings.ToUpper(d) {
	case "BEGINNER":
		return 1
	case "INTERMEDIATE":
		return 2
	case "ADVANCED":
		return 3
	}
	return 0
}

// forYouScore is one issue's Fit × Doable × Need (each raised to its weight),
// and the reasons worth showing, most telling first.
func forYouScore(it *Issue, v *ForYouViewer, r *ForYouRules, shown int) (float64, []string) {
	var reasons []string
	lower := func(xs []string) map[string]bool {
		m := make(map[string]bool, len(xs))
		for _, x := range xs {
			if x = strings.ToLower(strings.TrimSpace(x)); x != "" {
				m[x] = true
			}
		}
		return m
	}

	// Fit: only for someone signed in.
	fit := 1.0
	if v.ID != 0 {
		langs := lower(v.Languages)
		lang := strings.ToLower(it.Language)
		switch {
		case len(langs) == 0:
			fit = 0.7
		case langs[lang]:
			fit = 1.0
			reasons = append(reasons, "language")
		default:
			fit = 0.35
			for _, t := range it.Tech {
				if langs[strings.ToLower(t)] {
					fit = 0.75
					reasons = append(reasons, "tech")
					break
				}
			}
		}
		if v.MergedRepos[it.RepositoryID] {
			fit *= 1.2
			reasons = append(reasons, "returning")
		} else if v.MergedLanguages[lang] > 0 {
			fit *= 1.1
			reasons = append(reasons, "history")
		}
		d, mine, aim := levelRank(it.Difficulty), levelRank(v.Level), levelRank(v.Target)
		switch {
		case d == 0:
			fit *= 0.6
		case d == mine || d == aim:
			reasons = append(reasons, "level")
		case mine != 0 && (d-mine == 1 || mine-d == 1):
			fit *= 0.7
		default:
			fit *= 0.35
		}
		if interests := lower(v.Interests); len(interests) > 0 {
			for _, x := range append(append([]string{}, it.Labels...), it.Tech...) {
				if interests[strings.ToLower(x)] {
					fit *= 1.1
					reasons = append(reasons, "interest")
					break
				}
			}
		}
	}

	// Doable: what predicts an issue gets resolved.
	doable := 1.0
	if it.DifficultyScore == nil {
		doable *= 0.8
	}
	switch {
	case it.Comments <= 5:
	case it.Comments <= 15:
		doable *= 0.8
	case it.Comments <= 40:
		doable *= 0.6
	default:
		doable *= 0.4
	}
	if !it.updated.IsZero() {
		switch days := r.Now.Sub(it.updated).Hours() / 24; {
		case days <= 60:
		case days <= 365:
			doable *= 0.8
		default:
			doable *= 0.6
		}
	}
	if it.ContributingURL != nil {
		doable *= 1.05
	}
	if h := it.Health; h != nil {
		if h.Welcoming {
			doable *= 1.15
			reasons = append(reasons, "welcoming")
		}
		if h.Slow {
			doable *= 0.75
		}
	}

	// Need: waiting long, and seen by few lately.
	need := 1.0
	if !it.created.IsZero() {
		days := r.Now.Sub(it.created).Hours() / 24
		need = 0.75 + 0.35*math.Min(1, math.Max(0, days)/180)
		if days >= 90 {
			reasons = append(reasons, "waiting")
		}
	}
	if r.ExposureHalf > 0 {
		need /= 1 + float64(shown)/r.ExposureHalf
	}

	score := math.Pow(fit, r.WeightFit) * math.Pow(doable, r.WeightDoable) * math.Pow(need, r.WeightNeed)
	if len(reasons) > 2 {
		reasons = reasons[:2]
	}
	return score, reasons
}

// drawForYou orders a chunk: by log score plus Gumbel noise scaled by
// Randomness, which samples the order in proportion to score (Plackett-Luce)
// when Randomness is 1, and sharpens towards a plain ranking below it.
func drawForYou(items []Issue, v *ForYouViewer, r *ForYouRules, seed int64, shown map[int]int) []Issue {
	type keyed struct {
		it  Issue
		key float64
	}
	ks := make([]keyed, len(items))
	for i := range items {
		score, reasons := forYouScore(&items[i], v, r, shown[items[i].ID])
		it := items[i]
		it.ForYou = reasons
		u := forYouUniform(seed, it.ID)
		ks[i] = keyed{it, math.Log(math.Max(score, 1e-9)) + r.Randomness*-math.Log(-math.Log(u))}
	}
	sort.SliceStable(ks, func(a, b int) bool {
		if ks[a].key != ks[b].key {
			return ks[a].key > ks[b].key
		}
		return ks[a].it.ID < ks[b].it.ID
	})
	out := make([]Issue, len(ks))
	for i := range ks {
		out[i] = ks[i].it
	}
	return out
}

// maxForYouChunksPerPage bounds the chunks one page may draw.
const maxForYouChunksPerPage = 4

// forYouChunk is one drawn chunk: its issue ids in drawn order, their
// reasons, and where the walk goes on after it.
type forYouChunk struct {
	IDs     []int            `json:"ids"`
	Reasons map[int][]string `json:"reasons,omitempty"`
	// Next position, or End when the walk is over.
	NextKey     int  `json:"next_key"`
	NextID      int  `json:"next_id"`
	NextWrapped bool `json:"next_wrapped"`
	End         bool `json:"end"`
}

// ForYouPage returns one page of the For you feed after the cursor (from the
// day's seeded start for nil), and the cursor for the next page.
func (s *Service) ForYouPage(ctx context.Context, f Filters, after *Cursor, v ForYouViewer, r ForYouRules) ([]Issue, *Cursor, error) {
	page, next, err := s.forYouPage(ctx, f, after, &v, &r)
	return s.visible(page, f.Unassigned), next, err
}

func (s *Service) forYouPage(ctx context.Context, f Filters, after *Cursor, v *ForYouViewer, r *ForYouRules) ([]Issue, *Cursor, error) {
	if after != nil && after.Sort != SortForYou {
		return nil, nil, fmt.Errorf("%w: the cursor is for the %s sort, not foryou", ErrBadCursor, after.Sort)
	}
	limit := pageLimit(f.Limit)
	pos := after
	if pos == nil {
		seed := forYouSeed(v.ID, r.Now)
		start := int(seed % math.MaxInt32)
		// Just before the start: repository ids are positive.
		pos = &Cursor{Sort: SortForYou, Seed: seed, Start: start, Key: start, ID: 0}
	}

	var page []int
	reasons := map[int][]string{}
	cur := *pos
	// A chunk can come back empty (a walk cut short by its time budget, a
	// filter few repositories pass); after a few, the page is returned short,
	// with a cursor: a short page with a cursor isn't the end.
	for chunks := 0; len(page) < limit && chunks < maxForYouChunksPerPage; chunks++ {
		chunk, err := s.forYouChunk(ctx, f, &cur, v, r)
		if err != nil {
			return nil, nil, err
		}
		take := chunk.IDs[min(cur.Offset, len(chunk.IDs)):]
		take = take[:min(len(take), limit-len(page))]
		page = append(page, take...)
		for _, id := range take {
			reasons[id] = chunk.Reasons[id]
		}
		cur.Offset += len(take)
		if cur.Offset < len(chunk.IDs) {
			break // the page is full inside this chunk
		}
		if chunk.End {
			cur = Cursor{}
			break
		}
		cur = Cursor{Sort: SortForYou, Seed: cur.Seed, Start: cur.Start, Key: chunk.NextKey, ID: chunk.NextID, Wrapped: chunk.NextWrapped}
	}

	issues, err := s.loadInOrder(ctx, page)
	if err != nil {
		return nil, nil, err
	}
	for i := range issues {
		issues[i].ForYou = reasons[issues[i].ID]
	}
	if s.forYou != nil && len(page) > 0 {
		s.forYou.RecordShown(ctx, page)
	}
	if cur.Sort == "" {
		return issues, nil, nil
	}
	return issues, &cur, nil
}

// forYouChunk returns the drawn chunk that begins at the cursor's position,
// from the store when it's there.
func (s *Service) forYouChunk(ctx context.Context, f Filters, at *Cursor, v *ForYouViewer, r *ForYouRules) (forYouChunk, error) {
	filterKey, err := json.Marshal(f.cacheable())
	if err != nil {
		return forYouChunk{}, fmt.Errorf("issue: for-you chunk key: %w", err)
	}
	h := fnv.New64a()
	_, _ = h.Write(filterKey)
	key := "fy:" + strconv.FormatInt(at.Seed, 36) + ":" + strconv.FormatUint(h.Sum64(), 36) + ":" +
		strconv.Itoa(at.Key) + ":" + strconv.Itoa(at.ID) + ":" + strconv.FormatBool(at.Wrapped) + ":" + strconv.Itoa(r.Pool)
	if s.forYou != nil {
		if data := s.forYou.GetChunk(ctx, key); data != nil {
			var c forYouChunk
			if json.Unmarshal(data, &c) == nil {
				return c, nil
			}
		}
	}

	c, err := s.drawChunk(ctx, f, at, v, r)
	if err != nil {
		return forYouChunk{}, err
	}
	if s.forYou != nil && r.ChunkTTL > 0 {
		if data, err := json.Marshal(c); err == nil {
			s.forYou.SetChunk(ctx, key, data, r.ChunkTTL)
		}
	}
	return c, nil
}

// drawChunk walks the next Pool repositories of the shuffled order from the
// position, wrapping round the end once and stopping where the walk began,
// and draws their issues' order.
func (s *Service) drawChunk(ctx context.Context, f Filters, at *Cursor, v *ForYouViewer, r *ForYouRules) (forYouChunk, error) {
	f.Sort = SortShuffle
	var from *Cursor
	if !(at.Wrapped && at.Key < 0) {
		from = &Cursor{Sort: SortShuffle, Key: at.Key, ID: at.ID}
	}
	hits, next, err := s.shuffleWalk(ctx, f, from, max(r.Pool, 1))
	if err != nil {
		return forYouChunk{}, err
	}
	c := forYouChunk{}
	if at.Wrapped {
		// Past the end once already: stop at the walk's start.
		for i, h := range hits {
			if h.key >= at.Start {
				hits, next = hits[:i], nil
				break
			}
		}
		if next != nil && next.Key >= at.Start {
			next = nil
		}
		c.End = next == nil
	} else if next == nil {
		// The end of the order: go round from the top, once.
		c.NextKey, c.NextID, c.NextWrapped = -1, 0, true
	}
	if next != nil {
		c.NextKey, c.NextID, c.NextWrapped = next.Key, next.ID, at.Wrapped
	}

	ids := make([]int, len(hits))
	for i, h := range hits {
		ids[i] = h.issue
	}
	items, err := s.loadInOrder(ctx, ids)
	if err != nil {
		return forYouChunk{}, err
	}
	// Health is decided per request (health.go); the score reads it.
	rules := s.rulesNow()
	for i := range items {
		s.annotate(&items[i], nil, rules)
	}
	var shown map[int]int
	if s.forYou != nil {
		shown = s.forYou.ShownCounts(ctx, ids)
	}
	drawn := drawForYou(items, v, r, at.Seed, shown)
	c.IDs = make([]int, len(drawn))
	c.Reasons = make(map[int][]string, len(drawn))
	for i, it := range drawn {
		c.IDs[i] = it.ID
		if len(it.ForYou) > 0 {
			c.Reasons[it.ID] = it.ForYou
		}
	}
	return c, nil
}

// decodeForYou parses a for-you cursor's fields: seed, start, key, repository,
// offset and wrapped.
func decodeForYou(parts []string) (Cursor, error) {
	var n [6]int64
	for i, p := range parts {
		v, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return Cursor{}, ErrBadCursor
		}
		n[i] = v
	}
	if n[0] < 0 || n[1] < 0 || n[1] > math.MaxInt32 || n[2] < -1 || n[2] > math.MaxInt32 || n[3] < 0 || n[4] < 0 || n[5] < 0 || n[5] > 1 {
		return Cursor{}, ErrBadCursor
	}
	return Cursor{Sort: SortForYou, Seed: n[0], Start: int(n[1]), Key: int(n[2]), ID: int(n[3]), Offset: int(n[4]), Wrapped: n[5] == 1}, nil
}

// forYouFilterKey is what a chunk depends on among the filters: the lists,
// never the per-request settings or the page size.
type forYouFilterKey struct {
	Difficulties, Languages, Labels, Topics, Tech, Repos, Owners []string
	Unassigned                                                   bool
	CommentsMin, CommentsMax                                     *int
}

func (f Filters) cacheable() forYouFilterKey {
	return forYouFilterKey{f.Difficulties, f.Languages, f.Labels, f.Topics, f.Tech, f.Repos, f.Owners, f.Unassigned, f.CommentsMin, f.CommentsMax}
}

// loadInOrder loads issues by id, in that order; one deleted since is skipped.
func (s *Service) loadInOrder(ctx context.Context, ids []int) ([]Issue, error) {
	if len(ids) == 0 {
		return []Issue{}, nil
	}
	byID := make(map[int]Issue, len(ids))
	err := s.db.QueryRows(ctx, baseIssueSelect+" WHERE i.id = ANY($1)", []any{ids}, func(rows pgx.Rows) error {
		it, err := scanIssue(rows)
		if err != nil {
			return err
		}
		byID[it.ID] = it
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("issue: load issues by id: %w", err)
	}
	out := make([]Issue, 0, len(ids))
	for _, id := range ids {
		if it, ok := byID[id]; ok {
			out = append(out, it)
		}
	}
	return out, nil
}
