package httpv1

import (
	"crypto/subtle"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Transport-level controls for the local API.
//
// The API is unauthenticated by default and a browser can reach it, so three
// controls stand between a hostile page and the run executor, which shells out
// to `aconiq run` with request-controlled argv:
//
//  1. A Host allowlist. Only loopback names and the address the server was told
//     to listen on are accepted. This is what closes DNS rebinding: an attacker
//     who points their own name at 127.0.0.1 still sends that name in the Host
//     header, and an Origin check cannot see it because the browser considers
//     the request same-origin.
//  2. A media-type requirement on every endpoint that reads a body. text/plain,
//     application/x-www-form-urlencoded and multipart/form-data are the three
//     media types a cross-origin fetch may send without a preflight, so an
//     endpoint that decodes JSON out of any of them is reachable by a CORS
//     simple request.
//  3. A custom header on every state-changing method. A simple request cannot
//     carry one, so requiring it forces a preflight, and the preflight is
//     answered only for an allowed origin. The header's value is irrelevant —
//     its presence is the proof that a preflight happened.
//
// A bearer token sits on top of these and is deliberately opt-in
// (`aconiq serve --api-token`). The three controls above close the browser
// vectors on their own; a mandatory token answers a different threat model —
// another process on the same machine — and would cost every local integration
// a credential exchange it does not need today.
const (
	// ClientHeaderName is the custom header every state-changing request must
	// carry. Its value is not checked. Exported because it is part of the HTTP
	// contract every client has to satisfy, including `aconiq serve`'s own help.
	ClientHeaderName = "X-Aconiq-Client"

	authorizationHeaderName = "Authorization"
	bearerScheme            = "Bearer"

	mediaTypeJSON      = "application/json"
	mediaTypeMultipart = "multipart/form-data"
)

// Request body caps. Each endpoint that reads a body names its own, and the
// middleware applies maxRequestBodyBytes to everything as a backstop for paths
// that never reach a handler.
const (
	maxRequestBodyBytes   = 64 << 20 // 64 MB
	maxRunCreateBodyBytes = 256 << 10
	maxImportOSMBodyBytes = 64 << 10
	maxTerrainUploadBytes = 50 << 20 // 50 MB, the whole multipart body
	maxTerrainMemoryBytes = 8 << 20  // buffered in memory; the rest would spill to a temp file
)

// allowedOverpassHosts is the set of Overpass API servers the HTTP API may be
// pointed at. `overpass_endpoint` arrives in a request body, so without this the
// endpoint is a general-purpose HTTP client for whoever can reach the API: an
// internal address, a cloud metadata service, or a file-shaped URL. The CLI
// import path is not affected — it takes its endpoint from an operator's own
// command line, which is not an untrusted surface.
var allowedOverpassHosts = []string{
	"lz4.overpass-api.de",
	"maps.mail.ru",
	"overpass-api.de",
	"overpass.kumi.systems",
	"overpass.openstreetmap.fr",
	"overpass.openstreetmap.ru",
	"overpass.osm.ch",
	"z.overpass-api.de",
}

// errPathOutsideProject marks a manifest-declared path that does not stay inside
// the project root.
var errPathOutsideProject = stderrors.New("path escapes the project root")

type securityOptions struct {
	// allowedHosts holds Host header values accepted in addition to loopback,
	// normally just the host part of the configured listen address.
	allowedHosts []string
	// apiToken, when non-empty, must be presented as a bearer token on every
	// request.
	apiToken string
}

// securityMiddleware applies the transport-level controls described above,
// before routing, so an endpoint cannot forget one.
func securityMiddleware(opts securityOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !checkHost(w, r, opts.allowedHosts) {
				return
			}

			if !checkAPIToken(w, r, opts.apiToken) {
				return
			}

			if !checkClientHeader(w, r) {
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

			next.ServeHTTP(w, r)
		})
	}
}

func checkHost(w http.ResponseWriter, r *http.Request, allowed []string) bool {
	if isAllowedHost(r.Host, allowed) {
		return true
	}

	writeAPIError(w, http.StatusForbidden, apiError{
		Code:    errorCodeForbiddenHost,
		Message: "request Host header is not allowed",
		Details: map[string]any{
			"host": r.Host,
		},
		Hint: "Address the local API as 127.0.0.1 or localhost, or as the host passed to `aconiq serve --listen`.",
	})

	return false
}

// isAllowedHost reports whether a Host header may reach the router. Loopback
// names and addresses are always accepted; anything else must be listed.
func isAllowedHost(hostHeader string, allowed []string) bool {
	host := hostHeader

	if parsed, _, err := net.SplitHostPort(hostHeader); err == nil {
		host = parsed
	}

	host = strings.Trim(host, "[]")
	if host == "" {
		return false
	}

	if strings.EqualFold(host, "localhost") {
		return true
	}

	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}

	return slices.ContainsFunc(allowed, func(candidate string) bool {
		return strings.EqualFold(candidate, host)
	})
}

// hostsFromListenAddr derives the extra Host allowlist entry from a listen
// address. A wildcard bind names no host, so it contributes nothing and only
// loopback stays reachable.
func hostsFromListenAddr(addr string) []string {
	if addr == "" {
		return nil
	}

	host := addr

	if parsed, _, err := net.SplitHostPort(addr); err == nil {
		host = parsed
	}

	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" {
		return nil
	}

	return []string{host}
}

func checkAPIToken(w http.ResponseWriter, r *http.Request, token string) bool {
	if token == "" {
		return true
	}

	presented, ok := bearerToken(r.Header.Get(authorizationHeaderName))
	if ok && subtle.ConstantTimeCompare([]byte(presented), []byte(token)) == 1 {
		return true
	}

	w.Header().Set("WWW-Authenticate", bearerScheme)
	writeAPIError(w, http.StatusUnauthorized, apiError{
		Code:    errorCodeUnauthorized,
		Message: "missing or invalid API token",
		Hint:    "Send the token `aconiq serve --api-token` was started with as `Authorization: Bearer <token>`.",
	})

	return false
}

func bearerToken(header string) (string, bool) {
	scheme, value, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, bearerScheme) {
		return "", false
	}

	return strings.TrimSpace(value), true
}

// checkClientHeader requires the custom header on every method that can change
// state. Safe methods are exempt: OPTIONS carries the preflight itself, and a
// GET that mutated nothing would gain nothing from the requirement.
func checkClientHeader(w http.ResponseWriter, r *http.Request) bool {
	if isSafeMethod(r.Method) || r.Header.Get(ClientHeaderName) != "" {
		return true
	}

	writeAPIError(w, http.StatusForbidden, apiError{
		Code:    errorCodeClientHeaderRequired,
		Message: "state-changing requests must carry the " + ClientHeaderName + " header",
		Details: map[string]any{
			"header": ClientHeaderName,
		},
		Hint: "Set " + ClientHeaderName + " to any non-empty value. It forces a CORS preflight, which is what makes a cross-site request visible to the origin allowlist.",
	})

	return false
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// requireContentType rejects a body whose media type is not the one this
// endpoint parses. A charset (or, for multipart, a boundary) parameter is fine;
// the media type itself must match.
func requireContentType(w http.ResponseWriter, r *http.Request, want string) bool {
	raw := r.Header.Get("Content-Type")

	got, _, err := mime.ParseMediaType(raw)
	if err == nil && strings.EqualFold(got, want) {
		return true
	}

	writeAPIError(w, http.StatusUnsupportedMediaType, apiError{
		Code:    errorCodeUnsupportedMediaType,
		Message: "request body must be sent as " + want,
		Details: map[string]any{
			"expected": want,
			"received": raw,
		},
	})

	return false
}

// decodeJSONBody enforces the endpoint's media type and body cap and then
// decodes into dst. It reports whether the caller may continue; every failure
// path has already written an envelope.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, limit int64, dst any) bool {
	if !requireContentType(w, r, mediaTypeJSON) {
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, limit)

	err := json.NewDecoder(r.Body).Decode(dst)
	if err != nil {
		if writeRequestTooLarge(w, err, limit) {
			return false
		}

		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "request body must be valid JSON",
		})

		return false
	}

	return true
}

// writeTooLarge answers a request that ran past this endpoint's cap.
func writeTooLarge(w http.ResponseWriter, limit int64) {
	writeAPIError(w, http.StatusRequestEntityTooLarge, apiError{
		Code:    errorCodeRequestTooLarge,
		Message: "request body exceeds the limit for this endpoint",
		Details: map[string]any{
			"limit_bytes": limit,
		},
	})
}

// writeRequestTooLarge answers err if it is a body-cap overrun, and reports
// whether it did, so callers fall through to their own error for anything else.
func writeRequestTooLarge(w http.ResponseWriter, err error, limit int64) bool {
	var tooLarge *http.MaxBytesError
	if !stderrors.As(err, &tooLarge) {
		return false
	}

	writeTooLarge(w, limit)

	return true
}

// validateOverpassEndpoint constrains the Overpass server a request may point
// the importer at. Empty means "use the built-in default" and is left alone.
func validateOverpassEndpoint(endpoint string) *apiError {
	if endpoint == "" {
		return nil
	}

	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return &apiError{
			Code:    errorCodeOverpassEndpointNotAllowed,
			Message: "overpass_endpoint must be an absolute https URL",
			Details: map[string]any{
				"allowed_hosts": allowedOverpassHosts,
			},
		}
	}

	host := parsed.Hostname()
	if isAllowedOverpassHost(host) {
		return nil
	}

	return &apiError{
		Code:    errorCodeOverpassEndpointNotAllowed,
		Message: fmt.Sprintf("overpass_endpoint host %q is not an allowed Overpass server", host),
		Details: map[string]any{
			"allowed_hosts": allowedOverpassHosts,
		},
		Hint: "Omit overpass_endpoint to use the default server, or run the import from the CLI, where the endpoint comes from your own command line.",
	}
}

func isAllowedOverpassHost(host string) bool {
	return slices.ContainsFunc(allowedOverpassHosts, func(candidate string) bool {
		return strings.EqualFold(candidate, host)
	})
}

// readProjectFile reads a manifest-declared path from inside the project root.
//
// The path comes out of .noise/project.json, which travels with a project a
// user may have received from someone else, so it is untrusted input: a
// manifest whose log_path is "../../../../etc/passwd" must not become an HTTP
// read of that file. filepath.IsLocal rejects absolute paths, ".." escapes and
// the empty string before the open, and os.OpenInRoot re-checks every element
// against the root as it walks it, so a symlink planted inside the project
// cannot escape either.
func readProjectFile(root, relPath string) ([]byte, error) {
	rel := filepath.FromSlash(strings.ReplaceAll(relPath, `\`, "/"))
	if !filepath.IsLocal(rel) {
		return nil, fmt.Errorf("%w: %s", errPathOutsideProject, relPath)
	}

	file, err := os.OpenInRoot(root, rel)
	if err != nil {
		return nil, fmt.Errorf("open %s inside project root: %w", rel, err)
	}

	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}

	return data, nil
}

// writeProjectFileError distinguishes a manifest that points outside the
// project — a containment failure the operator has to look at — from an
// ordinary read failure.
func writeProjectFileError(w http.ResponseWriter, err error, message string) {
	if stderrors.Is(err, errPathOutsideProject) {
		writeAPIError(w, http.StatusForbidden, apiError{
			Code:    errorCodeForbiddenPath,
			Message: "project manifest refers to a path outside the project root",
			Hint:    "Inspect .noise/project.json: recorded paths must be relative to the project directory.",
		})

		return
	}

	writeAPIError(w, http.StatusInternalServerError, apiError{
		Code:    errorCodeInternalError,
		Message: message,
	})
}
