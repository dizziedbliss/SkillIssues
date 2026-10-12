package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	notifyChannel = "settings_changed"

	// reconcileInterval backstops the listener. NOTIFY is not durable: a change
	// committed while the listener is disconnected is never delivered, so
	// without this poll a missed notification would persist until restart.
	reconcileInterval = 60 * time.Second

	// listenerRetryBase grows to listenerRetryMax while the dedicated
	// connection cannot be established.
	listenerRetryBase = 1 * time.Second
	listenerRetryMax  = 30 * time.Second
)

// Live holds the current settings snapshot and keeps it fresh.
//
// Reads go through Current(), which is a single atomic load: no lock, no
// contention, and the returned snapshot is immutable.
type Live struct {
	pool *pgxpool.Pool
	dsn  string
	snap atomic.Pointer[Snapshot]

	subMu       sync.Mutex
	subscribers []func(*Snapshot)
}

// NewLive loads the settings once and returns a handle. It never fails on a
// read error: the process starts on registry defaults and the reconcile loop
// recovers, because refusing to boot over a settings read would make the
// config system a new single point of failure.
func NewLive(ctx context.Context, pool *pgxpool.Pool, dsn string) *Live {
	l := &Live{pool: pool, dsn: dsn}
	l.snap.Store(defaultSnapshot())

	if err := l.reload(ctx); err != nil {
		slog.Warn("settings load failed; continuing on defaults", "error", err)
	}
	return l
}

// Current returns the active snapshot.
func (l *Live) Current() *Snapshot { return l.snap.Load() }

// OnChange registers a callback invoked after every successful reload.
//
// Most values are read per operation and need nothing here. This exists for
// state that has to be pushed somewhere on change rather than polled — worker
// pool concurrency being the motivating case, since otherwise a new limit would
// not apply until the next sync cycle, up to fifteen minutes later.
//
// Callbacks run on the reload goroutine, so they must be cheap and must not
// block. Register during startup wiring only.
func (l *Live) OnChange(fn func(*Snapshot)) {
	if fn == nil {
		return
	}
	l.subMu.Lock()
	l.subscribers = append(l.subscribers, fn)
	l.subMu.Unlock()

	// Apply immediately so a subscriber does not wait for the first change.
	fn(l.Current())
}

// publish fans a new snapshot out to subscribers. A panicking subscriber must
// not take down the listener, so each call is isolated.
func (l *Live) publish(s *Snapshot) {
	l.subMu.Lock()
	subs := make([]func(*Snapshot), len(l.subscribers))
	copy(subs, l.subscribers)
	l.subMu.Unlock()

	for _, fn := range subs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("settings subscriber panicked", "panic", r)
				}
			}()
			fn(s)
		}()
	}
}

// Start runs the notify listener and the reconcile poll until ctx is done.
func (l *Live) Start(ctx context.Context) {
	go l.listen(ctx)
	go l.reconcile(ctx)
}

// reload reads every row and atomically swaps in a new snapshot. A snapshot
// that cannot be built is discarded and the previous one kept, so one bad write
// cannot leave a process unconfigured.
func (l *Live) reload(ctx context.Context) error {
	rows, err := l.pool.Query(ctx, `SELECT key, value, value_type FROM settings`)
	if err != nil {
		return fmt.Errorf("query settings: %w", err)
	}
	defer rows.Close()

	values := make(map[Key]any, len(Registry))
	var skipped int

	for rows.Next() {
		var key, valueType string
		var raw []byte
		if err := rows.Scan(&key, &raw, &valueType); err != nil {
			return fmt.Errorf("scan setting: %w", err)
		}

		v, err := decode(raw, Kind(valueType))
		if err != nil {
			// Keep going: one malformed row should not discard every other value.
			slog.Warn("ignoring unusable setting", "key", key, "error", err)
			skipped++
			continue
		}
		values[Key(key)] = v
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate settings: %w", err)
	}

	// Fill anything the table does not define so callers always get a value.
	var missing []string
	for k, spec := range Registry {
		if _, ok := values[k]; !ok {
			values[k] = spec.Default
			missing = append(missing, string(k))
		}
	}
	if len(missing) > 0 {
		slog.Warn("settings keys absent from the database; using defaults",
			"keys", missing, "hint", "add them in a migration")
	}

	snap := newSnapshot(values)
	l.snap.Store(snap)
	l.publish(snap)

	if skipped > 0 {
		slog.Warn("settings reloaded with skipped rows", "skipped", skipped)
	}
	return nil
}

func decode(raw []byte, kind Kind) (any, error) {
	switch kind {
	case KindInt:
		var f float64
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("expected a number: %w", err)
		}
		if f != float64(int(f)) {
			return nil, fmt.Errorf("expected a whole number, got %v", f)
		}
		return int(f), nil
	case KindBool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, fmt.Errorf("expected a boolean: %w", err)
		}
		return b, nil
	case KindString:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("expected a string: %w", err)
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unknown value type %q", kind)
	}
}

// listen holds a dedicated connection for LISTEN. Listening is connection
// scoped, so it cannot share the pool: a pooled connection would be returned
// between queries and silently stop delivering notifications.
func (l *Live) listen(ctx context.Context) {
	backoff := listenerRetryBase

	for ctx.Err() == nil {
		conn, err := pgx.Connect(ctx, l.dsn)
		if err != nil {
			slog.Warn("settings listener connect failed; retrying",
				"error", err, "retry_in", backoff)
			if !sleepCtx(ctx, backoff) {
				return
			}
			backoff = min(backoff*2, listenerRetryMax)
			continue
		}

		if _, err := conn.Exec(ctx, "LISTEN "+notifyChannel); err != nil {
			slog.Warn("settings LISTEN failed; retrying", "error", err)
			_ = conn.Close(ctx)
			if !sleepCtx(ctx, backoff) {
				return
			}
			backoff = min(backoff*2, listenerRetryMax)
			continue
		}

		slog.Info("settings listener connected", "channel", notifyChannel)
		backoff = listenerRetryBase

		// A reload here closes the gap between the last reconcile and this
		// subscription becoming active.
		if err := l.reload(ctx); err != nil {
			slog.Warn("settings reload after subscribe failed", "error", err)
		}

		for {
			n, err := conn.WaitForNotification(ctx)
			if err != nil {
				if ctx.Err() == nil {
					slog.Warn("settings listener dropped; reconnecting", "error", err)
				}
				break
			}
			slog.Info("settings change notified", "key", n.Payload)
			if err := l.reload(ctx); err != nil {
				slog.Warn("settings reload failed; keeping previous values", "error", err)
			}
		}

		_ = conn.Close(context.WithoutCancel(ctx))
	}
}

// reconcile reloads unconditionally on a timer. This is what makes a missed
// NOTIFY self-healing rather than permanent.
func (l *Live) reconcile(ctx context.Context) {
	t := time.NewTicker(reconcileInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := l.reload(ctx); err != nil {
				slog.Warn("settings reconcile failed; keeping previous values", "error", err)
			}
		}
	}
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
