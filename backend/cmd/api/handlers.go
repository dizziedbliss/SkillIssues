// handlers.go — shared handler-struct definition. The actual HTTP handlers
// are split across handlers_*.go by domain — see handlers_health.go,
// handlers_session.go, handlers_auth_*.go, handlers_issues.go,
// handlers_saved.go, handlers_notifications.go, handlers_recommendations.go,
// handlers_profile.go, handlers_webhook.go, handlers_admin.go, and
// handlers_admin_config.go.
//
// Response shapes are consumed by the web and desktop clients, so changing a
// payload shape requires updating both.
package main

import (
	"net"

	"github.com/dizziedbliss/skillissues/internal/auth"
	"github.com/dizziedbliss/skillissues/internal/cache"
	"github.com/dizziedbliss/skillissues/internal/claim"
	"github.com/dizziedbliss/skillissues/internal/config"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/httpx"
	"github.com/dizziedbliss/skillissues/internal/issue"
	"github.com/dizziedbliss/skillissues/internal/notification"
	"github.com/dizziedbliss/skillissues/internal/profile"
	"github.com/dizziedbliss/skillissues/internal/progress"
	"github.com/dizziedbliss/skillissues/internal/report"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/dizziedbliss/skillissues/internal/theme"
	"golang.org/x/sync/singleflight"
)

// handlers bundles all service dependencies for the handler functions.
type handlers struct {
	cfg             *config.Config
	live            *settings.Live
	db              *db.Pool
	cache           *cache.Client
	authSvc         *auth.Service
	ghOAuth         *auth.GitHubOAuth
	outbound        httpx.Doer // calls to ingestd; carries the outbound timeout
	profileSvc      *profile.Service
	issueSvc        *issue.Service
	notificationSvc *notification.Service
	themeSvc        *theme.Service
	reportSvc       *report.Service
	progressSvc     *progress.Service
	claimSvc        *claim.Service
	claimed         *claimedStore
	hidden          *hiddenStore

	// trustedProxies are the CIDRs whose X-Forwarded-For is believed, for
	// per-caller caps beyond the global rate limiter. Nil without Redis,
	// where those caps refuse anyway.
	trustedProxies []*net.IPNet

	// issuesFlight collapses concurrent identical signed-out catalog
	// requests that miss the cache into one query (handlers_issues.go).
	issuesFlight singleflight.Group
}
