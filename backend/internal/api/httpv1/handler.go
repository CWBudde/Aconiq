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
	// estimateExecutor prices a run request without starting it.
	estimateExecutor estimateExecutor
	// estimates holds the one estimate in flight. A pointer for the reason
	// manifest is one.
	estimates *estimateSlot
	// runs is every run this server has started and not yet settled. A
	// pointer for the reason manifest is one.
	runs *runSupervisor
	// runStartTimeout bounds how long POST /runs waits for a run's row.
	runStartTimeout time.Duration
	lgln            *lglnimport.Client
	// logger carries the few things this package has to say that no response
	// can carry: a cleanup that failed after the request had already been
	// answered, and a run it declined to attribute. Never nil — the
	// constructor falls back to slog.Default().
	logger *slog.Logger
}

// runExecutor computes one run, recorded under runID, and returns once the
// process computing it has exited. It runs under a context the server owns:
// when that context ends the run is interrupted, not abandoned.
type runExecutor func(ctx context.Context, runID string, req createRunRequest) error

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

// ServeHandler is the handler `aconiq serve` mounts, together with the runs it
// starts. Those runs outlive any request, so the server has to stop them
// itself on its way out: StopRuns.
type ServeHandler struct {
	http.Handler

	runs *runSupervisor
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
func NewServeHandler(store projectfs.Store, clock func() time.Time, registry framework.Registry, opts ServeOptions) *ServeHandler {
	runs := newRunSupervisor()

	return &ServeHandler{
		Handler: newHandlerWithOptions(store, handlerOptions{
			clock:        clock,
			sseInterval:  2 * time.Second,
			registry:     &registry,
			corsOrigins:  opts.CORSOrigins,
			allowedHosts: hostsFromListenAddr(opts.ListenAddr),
			apiToken:     opts.APIToken,
			logger:       opts.Logger,
			runs:         runs,
		}),
		runs: runs,
	}
}

// StopRuns interrupts every run this handler started and waits, until ctx
// ends, for each to settle — record itself, or be killed after its grace and
// closed by the server. New runs are refused from the moment it is called.
// Call it after http.Server.Shutdown, so no request is left waiting on a run
// that will never be started.
func (s *ServeHandler) StopRuns(ctx context.Context) error {
	return s.runs.shutdown(ctx)
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
	estimate     estimateExecutor   // nil falls back to the CLI child process
	runs         *runSupervisor     // nil gets a supervisor of its own
	runStartWait time.Duration      // zero falls back to runStartTimeout
	lgln         *lglnimport.Client // nil falls back to lglnimport.NewClient()
	logger       *slog.Logger       // nil falls back to slog.Default()
}

// lockManifest serialises one read-modify-write of `.noise/project.json`
// against the other handlers that do the same, and returns the release.
//
// Three routes Load the manifest, mutate the value and Save it back with
// nothing in between - POST /model, POST /import/terrain and
// DELETE /runs/{id} - so without this the second Save discards the first
// one's change. The fourth writer is settleRun, which closes a run whose
// child died without closing it. POST /runs itself never saves in process:
// it starts `aconiq run` and reads the manifest until the child's row is in
// it.
//
// The field is a pointer, and has to be. mux.HandleFunc takes a method value,
// which copies the Handler once per route, so a sync.Mutex value here would
// become fourteen independent mutexes that lock nothing against each other.
//
// This is the in-process half. The `aconiq run` children write the same file
// from other processes, which no mutex reaches: that half is
// projectfs.Store.LockManifest, a file lock. The routes that load and save
// the manifest themselves take it inside this mutex; DeleteRun and
// FailOpenRun take it on their own, so it is never held around them.
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
		store:            store,
		now:              now,
		sseInterval:      sseInterval,
		registry:         opts.registry,
		runExecutor:      opts.runExecutor,
		estimateExecutor: opts.estimate,
		estimates:        &estimateSlot{},
		runs:             opts.runs,
		runStartTimeout:  opts.runStartWait,
		manifest:         &sync.Mutex{},
		lgln:             opts.lgln,
		logger:           logger,
	}
	if handler.lgln == nil {
		handler.lgln = lglnimport.NewClient()
	}

	if handler.runs == nil {
		handler.runs = newRunSupervisor()
	}

	if handler.runStartTimeout <= 0 {
		handler.runStartTimeout = runStartTimeout
	}

	useChildProcessExecutors(&handler, store.Root())

	mux := handler.routes()

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

// routes is the route table: every path the API serves, and the 404 envelope
// for the rest. The OpenAPI document is hand-built, so a route added here needs
// its openapi_<area>.go section too.
func (h Handler) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", h.handleHealth)
	mux.HandleFunc("/api/v1/project/status", h.handleProjectStatus)
	mux.HandleFunc("/api/v1/standards", h.handleStandards)
	mux.HandleFunc("/api/v1/runs", h.handleRuns)
	mux.HandleFunc("/api/v1/runs/estimate", h.handleRunEstimate)
	mux.HandleFunc("/api/v1/runs/{id}", h.handleRun)
	mux.HandleFunc("/api/v1/runs/{id}/log", h.handleRunLog)
	mux.HandleFunc("/api/v1/runs/{id}/provenance", h.handleRunProvenance)
	mux.HandleFunc("/api/v1/runs/{id}/cancel", h.handleRunCancel)
	mux.HandleFunc("/api/v1/runs/{id}/contours", h.handleRunContours)
	mux.HandleFunc("/api/v1/artifacts/{id}/content", h.handleArtifactContent)
	mux.HandleFunc("/api/v1/events", h.handleEvents)
	mux.HandleFunc("/api/v1/openapi.json", h.handleOpenAPI)
	mux.HandleFunc("/api/v1/import/osm", h.handleImportOSM)
	mux.HandleFunc("/api/v1/import/lgln", h.handleImportLGLN)
	mux.HandleFunc("/api/v1/import/terrain", h.handleImportTerrain)
	mux.HandleFunc("/api/v1/model", h.handleModel)
	mux.HandleFunc("/api/v1/transform", h.handleTransform)
	mux.HandleFunc("/api/v1/facades/elevation", h.handleFacadeElevation)
	mux.HandleFunc("/api/v1/facades/window", h.handleFacadeWindow)
	mux.HandleFunc("/api/v1/facades/rectify", h.handleFacadeRectify)
	mux.HandleFunc("/", h.handleNotFound)

	return mux
}

func (h Handler) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeAPIError(w, http.StatusNotFound, apiError{
		Code:    errorCodeNotFound,
		Message: "endpoint not found",
		Details: map[string]any{
			"method": r.Method,
			"path":   r.URL.Path,
		},
		Hint: "Use /api/v1/health, /api/v1/project/status, /api/v1/runs, /api/v1/runs/{id}, /api/v1/runs/{id}/log, /api/v1/runs/{id}/provenance, /api/v1/runs/{id}/cancel, /api/v1/artifacts/{id}/content, /api/v1/standards, /api/v1/events, /api/v1/openapi.json, /api/v1/import/osm, /api/v1/import/lgln, /api/v1/import/terrain, or /api/v1/model.",
	})
}

func (h Handler) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	writeJSON(w, http.StatusOK, BuildOpenAPISpec(""))
}
