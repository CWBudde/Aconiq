package project

import "time"

// ProvenanceManifest stores reproducibility-critical metadata for one run.
type ProvenanceManifest struct {
	RunID         string            `json:"run_id"`
	ScenarioID    string            `json:"scenario_id"`
	Standard      StandardRef       `json:"standard"`
	ReceiverMode  string            `json:"receiver_mode,omitempty"`
	ReceiverSetID string            `json:"receiver_set_id,omitempty"`
	Parameters    map[string]string `json:"parameters,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	InputHashes   map[string]string `json:"input_hashes"`
	StandardData  *StandardDataRef  `json:"standard_data,omitempty"`
	GeneratedAt   time.Time         `json:"generated_at"`
	ToolName      string            `json:"tool_name"`
	ToolVersion   string            `json:"tool_version"`
	ManifestPath  string            `json:"-"`
}

// StandardDataRef records which coefficient data the standards module carried
// when a run was produced, so that identical provenance implies identical
// numbers.
//
// It is deliberately not an entry in InputHashes: that map is defined as
// input-file path to SHA-256, and every entry in it is rendered as an "Input
// files" row in generated reports. A coefficient table embedded in the binary
// is not a file the user supplied, and claiming it is would misreport the run.
type StandardDataRef struct {
	// Algorithm names the hash function, currently always "sha256".
	Algorithm string `json:"algorithm"`
	// Digest covers the standard ID, the evidence tier and every table below.
	Digest string `json:"digest"`
	// EvidenceTier is repeated here because it is an input to Digest; a reader
	// must be able to check the digest without consulting the metadata map.
	EvidenceTier string `json:"evidence_tier"`
	// Tables lists the individual coefficient sources, sorted by name, so a
	// digest mismatch can be attributed to one table rather than to the module.
	Tables []StandardDataTableRef `json:"tables,omitempty"`
}

// IsZero reports whether no standard data was recorded, which is the case for a
// module that carries no coefficient tables at all.
func (r StandardDataRef) IsZero() bool {
	return r.Digest == ""
}

// StandardDataTableRef is the digest of one named coefficient table.
type StandardDataTableRef struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}
