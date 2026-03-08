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
