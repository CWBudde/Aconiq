package project

import "time"

const (
	// CurrentManifestVersion is the current project manifest schema version.
	CurrentManifestVersion = 1

	// StorageKindJSONV1 stores metadata in JSON files in the project folder.
	StorageKindJSONV1 = "json-v1"
)

const (
	RunStatusPending   = "pending"
	RunStatusRunning   = "running"
	RunStatusCompleted = "completed"
	RunStatusFailed    = "failed"
)

// Project is the persisted v1 project manifest.
type Project struct {
	ManifestVersion int            `json:"manifest_version"`
	ProjectID       string         `json:"project_id"`
	Name            string         `json:"name"`
	CRS             string         `json:"crs"`
	Storage         StorageConfig  `json:"storage"`
	Scenarios       []Scenario     `json:"scenarios"`
	Runs            []Run          `json:"runs"`
	Artifacts       []ArtifactRef  `json:"artifacts"`
	Migrations      MigrationState `json:"migrations"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// StorageConfig captures how project metadata/results are persisted.
type StorageConfig struct {
	Kind  string `json:"kind"`
	Notes string `json:"notes,omitempty"`
}

// Scenario defines a configurable calculation scenario in the project.
type Scenario struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Standard   StandardRef    `json:"standard"`
	Parameters map[string]any `json:"parameters,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// Run captures one execution instance for a scenario.
type Run struct {
	ID             string      `json:"id"`
	ScenarioID     string      `json:"scenario_id"`
	Standard       StandardRef `json:"standard"`
	ReceiverMode   string      `json:"receiver_mode,omitempty"`
	ReceiverSetID  string      `json:"receiver_set_id,omitempty"`
	Status         string      `json:"status"`
	LogPath        string      `json:"log_path"`
	ProvenancePath string      `json:"provenance_path"`
	StartedAt      time.Time   `json:"started_at"`
	FinishedAt     time.Time   `json:"finished_at"`
}

// StandardRef identifies a standard implementation version/profile.
type StandardRef struct {
	Context string `json:"context,omitempty"`
	ID      string `json:"id"`
	Version string `json:"version"`
	Profile string `json:"profile,omitempty"`
}

// ArtifactRef points to a generated artifact in the project.
type ArtifactRef struct {
	ID        string    `json:"id"`
	RunID     string    `json:"run_id,omitempty"`
	Kind      string    `json:"kind"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

// ArtifactRef.Kind values. Import and run commands write these; `aconiq export`
// dispatches on them, so producer and consumer must agree on the spelling.
const (
	ArtifactKindModelNormalizedGeoJSON = "model.normalized_geojson"
	ArtifactKindModelDumpJSON          = "model.dump_json"
	ArtifactKindModelValidationReport  = "model.validation_report"

	ArtifactKindRunResultReceiverTableJSON = "run.result.receiver_table_json"
	ArtifactKindRunResultReceiverTableCSV  = "run.result.receiver_table_csv"
	ArtifactKindRunResultRasterMetadata    = "run.result.raster_metadata"
	ArtifactKindRunResultRasterBinary      = "run.result.raster_binary"
	ArtifactKindRunResultSummary           = "run.result.summary"
)

// ArtifactKindRunResultPrefix is the common prefix of every run-result artifact
// kind; `aconiq export` uses it to select the artifacts of a single run.
const ArtifactKindRunResultPrefix = "run.result."

// ArtifactRef.ID values for the model artifacts written by `aconiq import` and
// `aconiq validate`. They are stable and singleton per project, so importers
// overwrite rather than append.
const (
	ArtifactIDModelNormalized = "artifact-model-normalized"
	ArtifactIDModelDump       = "artifact-model-dump"
	ArtifactIDModelValidation = "artifact-model-validation"
)

// MigrationState tracks current and historical schema transitions.
type MigrationState struct {
	CurrentVersion         int               `json:"current_version"`
	LatestSupportedVersion int               `json:"latest_supported_version"`
	History                []MigrationRecord `json:"history,omitempty"`
}

// MigrationRecord captures one schema migration operation.
type MigrationRecord struct {
	FromVersion int       `json:"from_version"`
	ToVersion   int       `json:"to_version"`
	AppliedAt   time.Time `json:"applied_at"`
	ToolVersion string    `json:"tool_version,omitempty"`
}
