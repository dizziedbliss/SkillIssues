package settings

import (
	"net/http"
	"time"

	"github.com/dizziedbliss/skillissues/internal/httpx"
)

// OutboundClient returns the client for calls to GitHub and sibling services.
// Its deadline is http.outbound_timeout_seconds, read on every call, so a change
// applies to the next request. transport may be nil.
func OutboundClient(live *Live, transport http.RoundTripper) *httpx.Client {
	return httpx.New(func() time.Duration {
		return live.Current().Seconds(HTTPOutboundTimeoutSeconds)
	}, transport)
}
