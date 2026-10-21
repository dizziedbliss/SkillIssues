// Package claim lets a signed-in person say "I'm working on this issue" to
// everyone else on SkillIssues. A claim lives here only: it never touches
// GitHub, so the guide tells people to say so on the issue too.
//
// A claim is active until it is released, or until it expires: after
// api.claims.days, unless the claimer has an open pull request that closes
// the issue (tracked by internal/ingest/prs.go), which keeps it alive. One
// active claim per issue; at most api.claims.max_active per person.
package claim

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dizziedbliss/skillissues/internal/db"
)

// active is the one statement of the rule, for a claim row aliased c.
const active = `c.released_at IS NULL AND (c.expires_at > now() OR EXISTS (
	SELECT 1 FROM pull_request_issues l JOIN pull_requests p ON p.id = l.pull_request_id
	WHERE l.issue_id = c.issue_id AND p.user_id = c.user_id AND p.state = 'OPEN'))`

// Errors a claim can be refused with.
var (
	ErrNotFound = errors.New("claim: no such issue")
	ErrClosed   = errors.New("claim: the issue is closed")
	ErrTaken    = errors.New("claim: someone else has claimed it")
	ErrTooMany  = errors.New("claim: you hold as many claims as you may")
	ErrNotYours = errors.New("claim: you hold no claim on it")
)

// Claim is an active claim as its holder sees it.
type Claim struct {
	IssueID   int       `json:"issue_id"`
	ClaimedAt time.Time `json:"claimed_at"`
	ExpiresAt time.Time `json:"expires_at"`
	// Extended: an open pull request of yours that closes the issue keeps it
	// past ExpiresAt.
	Extended bool `json:"extended"`
}

// State is what a person sees of an issue's claim.
type State struct {
	Claimed bool   `json:"claimed"`
	Yours   *Claim `json:"yours"`
}

// Service reads and writes claims.
type Service struct{ db *db.Pool }

// New builds a Service.
func New(pool *db.Pool) *Service { return &Service{db: pool} }

// Claim claims an issue for userID, or returns their existing claim on it.
func (s *Service) Claim(ctx context.Context, userID, issueID int, days, maxActive int) (Claim, error) {
	var out Claim
	err := s.db.InTx(ctx, func(tx pgx.Tx) error {
		var state string
		if err := tx.QueryRow(ctx, `SELECT state FROM issues WHERE id = $1 FOR SHARE`, issueID).Scan(&state); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("claim: read issue: %w", err)
		}
		if state != "OPEN" {
			return ErrClosed
		}
		// Retire an expired claim on it, so the unique index lets this one in.
		if _, err := tx.Exec(ctx,
			`UPDATE issue_claims c SET released_at = now(), release_reason = 'expired'
			 WHERE c.issue_id = $1 AND c.released_at IS NULL AND NOT (`+active+`)`, issueID); err != nil {
			return fmt.Errorf("claim: retire an expired claim: %w", err)
		}
		var holder int
		err := tx.QueryRow(ctx, `SELECT user_id FROM issue_claims WHERE issue_id = $1 AND released_at IS NULL`, issueID).Scan(&holder)
		switch {
		case err == nil && holder == userID:
			return scanClaim(tx.QueryRow(ctx, claimSelect+` WHERE c.issue_id = $1 AND c.released_at IS NULL`, issueID), &out)
		case err == nil:
			return ErrTaken
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("claim: read the current claim: %w", err)
		}
		// Serialise one person's claims, so two at once can't both pass the cap.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('claim:' || $1::int::text))`, userID); err != nil {
			return fmt.Errorf("claim: lock: %w", err)
		}
		var held int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM issue_claims c WHERE c.user_id = $1 AND `+active, userID).Scan(&held); err != nil {
			return fmt.Errorf("claim: count your claims: %w", err)
		}
		if held >= maxActive {
			return ErrTooMany
		}
		err = scanClaim(tx.QueryRow(ctx,
			`INSERT INTO issue_claims AS c (issue_id, user_id, expires_at) VALUES ($1, $2, now() + make_interval(days => $3))
			 RETURNING c.issue_id, c.claimed_at, c.expires_at, false`, issueID, userID, days), &out)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrTaken // someone claimed it in the same instant
		}
		return err
	})
	return out, err
}

// Release gives up userID's claim on an issue.
func (s *Service) Release(ctx context.Context, userID, issueID int) error {
	var released bool
	err := s.db.QueryRow(ctx,
		`WITH r AS (UPDATE issue_claims SET released_at = now(), release_reason = 'released'
		            WHERE issue_id = $1 AND user_id = $2 AND released_at IS NULL RETURNING 1)
		 SELECT EXISTS (SELECT 1 FROM r)`, []any{issueID, userID},
		func(r pgx.Row) error { return r.Scan(&released) })
	if err != nil {
		return fmt.Errorf("claim: release: %w", err)
	}
	if !released {
		return ErrNotYours
	}
	return nil
}

// Of is the claim state of one issue as userID (0 when signed out) sees it.
func (s *Service) Of(ctx context.Context, userID, issueID int) (State, error) {
	var st State
	var c Claim
	var holder int
	err := s.db.QueryRow(ctx, `SELECT c.user_id, `+claimColumns+` FROM issue_claims c WHERE c.issue_id = $1 AND `+active,
		[]any{issueID}, func(r pgx.Row) error {
			return r.Scan(&holder, &c.IssueID, &c.ClaimedAt, &c.ExpiresAt, &c.Extended)
		})
	if errors.Is(err, db.ErrNotFound) {
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("claim: read: %w", err)
	}
	st.Claimed = true
	if userID != 0 && holder == userID {
		st.Yours = &c
	}
	return st, nil
}

// Mine is userID's active claims, soonest to expire first.
func (s *Service) Mine(ctx context.Context, userID int) ([]Claim, error) {
	out := []Claim{}
	err := s.db.QueryRows(ctx, claimSelect+` WHERE c.user_id = $1 AND `+active+` ORDER BY c.expires_at`,
		[]any{userID}, func(rows pgx.Rows) error {
			var c Claim
			if err := rows.Scan(&c.IssueID, &c.ClaimedAt, &c.ExpiresAt, &c.Extended); err != nil {
				return err
			}
			out = append(out, c)
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("claim: list yours: %w", err)
	}
	return out, nil
}

// ActiveIssues is every claimed issue, for lists (go-api keeps it in memory).
func (s *Service) ActiveIssues(ctx context.Context) (map[int]bool, error) {
	out := map[int]bool{}
	err := s.db.QueryRows(ctx, `SELECT c.issue_id FROM issue_claims c WHERE `+active, nil,
		func(rows pgx.Rows) error {
			var id int
			if err := rows.Scan(&id); err != nil {
				return err
			}
			out[id] = true
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("claim: read claimed issues: %w", err)
	}
	return out, nil
}

const claimColumns = `c.issue_id, c.claimed_at, c.expires_at, c.expires_at <= now()`

const claimSelect = `SELECT ` + claimColumns + ` FROM issue_claims c`

func scanClaim(row pgx.Row, c *Claim) error {
	if err := row.Scan(&c.IssueID, &c.ClaimedAt, &c.ExpiresAt, &c.Extended); err != nil {
		return err
	}
	return nil
}
