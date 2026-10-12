package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLimiterBoundsConcurrency(t *testing.T) {
	l := NewLimiter(3)
	var inFlight, peak atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.Acquire(context.Background()); err != nil {
				t.Error(err)
				return
			}
			defer l.Release()

			n := inFlight.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			inFlight.Add(-1)
		}()
	}
	wg.Wait()

	if got := peak.Load(); got > 3 {
		t.Errorf("peak concurrency %d exceeded limit of 3", got)
	}
	if l.InUse() != 0 {
		t.Errorf("expected all slots released, %d still in use", l.InUse())
	}
}

// Raising the limit must admit already-queued waiters, not just future ones.
// This is the property that makes the setting hot-reloadable.
func TestLimiterRaisingLimitAdmitsWaiters(t *testing.T) {
	l := NewLimiter(1)

	if err := l.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	admitted := make(chan struct{})
	go func() {
		if err := l.Acquire(context.Background()); err != nil {
			t.Error(err)
			return
		}
		close(admitted)
	}()

	select {
	case <-admitted:
		t.Fatal("second acquire should block while the limit is 1")
	case <-time.After(50 * time.Millisecond):
	}

	l.SetLimit(2)

	select {
	case <-admitted:
	case <-time.After(time.Second):
		t.Fatal("raising the limit did not admit the queued waiter")
	}
}

// Lowering the limit must not preempt running work, only stop new admissions.
func TestLimiterLoweringLimitDoesNotPreempt(t *testing.T) {
	l := NewLimiter(4)
	for i := 0; i < 4; i++ {
		if err := l.Acquire(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	l.SetLimit(1)

	if l.InUse() != 4 {
		t.Errorf("running work should be untouched, got %d in use", l.InUse())
	}

	// No slot is free until usage falls back below the new limit.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := l.Acquire(ctx); err == nil {
		t.Error("expected acquire to block while over the lowered limit")
	}

	for i := 0; i < 4; i++ {
		l.Release()
	}
	if err := l.Acquire(context.Background()); err != nil {
		t.Errorf("expected a slot after draining: %v", err)
	}
}

// A cancelled Acquire must not consume capacity. Leaking here would shrink the
// effective limit on every timeout until the pool stalled completely.
func TestLimiterCancelledAcquireDoesNotLeak(t *testing.T) {
	l := NewLimiter(1)

	if err := l.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		if err := l.Acquire(ctx); err == nil {
			t.Fatal("expected the acquire to time out")
		}
		cancel()
	}

	l.Release()

	if got := l.InUse(); got != 0 {
		t.Fatalf("capacity leaked: %d slots still held after release", got)
	}
	if err := l.Acquire(context.Background()); err != nil {
		t.Fatalf("limiter unusable after cancellations: %v", err)
	}
}

func TestLimiterRejectsNonPositiveLimit(t *testing.T) {
	if got := NewLimiter(0).Limit(); got != 1 {
		t.Errorf("NewLimiter(0): got %d, want 1", got)
	}
	l := NewLimiter(5)
	l.SetLimit(-3)
	if got := l.Limit(); got != 1 {
		t.Errorf("SetLimit(-3): got %d, want 1", got)
	}
}

func TestPoolSetConcurrencyIsReflectedInStats(t *testing.T) {
	p := NewPool("resize-pool", 2, 8)
	defer p.Shutdown(time.Second)

	if got := p.Stats().Concurrency; got != 2 {
		t.Fatalf("initial concurrency: got %d, want 2", got)
	}
	p.SetConcurrency(9)
	if got := p.Stats().Concurrency; got != 9 {
		t.Errorf("after resize: got %d, want 9", got)
	}
}
