package cache

import (
	"context"
	"strconv"
	"time"
)

// ForYou is the For you feed's store (issue.ForYouStore): drawn chunks, and how
// many times each issue was served, one hash per UTC day kept for days() days.
// Like the rest of the cache it fails open: a miss, no counts, or nothing
// recorded while Redis is down.
type ForYou struct {
	c    *Client
	days func() int
	now  func() time.Time
}

// NewForYou wraps the client; days is api.foryou.exposure_days, read per call.
func NewForYou(c *Client, days func() int) *ForYou {
	return &ForYou{c: c, days: days, now: time.Now}
}

func shownKey(day time.Time) string { return "fy:shown:" + day.UTC().Format("2006-01-02") }

// GetChunk returns a drawn chunk, or nil.
func (f *ForYou) GetChunk(ctx context.Context, key string) []byte {
	if f == nil || f.c == nil || f.c.rdb == nil {
		return nil
	}
	return f.c.get(ctx, key)
}

// SetChunk keeps a drawn chunk for ttl.
func (f *ForYou) SetChunk(ctx context.Context, key string, data []byte, ttl time.Duration) {
	if f == nil || f.c == nil || f.c.rdb == nil {
		return
	}
	f.c.set(ctx, key, data, ttl)
}

// ShownCounts adds up each issue's serves over the last days() days.
func (f *ForYou) ShownCounts(ctx context.Context, ids []int) map[int]int {
	out := make(map[int]int, len(ids))
	if f == nil || f.c == nil || f.c.rdb == nil || len(ids) == 0 {
		return out
	}
	callCtx, cancel, ok := f.c.begin(ctx, "foryou_shown")
	if !ok {
		return out
	}
	defer cancel()
	fields := make([]string, len(ids))
	for i, id := range ids {
		fields[i] = strconv.Itoa(id)
	}
	pipe := f.c.rdb.Pipeline()
	today := f.now()
	days := max(f.days(), 1)
	reads := make([]interface{ Val() []any }, 0, days)
	for d := 0; d < days; d++ {
		reads = append(reads, pipe.HMGet(callCtx, shownKey(today.AddDate(0, 0, -d)), fields...))
	}
	_, err := pipe.Exec(callCtx)
	f.c.observe(ctx, "foryou_shown", err)
	if err != nil {
		return out
	}
	for _, r := range reads {
		for i, v := range r.Val() {
			if s, ok := v.(string); ok {
				if n, err := strconv.Atoi(s); err == nil {
					out[ids[i]] += n
				}
			}
		}
	}
	return out
}

// RecordShown counts one serve of each issue, today.
func (f *ForYou) RecordShown(ctx context.Context, ids []int) {
	if f == nil || f.c == nil || f.c.rdb == nil || len(ids) == 0 {
		return
	}
	callCtx, cancel, ok := f.c.begin(ctx, "foryou_record")
	if !ok {
		return
	}
	defer cancel()
	key := shownKey(f.now())
	pipe := f.c.rdb.Pipeline()
	for _, id := range ids {
		pipe.HIncrBy(callCtx, key, strconv.Itoa(id), 1)
	}
	// A day's counts outlive the window that reads them by a day.
	pipe.Expire(callCtx, key, time.Duration(max(f.days(), 1)+1)*24*time.Hour)
	_, err := pipe.Exec(callCtx)
	f.c.observe(ctx, "foryou_record", err)
}
