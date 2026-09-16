// Package worker provides bounded, concurrent worker pools with graceful shutdown,
// queue metrics, and context-based job execution.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrQueueFull is returned when a non-blocking Submit cannot enqueue the job.
	ErrQueueFull = errors.New("worker pool queue is full")
	// ErrPoolClosed is returned when attempting to submit a job to a stopped pool.
	ErrPoolClosed = errors.New("worker pool is stopped")
)

// Job represents an executable unit of work.
type Job func(ctx context.Context) error

// PoolStats reports live telemetry for a worker pool.
type PoolStats struct {
	Name          string `json:"name"`
	Concurrency   int    `json:"concurrency"`
	QueueCapacity int    `json:"queue_capacity"`
	QueueLength   int    `json:"queue_length"`
	JobsSubmitted int64  `json:"jobs_submitted"`
	JobsCompleted int64  `json:"jobs_completed"`
	JobsFailed    int64  `json:"jobs_failed"`
	ActiveWorkers int64  `json:"active_workers"`
}

// Pool manages a fixed set of goroutines consuming from a bounded job channel.
type Pool struct {
	name          string
	concurrency   int
	queueCap      int
	jobs          chan Job
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	mu            sync.RWMutex
	closed        bool
	submitted     atomic.Int64
	completed     atomic.Int64
	failed        atomic.Int64
	activeWorkers atomic.Int64
}

// NewPool initializes and starts a worker pool with the specified concurrency and queue depth.
func NewPool(name string, concurrency int, queueCap int) *Pool {
	if concurrency <= 0 {
		concurrency = 1
	}
	if queueCap <= 0 {
		queueCap = concurrency * 4
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		name:        name,
		concurrency: concurrency,
		queueCap:    queueCap,
		jobs:        make(chan Job, queueCap),
		ctx:         ctx,
		cancel:      cancel,
	}

	for i := 0; i < concurrency; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	slog.Info("worker pool started", "name", name, "concurrency", concurrency, "queue_cap", queueCap)
	return p
}

// worker is the worker goroutine loop.
func (p *Pool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			p.activeWorkers.Add(1)
			p.runJob(job)
			p.activeWorkers.Add(-1)
		}
	}
}

func (p *Pool) runJob(job Job) {
	defer func() {
		if r := recover(); r != nil {
			p.failed.Add(1)
			slog.Error("worker job panicked", "pool", p.name, "panic", fmt.Sprint(r))
		}
	}()

	if err := job(p.ctx); err != nil {
		p.failed.Add(1)
		slog.Warn("worker job failed", "pool", p.name, "error", err)
	} else {
		p.completed.Add(1)
	}
}

// Submit enqueues a job without blocking. Returns ErrQueueFull if channel is full.
func (p *Pool) Submit(job Job) error {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return ErrPoolClosed
	}
	p.mu.RUnlock()

	select {
	case p.jobs <- job:
		p.submitted.Add(1)
		return nil
	default:
		return ErrQueueFull
	}
}

// SubmitWait enqueues a job, waiting up to timeout duration if the queue is full.
func (p *Pool) SubmitWait(ctx context.Context, job Job, timeout time.Duration) error {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return ErrPoolClosed
	}
	p.mu.RUnlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-p.ctx.Done():
		return ErrPoolClosed
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ErrQueueFull
	case p.jobs <- job:
		p.submitted.Add(1)
		return nil
	}
}

// Shutdown initiates graceful termination of the worker pool, draining or waiting up to timeout.
func (p *Pool) Shutdown(timeout time.Duration) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	close(p.jobs)
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.cancel()
		slog.Info("worker pool shut down cleanly", "name", p.name)
		return nil
	case <-time.After(timeout):
		p.cancel()
		slog.Warn("worker pool shutdown timed out, cancelling context", "name", p.name)
		return errors.New("worker pool shutdown timed out")
	}
}

// Stats returns a snapshot of pool telemetry.
func (p *Pool) Stats() PoolStats {
	return PoolStats{
		Name:          p.name,
		Concurrency:   p.concurrency,
		QueueCapacity: p.queueCap,
		QueueLength:   len(p.jobs),
		JobsSubmitted: p.submitted.Load(),
		JobsCompleted: p.completed.Load(),
		JobsFailed:    p.failed.Load(),
		ActiveWorkers: p.activeWorkers.Load(),
	}
}
