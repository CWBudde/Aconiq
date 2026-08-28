package httpv1

import (
	"net/http"
	"net/url"
	"strings"
)

// corsMiddleware returns middleware that adds CORS headers for local-only use.
//
// Design: local API serves only localhost/127.0.0.1 callers. By default it
// allows any localhost/127.0.0.1 origin (any port) so the Vite dev server on
// :5173 and a production bundle on :8080 both work without configuration.
// Callers can supply additional explicit origins via allowedOrigins.
//
// This is intentionally simple: no credentials, no wildcard-domain matching,
// localhost-only. What it is *not* is a CSRF control. A browser does not block
// a cross-site request to a loopback address; it blocks the attacker from
// *reading the response*, and it does not even do that for a simple request,
// which needs no preflight and so never consults this allowlist. And an origin
// allowlist is blind to DNS rebinding, where the request genuinely originates
// from the attacker's page but arrives labelled same-origin. The controls that
// close both shapes live in security.go — the Host allowlist, the media-type
// requirement and the custom header this middleware advertises below.
func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && isAllowedOrigin(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				// The custom header must be advertised, or the browser fails the
				// preflight that requiring it is meant to force.
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization, "+ClientHeaderName)
				w.Header().Set("Access-Control-Max-Age", "86400")
				// Vary: Origin tells caches that the response differs by origin.
				w.Header().Add("Vary", "Origin")

				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isAllowedOrigin reports whether origin should receive CORS headers.
// Localhost and 127.0.0.1 are always permitted (any port).
// Additional origins can be passed explicitly or "*" to allow all.
func isAllowedOrigin(origin string, extra []string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" {
		return true
	}

	for _, allowed := range extra {
		if allowed == "*" {
			return true
		}

		// Exact match (scheme + host + port).
		if strings.EqualFold(allowed, origin) {
			return true
		}
	}

	return false
}
