package cache

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// Serves recorded today are kept but not counted until tomorrow, so a chunk
// drawn twice in a day sees the same counts; the days before today, up to
// days(), are counted.
func TestForYouCountsOnlyDaysBeforeToday(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	id := int(time.Now().UnixNano() % 1_000_000_000) // a field no other run has written
	today := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	f := &ForYou{c: c, days: func() int { return 2 }}
	t.Cleanup(func() {
		for d := 0; d <= 3; d++ {
			c.rdb.HDel(ctx, shownKey(today.AddDate(0, 0, -d)), strconv.Itoa(id))
		}
	})

	record := func(day time.Time, n int) {
		f.now = func() time.Time { return day }
		for range n {
			f.RecordShown(ctx, []int{id})
		}
	}
	record(today.AddDate(0, 0, -3), 8) // outside the window
	record(today.AddDate(0, 0, -2), 4)
	record(today.AddDate(0, 0, -1), 2)

	f.now = func() time.Time { return today }
	before := f.ShownCounts(ctx, []int{id})[id]
	if before != 6 {
		t.Fatalf("counted %d serves, want 6 (the two days before today)", before)
	}
	record(today, 5)
	f.now = func() time.Time { return today }
	if got := f.ShownCounts(ctx, []int{id})[id]; got != before {
		t.Fatalf("today's serves changed the count: %d, then %d", before, got)
	}
}
