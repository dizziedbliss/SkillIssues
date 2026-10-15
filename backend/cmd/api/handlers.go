// handlers.go — shared handler-struct definition and the desktop OAuth
// ticket bookkeeping used by both the classic OAuth flow and the GitHub
// App flow. The actual HTTP handlers are split across handlers_*.go by
// domain (D3) — see handlers_health.go, handlers_session.go,
// handlers_auth_*.go, handlers_github.go, handlers_issues.go,
// handlers_saved.go, handlers_recommendations.go, handlers_profile.go,
// handlers_contributions.go, handlers_webhook.go, handlers_admin.go, and
// handlers_admin_config.go.
//
// Response shapes are consumed by both the web frontend and the Electron
// desktop app, so changing a payload shape requires updating both clients.
package main

import (
	"context"
	"sync"
	"time"

	"github.com/dizziedbliss/skillissues/internal/auth"
	"github.com/dizziedbliss/skillissues/internal/cache"
	"github.com/dizziedbliss/skillissues/internal/config"
	"github.com/dizziedbliss/skillissues/internal/contribution"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/github"
	"github.com/dizziedbliss/skillissues/internal/githubapp"
	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/notification"
	"github.com/dizziedbliss/skillissues/internal/profile"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// ticketTTL bounds how long a desktop OAuth ticket stays claimable. Without
// an expiry, a ticket that's never polled (or polled only once and left
// unconsumed) would hand out a live session to whoever holds it, forever.
const ticketTTL = 10 * time.Minute

// ticketEntry pairs stored ticket data with when it was created, so expired
// entries can be rejected and swept instead of living in memory forever.
type ticketEntry struct {
	data      any
	createdAt time.Time
}

func (t ticketEntry) expired() bool { return time.Since(t.createdAt) > ticketTTL }

// handlers bundles all service dependencies for the handler functions.
type handlers struct {
	cfg             *config.Config
	live            *settings.Live
	db              *db.Pool
	cache           *cache.Client
	authSvc         *auth.Service
	ghOAuth         *auth.GitHubOAuth
	ghAppLogin      *auth.GitHubAppLogin // nil until the GitHub App is configured
	ghApp           *githubapp.App       // nil until the GitHub App is configured (D23 bot identity)
	profileSvc      *profile.Service
	issueSvc        *issue.Service
	contribSvc      *contribution.Service
	notificationSvc *notification.Service
	ghClient        *github.Client
	bodyFetcher     *issue.BodyFetcher
	desktopTickets  sync.Map // ticket -> ticketEntry{data: session data}
	stateTickets    sync.Map // state -> ticketEntry{data: ticket string}
}

// sweepExpiredTickets periodically drops stale entries from desktopTickets
// and stateTickets so abandoned OAuth attempts don't accumulate forever.
func (h *handlers) sweepExpiredTickets(ctx context.Context) {
	ticker := time.NewTicker(ticketTTL)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, m := range []*sync.Map{&h.desktopTickets, &h.stateTickets} {
				m.Range(func(key, value any) bool {
					if entry, ok := value.(ticketEntry); ok && entry.expired() {
						m.Delete(key)
					}
					return true
				})
			}
		}
	}
}
