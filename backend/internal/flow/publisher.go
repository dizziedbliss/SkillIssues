package flow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/redis/go-redis/v9"
)

// Limits are the live settings a Publisher reads on every Emit.
type Limits struct {
	Enabled        bool
	BufferSize     int
	PublishTimeout time.Duration
	MaxLabelChars  int
}

// sink is where a batch goes; Redis in production, a fake in tests.
type sink interface {
	publish(ctx context.Context, msg []byte) (subscribers int64, err error)
}

type redisSink struct{ rdb *redis.Client }

func (s redisSink) publish(ctx context.Context, msg []byte) (int64, error) {
	return s.rdb.Publish(ctx, Channel, msg).Result()
}

// Publisher buffers emitted events and publishes them to Redis in batches:
// one PUBLISH carries everything that queued while the previous one was in
// flight, as a JSON array. Nothing waits on it. A full buffer, a Redis error
// or a missing subscriber drops events and counts them in
// flow_events_dropped_total, so the visualizer can never slow a request or
// fail the crawl.
type Publisher struct {
	self   Node
	sink   sink
	close  func() error
	limits func() Limits

	mu   sync.Mutex
	buf  []Event
	wake chan struct{}

	// down is set while Redis is failing, so an outage is logged when it
	// starts and when it ends rather than on every batch.
	down atomic.Bool
}

// NewPublisher returns a publisher for the process that is node self. It
// does not contact Redis: the process runs the same without it.
func NewPublisher(self Node, redisURL string, limits func() Limits) (*Publisher, error) {
	if !self.Valid() {
		return nil, fmt.Errorf("flow: unknown node %q", self)
	}
	if limits == nil {
		return nil, errors.New("flow: limits must not be nil")
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("flow: parse redis URL: %w", err)
	}
	rdb := redis.NewClient(opts)
	return newPublisher(self, redisSink{rdb}, rdb.Close, limits), nil
}

// NewPublisherFunc returns a publisher that hands each batch to publish
// instead of Redis; publish returns how many subscribers received it. For
// tests that need to see exactly what would leave the process.
func NewPublisherFunc(self Node, publish func(ctx context.Context, msg []byte) (int64, error), limits func() Limits) *Publisher {
	return newPublisher(self, funcSink(publish), func() error { return nil }, limits)
}

type funcSink func(ctx context.Context, msg []byte) (int64, error)

func (f funcSink) publish(ctx context.Context, msg []byte) (int64, error) { return f(ctx, msg) }

func newPublisher(self Node, s sink, close func() error, limits func() Limits) *Publisher {
	return &Publisher{self: self, sink: s, close: close, limits: limits, wake: make(chan struct{}, 1)}
}

// Close releases the Redis connection. Uninstall the publisher first.
func (p *Publisher) Close() error { return p.close() }

func (p *Publisher) enqueue(e Event, limit int) {
	p.mu.Lock()
	if len(p.buf) >= limit {
		p.mu.Unlock()
		metrics.FlowEventsDropped.WithLabelValues("buffer_full").Inc()
		return
	}
	p.buf = append(p.buf, e)
	p.mu.Unlock()
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// Run publishes queued events until ctx is done.
func (p *Publisher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.wake:
			p.Flush(ctx)
		}
	}
}

// Flush publishes everything queued now. Run calls it; tests call it to
// publish synchronously.
func (p *Publisher) Flush(ctx context.Context) {
	p.mu.Lock()
	batch := p.buf
	p.buf = nil
	p.mu.Unlock()
	if len(batch) == 0 {
		return
	}

	msg, err := json.Marshal(batch)
	if err != nil {
		// Every field is a string, number or time; this cannot happen, but a
		// dropped batch must still be visible if it does.
		metrics.FlowEventsDropped.WithLabelValues("encode").Add(float64(len(batch)))
		slog.Error("flow: encode batch", "error", err)
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, p.limits().PublishTimeout)
	subscribers, err := p.sink.publish(callCtx, msg)
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		metrics.FlowEventsDropped.WithLabelValues("redis").Add(float64(len(batch)))
		if p.down.CompareAndSwap(false, true) {
			slog.Warn("flow: publish to redis failed; events are dropped until it recovers", "error", err)
		}
		return
	}
	if p.down.CompareAndSwap(true, false) {
		slog.Info("flow: publishing to redis again")
	}
	if subscribers == 0 {
		metrics.FlowEventsDropped.WithLabelValues("no_subscriber").Add(float64(len(batch)))
		return
	}
	metrics.FlowEventsPublished.Add(float64(len(batch)))
}
