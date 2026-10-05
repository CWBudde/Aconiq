package httpv1

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/aconiq/backend/internal/io/lglnimport"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	apiVersion = "v1"
	// evidenceTierField is the JSON member the evidence tier travels under, in
	// responses, in error details and in the OpenAPI document alike.
	evidenceTierField = "evidence_tier"
	// messageRunIDRequired is shared by every run-scoped endpoint, so that the
	// three of them answer an empty {id} in one voice rather than three.
	messageRunIDRequired = "run id is required"
)

type Handler struct {
	store       projectfs.Store
	manifest    *sync.Mutex
	now         func() time.Time
	sseInterval time.Duration
	registry    *framework.Registry
	runExecutor runExecutor
	lgln        *lglnimport.Client
	// logger carries the few things this package has to say that no response
	// can carry: a cleanup that failed after the request had already been
	// answered, and a run it declined to attribute. Never nil — the
	// constructor falls back to slog.Default().
	logger *slog.Logger
}

type runExecutor func(context.Context, createRunRequest) error

func NewHandler(store projectfs.Store, clock func() time.Time) http.Handler {
	return newHandlerWithOptions(store, handlerOptions{
		clock:       clock,
		sseInterval: 2 * time.Second,
	})
}

// NewHandlerWithRegistry returns a handler that also exposes /api/v1/standards.
// CORS is enabled by default (localhost/127.0.0.1 allowed).
func NewHandlerWithRegistry(store projectfs.Store, clock func() time.Time, registry framework.Registry) http.Handler {
	return newHandlerWithOptions(store, handlerOptions{
		clock:       clock,
		sseInterval: 2 * time.Second,
		registry:    &registry,
	})
}

// ServeOptions configures the handler `aconiq serve` mounts.
type ServeOptions struct {
	// CORSOrigins holds extra allowed origins beyond localhost/127.0.0.1
	// (nil is fine for local use).
	CORSOrigins []string
	// ListenAddr is the address the server was told to bind. Its host part joins
	// the Host allowlist, so a server bound to a named or LAN address stays
	// reachable under that name while DNS rebinding does not.
	ListenAddr string
	// APIToken, when non-empty, must be presented as a bearer token on every
	// request. Empty means the transport controls stand alone.
	APIToken string
	// Logger receives what the handler cannot put in a response — a failed
	// cleanup, most of all. Nil falls back to slog.Default().
	Logger *slog.Logger
}

// NewServeHandler builds a handler suitable for `aconiq serve` with CORS enabled.
func NewServeHandler(store projectfs.Store, clock func() time.Time, registry framework.Registry, opts ServeOptions) http.Handler {
	return newHandlerWithOptions(store, handlerOptions{
		clock:        clock,
		sseInterval:  2 * time.Second,
		registry:     &registry,
		corsOrigins:  opts.CORSOrigins,
		allowedHosts: hostsFromListenAddr(opts.ListenAddr),
		apiToken:     opts.APIToken,
		logger:       opts.Logger,
	})
}

type handlerOptions struct {
	clock        func() time.Time
	sseInterval  time.Duration
	registry     *framework.Registry
	corsOrigins  []string // extra allowed origins beyond localhost/127.0.0.1
	corsDisabled bool     // set true for same-origin deployments (Wails etc.)
	allowedHosts []string // extra Host header values beyond loopback
	apiToken     string   // optional bearer token; empty disables the check
	runExecutor  runExecutor
	lgln         *lglnimport.Client // nil falls back to lglnimport.NewClient()
	logger       *slog.Logger       // nil falls back to slog.Default()
}

// lockManifest serialises one read-modify-write of `.noise/project.json`
// against the other handlers that do the same, and returns the release.
//
// Three routes Load the manifest, mutate the value and Save it back with
// nothing in between - POST /model, POST /import/terrain and
// DELETE /runs/{id} - so without this the second Save discards the first
// one's change. POST /runs is not among them: it Loads, execs `aconiq run`
// and Loads again, never saving in process.
//
// The field is a pointer, and has to be. mux.HandleFunc takes a method value,
// which copies the Handler once per route, so a sync.Mutex value here would
// become fourteen independent mutexes that lock nothing against each other.
//
// This covers the in-process half only. The `aconiq run` child writes the same
// file from another process, which no mutex can reach; what protects the
// manifest's integrity there is projectfs.writeFileAtomic's unique temp name.
func (h Handler) lockManifest() func() {
	h.manifest.Lock()

	return h.manifest.Unlock
}

func newHandlerWithOptions(store projectfs.Store, opts handlerOptions) http.Handler {
	now := opts.clock
	if opts.clock == nil {
		now = time.Now
	}

	sseInterval := opts.sseInterval
	if sseInterval <= 0 {
		sseInterval = 2 * time.Second
	}

	logger := opts.logger
	if logger == nil {
		logger = slog.Default()
	}

	handler := Handler{
		store:       store,
		now:         now,
		sseInterval: sseInterval,
		registry:    opts.registry,
		runExecutor: opts.runExecutor,
		manifest:    &sync.Mutex{},
		lgln:        opts.lgln,
		logger:      logger,
	}
	if handler.lgln == nil {
		handler.lgln = lglnimport.NewClient()
	}

	if handler.runExecutor == nil {
		handler.runExecutor = newCLIProcessRunExecutor(store.Root())
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", handler.handleHealth)
	mux.HandleFunc("/api/v1/project/status", handler.handleProjectStatus)
	mux.HandleFunc("/api/v1/standards", handler.handleStandards)
	mux.HandleFunc("/api/v1/runs", handler.handleRuns)
	mux.HandleFunc("/api/v1/runs/{id}", handler.handleRun)
	mux.HandleFunc("/api/v1/runs/{id}/log", handler.handleRunLog)
	mux.HandleFunc("/api/v1/runs/{id}/contours", handler.handleRunContours)
	mux.HandleFunc("/api/v1/artifacts/{id}/content", handler.handleArtifactContent)
	mux.HandleFunc("/api/v1/events", handler.handleEvents)
	mux.HandleFunc("/api/v1/openapi.json", handler.handleOpenAPI)
	mux.HandleFunc("/api/v1/import/osm", handler.handleImportOSM)
	mux.HandleFunc("/api/v1/import/lgln", handler.handleImportLGLN)
	mux.HandleFunc("/api/v1/import/terrain", handler.handleImportTerrain)
	mux.HandleFunc("/api/v1/model", handler.handleModel)
	mux.HandleFunc("/api/v1/transform", handler.handleTransform)
	mux.HandleFunc("/", handler.handleNotFound)

	// The security middleware sits inside CORS so that a refusal still carries
	// the CORS headers a browser needs in order to read the envelope, and so
	// that a preflight is answered before the state-changing-method controls
	// (which a preflight, being an OPTIONS, cannot satisfy).
	guarded := securityMiddleware(securityOptions{
		allowedHosts: opts.allowedHosts,
		apiToken:     opts.apiToken,
	})(mux)

	if opts.corsDisabled {
		return guarded
	}

	return corsMiddleware(opts.corsOrigins)(guarded)
}

func (h Handler) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeAPIError(w, http.StatusNotFound, apiError{
		Code:    errorCodeNotFound,
		Message: "endpoint not found",
		Details: map[string]any{
			"method": r.Method,
			"path":   r.URL.Path,
		},
		Hint: "Use /api/v1/health, /api/v1/project/status, /api/v1/runs, /api/v1/runs/{id}, /api/v1/runs/{id}/log, /api/v1/artifacts/{id}/content, /api/v1/standards, /api/v1/events, /api/v1/openapi.json, /api/v1/import/osm, /api/v1/import/lgln, /api/v1/import/terrain, or /api/v1/model.",
	})
}

func (h Handler) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	writeJSON(w, http.StatusOK, BuildOpenAPISpec(""))
}
