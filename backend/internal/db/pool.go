// Package db provides a PostgreSQL connection pool and query execution utilities.
// Uses pgx/v5 with the native protocol driver for maximum performance.
// All queries are parameterized — no string interpolation of user input.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/dizziedbliss/skillissues/internal/flow"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps pgxpool.Pool and provides helper methods for common query patterns.
type Pool struct {
	pool *pgxpool.Pool
}

// Raw exposes the underlying pgxpool for callers that need pgx APIs the
// helpers above do not wrap, such as streaming a result set directly.
func (p *Pool) Raw() *pgxpool.Pool { return p.pool }

// Config holds pool configuration parameters.
type Config struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// DefaultConfig returns sane production defaults for the connection pool.
func DefaultConfig(databaseURL string) Config {
	return Config{
		URL:             databaseURL,
		MaxConns:        20,
		MinConns:        2,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
	}
}

// New creates and validates a new connection pool. It verifies connectivity
// via a test query before returning. The caller is responsible for calling Close().
func New(ctx context.Context, cfg Config) (*Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime

	// Every statement is a hop in the live visualizer: verb and table only,
	// never the SQL or its arguments. A no-op in a process that installs no
	// flow publisher.
	poolCfg.ConnConfig.Tracer = flow.QueryTracer{}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	// Verify connectivity immediately so startup fails fast rather than at first request
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping failed: %w", err)
	}

	return &Pool{pool: pool}, nil
}

// Close releases all connections in the pool. Should be deferred in main().
func (p *Pool) Close() {
	p.pool.Close()
}

// Ping checks that the database is reachable. Used by health check endpoint.
func (p *Pool) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// QueryRows executes a SELECT query and returns all rows scanned by the provided
// scan function. Returns an empty slice (never nil) when no rows match.
func (p *Pool) QueryRows(ctx context.Context, sql string, args []any, scan func(pgx.Rows) error) error {
	rows, err := p.pool.Query(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("db: query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		if err := scan(rows); err != nil {
			return fmt.Errorf("db: scan row: %w", err)
		}
	}
	return rows.Err()
}

// QueryRow executes a SELECT that is expected to return exactly one row.
// Returns ErrNoRows if no row is found (wrapped for caller inspection).
func (p *Pool) QueryRow(ctx context.Context, sql string, args []any, scan func(pgx.Row) error) error {
	row := p.pool.QueryRow(ctx, sql, args...)
	if err := scan(row); err != nil {
		if err == pgx.ErrNoRows {
			return ErrNotFound
		}
		return fmt.Errorf("db: query row: %w", err)
	}
	return nil
}

// Exec executes a statement that returns no rows (INSERT, UPDATE, DELETE).
func (p *Pool) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := p.pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("db: exec: %w", err)
	}
	return nil
}

// ExecReturning executes a statement and scans the single returned row (e.g. INSERT ... RETURNING id).
func (p *Pool) ExecReturning(ctx context.Context, sql string, args []any, scan func(pgx.Row) error) error {
	row := p.pool.QueryRow(ctx, sql, args...)
	if err := scan(row); err != nil {
		return fmt.Errorf("db: exec returning: %w", err)
	}
	return nil
}

// InTx executes fn inside a database transaction. If fn returns an error,
// the transaction is rolled back. On success it is committed.
// Panics are recovered and cause a rollback.
func (p *Pool) InTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback(ctx)
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit tx: %w", err)
	}
	return nil
}

// Stats returns current pool statistics for metrics/observability.
func (p *Pool) Stats() *pgxpool.Stat {
	return p.pool.Stat()
}

// ErrNotFound is returned when a query returns no rows.
var ErrNotFound = fmt.Errorf("db: record not found")
