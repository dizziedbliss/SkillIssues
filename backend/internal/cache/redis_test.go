package cache

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// testClient connects to REDIS_TEST_URL. Tests must pass with Redis stopped,
// so without it (or with -short) these are skipped, not failed.
func testClient(t *testing.T) *Client {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping Redis test in short mode")
	}
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		t.Skip("REDIS_TEST_URL not set")
	}
	c, err := New(url)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if err := c.Ping(context.Background()); err != nil {
		t.Skipf("Redis unreachable: %v", err)
	}
	return c
}

// A steady client must not keep its window open forever: the expiry set by
// the first request is never pushed back by later ones.
func TestCheckRateLimitWindowDoesNotSlide(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	key := fmt.Sprintf("test:%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = c.rdb.Del(ctx, "rl:"+key).Err() })

	const window = 2 * time.Second
	first := c.CheckRateLimit(ctx, key, 2, window)
	if !first.Allowed || first.Remaining != 1 {
		t.Fatalf("first request: %+v", first)
	}

	time.Sleep(1200 * time.Millisecond)
	second := c.CheckRateLimit(ctx, key, 2, window)
	if !second.Allowed {
		t.Fatalf("second request rejected: %+v", second)
	}
	if second.ResetAfter > time.Second {
		t.Fatalf("expiry was pushed back: ResetAfter=%v, want <= 1s", second.ResetAfter)
	}
	if third := c.CheckRateLimit(ctx, key, 2, window); third.Allowed {
		t.Fatalf("third request allowed over a limit of 2: %+v", third)
	}

	time.Sleep(time.Second)
	if after := c.CheckRateLimit(ctx, key, 2, window); !after.Allowed || after.Remaining != 1 {
		t.Fatalf("window did not reset %v after the first request: %+v", window, after)
	}
}

func TestCheckRateLimitFailsOpen(t *testing.T) {
	c, err := New("redis://127.0.0.1:1/0")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if res := c.CheckRateLimit(ctx, "unreachable", 1, time.Minute); !res.Allowed {
		t.Fatalf("unreachable Redis must fail open, got %+v", res)
	}
	if !c.down.Load() {
		t.Fatal("failure was not recorded")
	}
}
