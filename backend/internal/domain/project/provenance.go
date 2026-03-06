package project

import "time"

// ProvenanceManifest stores reproducibility-critical metadata for one run.
type ProvenanceManifest struct {
	RunID        string            `json:"run_id"`
	ScenarioID   string            `json:"scenario_id"`
	Standard     StandardRef       `json:"standard"`
	Parameters   map[string]string `json:"parameters,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	InputHashes  map[string]string `json:"input_hashes"`
	GeneratedAt  time.Time         `json:"generated_at"`
	ToolName     string            `json:"tool_name"`
	ToolVersion  string            `json:"tool_version"`
	ManifestPath string            `json:"-"`
}
