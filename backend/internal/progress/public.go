package progress

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/xp"
)

// ErrNotPublic is a profile that doesn't exist or isn't public: the two read
// the same, so a private account can't be told apart from no account.
var ErrNotPublic = errors.New("progress: no public profile by that name")

// Public is what anyone may see of someone who made their profile public.
type Public struct {
	Username  string     `json:"username"`
	Name      string     `json:"name"`
	AvatarURL string     `json:"avatar_url"`
	JoinedAt  time.Time  `json:"joined_at"`
	Summary   xp.Summary `json:"summary"`
	// Merged are their latest merged pull requests to catalog repositories.
	Merged    []PullSummary `json:"merged"`
	BadgeKeys []string      `json:"badge_keys"`
}

// PublicProfile reads login's public profile, if they made it public.
func (s *Service) PublicProfile(ctx context.Context, login string, step, pulls int, now time.Time) (Public, error) {
	var p Public
	var userID int
	err := s.db.QueryRow(ctx,
		`SELECT id, username, COALESCE(name, username), COALESCE(avatar_url, ''), created_at
		 FROM users WHERE lower(username) = lower($1) AND public_profile`,
		[]any{login}, func(r pgx.Row) error { return r.Scan(&userID, &p.Username, &p.Name, &p.AvatarURL, &p.JoinedAt) })
	if errors.Is(err, db.ErrNotFound) {
		return Public{}, ErrNotPublic
	}
	if err != nil {
		return Public{}, fmt.Errorf("progress: read public profile: %w", err)
	}
	events, err := s.Events(ctx, userID)
	if err != nil {
		return Public{}, err
	}
	p.Summary = xp.Summarise(events, step, now)
	p.BadgeKeys = xp.BadgeKeys()
	p.Merged = []PullSummary{}
	err = s.db.QueryRows(ctx,
		`SELECT p.id, r.full_name, p.number, p.title, p.url
		 FROM pull_requests p JOIN repositories r ON r.id = p.repository_id
		 WHERE p.user_id = $1 AND p.state = 'MERGED' AND p.no_xp_reason IS NULL
		 ORDER BY p.merged_at DESC LIMIT $2`, []any{userID, pulls}, func(rows pgx.Rows) error {
			var m PullSummary
			if err := rows.Scan(&m.ID, &m.Repository, &m.Number, &m.Title, &m.URL); err != nil {
				return err
			}
			p.Merged = append(p.Merged, m)
			return nil
		})
	if err != nil {
		return Public{}, fmt.Errorf("progress: read public merges: %w", err)
	}
	return p, nil
}

// Entry is one row of a leaderboard.
type Entry struct {
	Rank      int      `json:"rank"`
	Username  string   `json:"username"`
	AvatarURL string   `json:"avatar_url"`
	XP        int      `json:"xp"`
	Level     xp.Level `json:"level"`
}

// Leaderboard ranks people with public profiles by XP earned since since (nil
// for all time), in one language's repositories when language isn't empty.
// Levels are of the XP counted here.
func (s *Service) Leaderboard(ctx context.Context, since *time.Time, language string, step, limit int) ([]Entry, error) {
	out := []Entry{}
	err := s.db.QueryRows(ctx,
		`SELECT u.username, COALESCE(u.avatar_url, ''), sum(e.amount)::int AS xp
		 FROM xp_events e JOIN users u ON u.id = e.user_id
		 WHERE u.public_profile AND u.username IS NOT NULL
		   AND ($1::timestamptz IS NULL OR e.earned_at >= $1)
		   AND ($2 = '' OR e.detail->>'language' = $2)
		 GROUP BY u.id
		 HAVING sum(e.amount) > 0
		 ORDER BY xp DESC, lower(u.username)
		 LIMIT $3`, []any{since, language, limit}, func(rows pgx.Rows) error {
			var e Entry
			if err := rows.Scan(&e.Username, &e.AvatarURL, &e.XP); err != nil {
				return err
			}
			e.Rank = len(out) + 1
			e.Level = xp.LevelOf(e.XP, step)
			out = append(out, e)
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("progress: read leaderboard: %w", err)
	}
	return out, nil
}
