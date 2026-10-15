package cache

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
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
	c, err := New(url, fixedLimits(time.Second, time.Second))
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

func fixedLimits(timeout, retry time.Duration) func() Limits {
	return func() Limits { return Limits{CallTimeout: timeout, RetryAfter: retry} }
}

// unreachableClient points at TEST-NET-1 (RFC 5737), which nothing answers:
// connection attempts hang until a timeout, like a stopped container's
// address. A refused port would fail instantly and hide a missing deadline.
func unreachableClient(t *testing.T, timeout, retry time.Duration) *Client {
	t.Helper()
	c, err := New("redis://192.0.2.1:6379/0", fixedLimits(timeout, retry))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// An unreachable Redis must fail open within the call deadline, not after
// the go-redis dial timeout and retries (~20s), and the failure is recorded.
func TestCheckRateLimitFailsOpenFast(t *testing.T) {
	c := unreachableClient(t, 100*time.Millisecond, time.Minute)

	start := time.Now()
	res := c.CheckRateLimit(context.Background(), "unreachable", 1, time.Minute)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("fail-open took %v, want about the 100ms call deadline", elapsed)
	}
	if !res.Allowed {
		t.Fatalf("unreachable Redis must fail open, got %+v", res)
	}
	if !c.down.Load() {
		t.Fatal("failure was not recorded")
	}
}

// Once a call has failed, later calls skip Redis entirely until the retry
// interval passes, so an outage costs one deadline, not one per call.
func TestDownRedisIsSkippedUntilRetry(t *testing.T) {
	c := unreachableClient(t, 100*time.Millisecond, time.Minute)
	ctx := context.Background()
	c.GetRecommendations(ctx, 1) // fails and marks Redis down

	start := time.Now()
	for i := 0; i < 50; i++ {
		if res := c.CheckRateLimit(ctx, "skipped", 1, time.Minute); !res.Allowed {
			t.Fatalf("call %d did not fail open: %+v", i, res)
		}
		if c.GetHotIssues(ctx) != nil {
			t.Fatalf("call %d returned data from a down Redis", i)
		}
		c.SetCachedSearch(ctx, "k", []byte("v"), time.Minute)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("150 calls to a down Redis took %v; they should skip it", elapsed)
	}
}

// After the retry interval exactly one call probes Redis; concurrent callers
// do not each pay the deadline.
func TestDownRedisSendsOneProbePerInterval(t *testing.T) {
	c := unreachableClient(t, 200*time.Millisecond, 50*time.Millisecond)
	ctx := context.Background()
	c.GetRecommendations(ctx, 1)
	time.Sleep(60 * time.Millisecond) // past the retry interval

	const callers = 20
	var wg sync.WaitGroup
	var probed atomic.Int32
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			c.GetRecommendations(ctx, 1)
			if time.Since(start) >= 150*time.Millisecond {
				probed.Add(1)
			}
		}()
	}
	wg.Wait()
	if n := probed.Load(); n != 1 {
		t.Fatalf("%d of %d callers waited on Redis after the retry interval, want exactly 1", n, callers)
	}
}

// A request cancelled by its caller is not a Redis failure and must not make
// every other request skip Redis.
func TestCancelledRequestDoesNotMarkRedisDown(t *testing.T) {
	c := unreachableClient(t, time.Second, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.GetRecommendations(ctx, 1)
	if c.down.Load() {
		t.Fatal("a cancelled request marked Redis down")
	}
}

// Ping reports Redis's current state even while calls are being skipped, and
// a successful one ends the outage without waiting for the retry interval.
func TestPingBypassesSkipAndRecovers(t *testing.T) {
	c := testClient(t)
	c.limits = fixedLimits(time.Second, time.Hour)
	c.probeAt.Store(time.Now().Add(time.Hour).UnixNano())
	c.down.Store(true)

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping skipped or failed: %v", err)
	}
	if c.down.Load() {
		t.Fatal("successful Ping did not end the outage")
	}
}
