package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPoolExecution(t *testing.T) {
	p := NewPool("test-pool", 4, 16)
	defer p.Shutdown(1 * time.Second)

	var counter atomic.Int64
	jobCount := 20

	for i := 0; i < jobCount; i++ {
		err := p.SubmitWait(context.Background(), func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			counter.Add(1)
			return nil
		}, 2*time.Second)
		if err != nil {
			t.Fatalf("unexpected error submitting job: %v", err)
		}
	}

	time.Sleep(200 * time.Millisecond)
	if counter.Load() != int64(jobCount) {
		t.Fatalf("expected %d jobs completed, got %d", jobCount, counter.Load())
	}

	stats := p.Stats()
	if stats.JobsCompleted != int64(jobCount) {
		t.Fatalf("expected stats.JobsCompleted == %d, got %d", jobCount, stats.JobsCompleted)
	}
}

func TestWorkerPoolFailureAndPanicRecovery(t *testing.T) {
	p := NewPool("panic-pool", 2, 8)
	defer p.Shutdown(1 * time.Second)

	_ = p.Submit(func(ctx context.Context) error {
		return errors.New("simulated error")
	})

	_ = p.Submit(func(ctx context.Context) error {
		panic("boom")
	})

	time.Sleep(50 * time.Millisecond)
	stats := p.Stats()
	if stats.JobsFailed != 2 {
		t.Fatalf("expected 2 failed jobs, got %d", stats.JobsFailed)
	}
}

func TestWorkerPoolShutdownDrain(t *testing.T) {
	p := NewPool("shutdown-pool", 2, 10)

	var counter atomic.Int64
	for i := 0; i < 5; i++ {
		_ = p.Submit(func(ctx context.Context) error {
			time.Sleep(20 * time.Millisecond)
			counter.Add(1)
			return nil
		})
	}

	err := p.Shutdown(2 * time.Second)
	if err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}

	if counter.Load() != 5 {
		t.Fatalf("expected all 5 queued jobs to complete before shutdown, got %d", counter.Load())
	}

	// Submitting to closed pool should return ErrPoolClosed
	err = p.Submit(func(ctx context.Context) error { return nil })
	if err != ErrPoolClosed {
		t.Fatalf("expected ErrPoolClosed, got %v", err)
	}
}
