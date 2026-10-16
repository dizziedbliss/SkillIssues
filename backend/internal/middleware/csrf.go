package middleware

import (
	"fmt"
	"mime"
	"net/http"
	"strings"
)

// CSRF protects the cookie-authenticated API from cross-site requests.
//
// Two layers apply to every state-changing request (anything but GET, HEAD
// and OPTIONS):
//
//  1. http.CrossOriginProtection refuses a browser request whose
//     Sec-Fetch-Site or Origin shows it came from another site, unless that
//     origin is in frontendURL. Requests with neither header (curl, the
//     desktop app) pass: they are not browsers and cannot be made to send
//     the user's cookie.
//  2. A request that carries the session cookie must declare
//     Content-Type: application/json. A cross-site <form> can only send
//     form or text encodings, and a fetch() with application/json forces a
//     CORS preflight that CORS refuses for an unlisted origin. Requests
//     authenticated by a bearer token are exempt from both: a browser will
//     not attach an Authorization header on a page's behalf.
//
// frontendURL may be a comma-separated list, as for CORS. A malformed entry
// is an error, not silently dropped: a trusted origin that does not parse
// would otherwise break sign-in with no hint why.
func CSRF(frontendURL string, sessionCookie string) (func(http.Handler) http.Handler, error) {
	cop := http.NewCrossOriginProtection()
	for _, origin := range strings.Split(frontendURL, ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if err := cop.AddTrustedOrigin(origin); err != nil {
			return nil, fmt.Errorf("csrf: trusted origin %q: %w", origin, err)
		}
	}
	cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondError(w, http.StatusForbidden, "cross-origin request refused")
	}))

	return func(next http.Handler) http.Handler {
		checked := cop.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !safeMethod(r.Method) && usesSessionCookie(r, sessionCookie) && !isJSON(r) {
				respondError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
				return
			}
			next.ServeHTTP(w, r)
		}))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if hasBearer(r) {
				next.ServeHTTP(w, r)
				return
			}
			checked.ServeHTTP(w, r)
		})
	}, nil
}

func safeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

func hasBearer(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
}

func usesSessionCookie(r *http.Request, name string) bool {
	_, err := r.Cookie(name)
	return err == nil
}

func isJSON(r *http.Request) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mt == "application/json"
}
