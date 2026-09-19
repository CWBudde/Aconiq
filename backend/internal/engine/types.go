package engine

import (
	"time"

	"github.com/aconiq/backend/internal/geo"
)

// Source is a minimal source model used by the engine skeleton.
type Source struct {
	ID       string
	Point    geo.Point2D
	Emission float64
}

// Receiver aliases the geo receiver model for engine input.
type Receiver = geo.PointReceiver

// RunConfig configures one compute execution.
type RunConfig struct {
	RunID            string
	Workers          int
	ChunkSize        int
	CacheDir         string
	RunCacheKeepLast int
	Receivers        []Receiver
	Sources          []Source
	DisableCache     bool
	ComputeDelay     time.Duration
	SourceIndexCellM float64
	DeterminismTag   string

	// StandardKey names the resolved standard whose kernel produced the
	// levels, and is part of the shared chunk cache key.
	//
	// It is separate from DeterminismTag, which looks made for the job and is
	// not: `aconiq bench` sets that tag to "bench-cold" and "bench-warm"
	// precisely so those two runs SHARE a cache entry, and keying on it would
	// make the warm run cold. The tag describes the run; this describes the
	// calculation.
	//
	// The engine hard-codes dummy/freefield today, so every real run passes
	// the same value and nothing can collide. That stops being true when the
	// kernel is parameterised, which is why the key carries it already.
	StandardKey StandardKey

	// OnReceiverComputed, when non-nil, is invoked once per receiver right
	// after its level has been evaluated, with the receiver ID.
	//
	// It exists so tests can drive cancellation (and similar observations)
	// off a deterministic point inside the compute loop instead of racing a
	// wall-clock sleep against the workers. It is called from every worker
	// goroutine, so an implementation must be safe for concurrent use, and it
	// must not influence the computed values - the determinism policy in
	// docs/policies/determinism.md applies unchanged.
	OnReceiverComputed func(receiverID string)
}

// StandardKey identifies the resolved standard a set of levels came from.
//
// Three fields rather than one string because that is what resolution
// produces - framework.ResolvedProfile carries StandardID, Version and Profile
// - and because a joined string would need an escaping rule the moment an id
// contained the separator.
type StandardKey struct {
	StandardID string `json:"standard_id"`
	Version    string `json:"version"`
	Profile    string `json:"profile"`
}

// ReceiverResult stores one computed indicator value.
type ReceiverResult struct {
	ReceiverID string  `json:"receiver_id"`
	LevelDB    float64 `json:"level_db"`
}

// RunOutput is the persisted and returned output of one run.
type RunOutput struct {
	RunID            string           `json:"run_id"`
	Status           string           `json:"status"`
	StartedAt        time.Time        `json:"started_at"`
	FinishedAt       time.Time        `json:"finished_at"`
	Results          []ReceiverResult `json:"results"`
	OutputHash       string           `json:"output_hash"`
	TotalChunks      int              `json:"total_chunks"`
	UsedCachedChunks int              `json:"used_cached_chunks"`
	Metadata         map[string]any   `json:"metadata,omitempty"`
}

// RunState stores resumable/inspectable run state on disk.
type RunState struct {
	RunID           string    `json:"run_id"`
	Status          string    `json:"status"`
	UpdatedAt       time.Time `json:"updated_at"`
	TotalChunks     int       `json:"total_chunks"`
	CompletedChunks int       `json:"completed_chunks"`
	Message         string    `json:"message,omitempty"`
}

const (
	RunStateRunning   = "running"
	RunStateCompleted = "completed"
	RunStateCanceled  = "canceled"
	RunStateFailed    = "failed"
)

// ProgressEvent is emitted during staged engine execution.
type ProgressEvent struct {
	Time            time.Time `json:"time"`
	RunID           string    `json:"run_id"`
	Stage           string    `json:"stage"`
	Message         string    `json:"message,omitempty"`
	ChunkIndex      int       `json:"chunk_index,omitempty"`
	CompletedChunks int       `json:"completed_chunks,omitempty"`
	TotalChunks     int       `json:"total_chunks,omitempty"`
}

// ProgressSink receives structured events.
type ProgressSink func(event ProgressEvent)
