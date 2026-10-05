package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/report/reporting"
)

// evidenceTierFromProvenance reads the evidence tier the run recorded in its own
// provenance metadata, under the same key the generated report reads, so the
// bundle summary can never disagree with the report inside the bundle. The
// bundle summary must state the tier the run actually
// ran at, so it is read from provenance rather than resolved against the current
// registry: a standard that has since been retired, renamed or re-tiered would
// otherwise be reported under a tier the run never used. Every failure path —
// no provenance copied into the bundle, an unreadable or malformed file, or
// provenance predating the disclosure — yields the unknown marker rather than an
// error, because a missing tier must not stop an export.
func evidenceTierFromProvenance(path string) string {
	if strings.TrimSpace(path) == "" {
		return reporting.UnknownEvidenceTier
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		return reporting.UnknownEvidenceTier
	}

	var parsed project.ProvenanceManifest

	err = json.Unmarshal(payload, &parsed)
	if err != nil {
		return reporting.UnknownEvidenceTier
	}

	tier := strings.TrimSpace(parsed.Metadata[reporting.ProvenanceEvidenceTierKey])
	if tier == "" {
		return reporting.UnknownEvidenceTier
	}

	return tier
}

// computeCRSFromProvenance returns the CRS a run's results are expressed in.
//
// The empty string means one thing only: a manifest that decoded and carries
// no compute_crs, which is a run recorded before the key existed and whose
// results are therefore in the project CRS. newFormatExportContext applies
// that fallback.
//
// A manifest that cannot be read or decoded is an error rather than that same
// empty string. Conflating the two would label a geographic run's UTM results
// EPSG:4326 — putting every exported result off the coast of Africa — on the
// strength of an unreadable file, which is the one case where guessing is
// least defensible.
//
// A run with no provenance path at all is not an error: `aconiq export`
// stages one only when the run has one.
func computeCRSFromProvenance(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read provenance %s: %w", path, err)
	}

	var parsed project.ProvenanceManifest

	err = json.Unmarshal(payload, &parsed)
	if err != nil {
		return "", fmt.Errorf("decode provenance %s: %w", path, err)
	}

	return strings.TrimSpace(parsed.Metadata[provenanceComputeCRSKey]), nil
}
