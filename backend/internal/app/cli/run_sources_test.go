package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// roadSource is a road along y with the review_status given, "" for none: a
// source taken from plans or an import.
func roadSource(id string, y string, reviewStatus string) string {
	review := ""
	if reviewStatus != "" {
		review = `,"review_status":"` + reviewStatus + `","evidence":[{"kind":"orthophoto","licence":"CC-BY-4.0"}]`
	}

	return `
    {"type":"Feature","properties":{"id":"` + id + `","kind":"source","source_type":"line"` + review + `},
     "geometry":{"type":"LineString","coordinates":[[-100,` + y + `],[200,` + y + `]]}}`
}

const plainReceiver = `
    {"type":"Feature","properties":{"id":"r1","kind":"receiver","height_m":4},
     "geometry":{"type":"Point","coordinates":[5,30]}}`

func runSummary(t *testing.T, resultsDir string) map[string]any {
	t.Helper()

	payload, err := os.ReadFile(filepath.Join(resultsDir, "run-summary.json"))
	if err != nil {
		t.Fatalf("read run summary: %v", err)
	}

	var summary map[string]any

	err = json.Unmarshal(payload, &summary)
	if err != nil {
		t.Fatalf("decode run summary: %v", err)
	}

	return summary
}

// A normative run over a source someone traced from imagery and nobody
// confirmed is refused before the run exists, and so is its estimate; the
// opt-in lifts both and records the count. A rejected source is not computed.
func TestProposedSourcesFollowTheGate(t *testing.T) {
	t.Parallel()

	projectDir := facadeProject(t, facadeModel(
		roadSource("traced", "-20", "proposed"), roadSource("gone", "-40", "rejected"), plainReceiver,
	))
	args := facadeRunArgs(projectDir, "rls19-road", "--receiver-mode", receiverModeCustom)

	requireErrorContaining(t, runCLI(args...), `"rls19-road" is normative`, "1 proposed source(s)", "--allow-unconfirmed")
	requireErrorContaining(t, runCLI(append(args, "--estimate", "--estimate-budget", "50ms")...), "1 proposed source(s)")

	if n := projectRunCount(t, projectDir); n != 0 {
		t.Fatalf("a refused run left %d runs", n)
	}

	mustRunCLI(t, append(args, "--allow-unconfirmed")...)

	metadata, resultsDir := facadeRunOutputs(t, projectDir)
	if metadata[sourceProposedCountKey] != "1" || metadata[sourceRejectedCountKey] != "1" || metadata[allowUnconfirmedKey] != "true" {
		t.Fatalf("provenance metadata = %v", metadata)
	}

	summary := runSummary(t, resultsDir)
	if summary["source_count"] != 1.0 || summary[sourceProposedCountKey] != 1.0 || summary[sourceRejectedCountKey] != 1.0 {
		t.Fatalf("run summary = %v", summary)
	}
}

// Confirmed and unreviewed sources need no opt-in; a run over them records
// nothing new, and a rejected one is still left out.
func TestConfirmedSourcesRunWithoutTheOptIn(t *testing.T) {
	t.Parallel()

	projectDir := facadeProject(t, facadeModel(
		roadSource("checked", "-20", "confirmed"), roadSource("planned", "-30", ""),
		roadSource("gone", "-40", "rejected"), plainReceiver,
	))

	mustRunCLI(t, facadeRunArgs(projectDir, "rls19-road", "--receiver-mode", receiverModeCustom)...)

	metadata, resultsDir := facadeRunOutputs(t, projectDir)
	if metadata[sourceProposedCountKey] != "" || metadata[sourceRejectedCountKey] != "1" || metadata[allowUnconfirmedKey] != "" {
		t.Fatalf("provenance metadata = %v", metadata)
	}

	if summary := runSummary(t, resultsDir); summary["source_count"] != 2.0 {
		t.Fatalf("run summary = %v, want the two kept sources counted", summary)
	}
}
