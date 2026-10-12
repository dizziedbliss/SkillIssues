package worker

import (
	"context"
	"sync"
)

// Limiter bounds concurrency to a limit that can change while in use.
//
// A buffered channel cannot be resized and golang.org/x/sync/semaphore has no
// resize either, so neither can back a hot-reloadable concurrency setting.
// This keeps an explicit FIFO queue of waiters instead: raising the limit
// admits queued waiters immediately, and lowering it lets running work drain
// rather than preempting it.
type Limiter struct {
	mu      sync.Mutex
	limit   int
	inUse   int
	waiters []chan struct{}
}

func NewLimiter(limit int) *Limiter {
	if limit < 1 {
		limit = 1
	}
	return &Limiter{limit: limit}
}

// Acquire takes a slot, blocking until one is free or ctx is done.
func (l *Limiter) Acquire(ctx context.Context) error {
	l.mu.Lock()
	if l.inUse < l.limit {
		l.inUse++
		l.mu.Unlock()
		return nil
	}
	ch := make(chan struct{})
	l.waiters = append(l.waiters, ch)
	l.mu.Unlock()

	select {
	case <-ch:
		// Woken by a release or a raised limit, which already reserved the slot.
		return nil
	case <-ctx.Done():
		l.abandon(ch)
		return ctx.Err()
	}
}

// Release returns a slot and hands it to the longest-waiting caller.
func (l *Limiter) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inUse > 0 {
		l.inUse--
	}
	l.admitLocked()
}

// SetLimit changes the ceiling. Raising it admits waiters at once; lowering it
// below the number in flight does not cancel anything, it just stops new
// acquisitions until usage falls back under the new limit.
func (l *Limiter) SetLimit(limit int) {
	if limit < 1 {
		limit = 1
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.limit == limit {
		return
	}
	l.limit = limit
	l.admitLocked()
}

func (l *Limiter) Limit() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.limit
}

func (l *Limiter) InUse() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inUse
}

// admitLocked wakes as many waiters as there is now room for.
func (l *Limiter) admitLocked() {
	for l.inUse < l.limit && len(l.waiters) > 0 {
		ch := l.waiters[0]
		l.waiters = l.waiters[1:]
		l.inUse++
		close(ch)
	}
}

// abandon removes a cancelled waiter. If it is already gone, a slot was
// reserved for it concurrently and must be given back — otherwise a cancelled
// Acquire would leak capacity permanently.
func (l *Limiter) abandon(ch chan struct{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for i, w := range l.waiters {
		if w == ch {
			l.waiters = append(l.waiters[:i], l.waiters[i+1:]...)
			return
		}
	}

	if l.inUse > 0 {
		l.inUse--
	}
	l.admitLocked()
}
