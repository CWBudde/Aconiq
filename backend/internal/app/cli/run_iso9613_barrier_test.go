package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/report/results"
	"github.com/aconiq/backend/internal/standards/iso9613"
)

// ISO 9613-2's screening formulas were complete and unit-tested long before a
// run could reach them: PropagationConfig.Barrier was a pre-computed geometry
// nothing set, no descriptor parameter could express one, and the generic
// receiver run module handed compute no model to find a barrier in. Every run
// added 0 dB for A_bar and said nothing about it.
//
// This test is the end-to-end half of closing that. It runs the same model
// twice — once with a `kind: barrier` feature standing between the sources and
// the southern receivers, once without — and reads the levels back off the
// persisted receiver table. Nothing here constructs a BarrierGeometry, so the
// only way the levels can differ is the extraction and the per-path derivation
// both working.
func TestISO9613RunScreensBehindAnImportedBarrier(t *testing.T) {
	t.Parallel()

	params := []string{
		"--param", "grid_resolution_m=20",
		"--param", "grid_padding_m=40",
	}

	open := iso9613ReceiverLevels(t, registryRunFixtures["iso9613"], params)
	screened := iso9613ReceiverLevels(t, []string{"phase19", "iso9613_screened_model.geojson"}, params)

	if len(open) == 0 {
		t.Fatal("the unscreened run produced no receivers")
	}

	if len(open) != len(screened) {
		t.Fatalf("the two runs produced %d and %d receivers; the barrier must not change the grid", len(open), len(screened))
	}

	var (
		lowered int
		best    float64
	)

	for id, openLevel := range open {
		screenedLevel, ok := screened[id]
		if !ok {
			t.Fatalf("receiver %q is missing from the screened run", id)
		}

		if screenedLevel > openLevel+1e-9 {
			t.Fatalf("receiver %q got louder behind a barrier: %.6f dB vs %.6f dB", id, screenedLevel, openLevel)
		}

		if drop := openLevel - screenedLevel; drop > 1e-9 {
			lowered++

			if drop > best {
				best = drop
			}
		}
	}

	if lowered == 0 {
		t.Fatal("a 9 m barrier across the model lowered no receiver at all, so A_bar is still identically zero")
	}

	// A barrier that moves the level by a rounding error is the same silent
	// failure with a different number on it.
	if best < 1 {
		t.Fatalf("the best-screened receiver gained only %.6f dB, which is not screening", best)
	}

	// Some receiver must be unaffected too, or the scene is not discriminating
	// between screened and open paths — it is applying something globally.
	if lowered == len(open) {
		t.Fatalf("all %d receivers were screened; a barrier along one side must leave the far side alone", len(open))
	}
}

// iso9613ReceiverLevels runs one model through the CLI and returns the
// persisted level per receiver id.
func iso9613ReceiverLevels(t *testing.T, fixture []string, params []string) map[string]float64 {
	t.Helper()

	projectDir := t.TempDir()

	mustRunCLI(t, "--project", projectDir, "init", "--name", "ISO9613Barrier", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, fixture...))

	args := append([]string{"--project", projectDir, "run", "--standard", "iso9613"}, params...)
	mustRunCLI(t, args...)

	runID := digestRunRecord(t, projectDir, "iso9613")

	payload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "runs", runID, "results", "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable

	err = json.Unmarshal(payload, &table)
	if err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	levels := make(map[string]float64, len(table.Records))

	for _, record := range table.Records {
		value, ok := record.Values[iso9613.IndicatorLpAeqDW]
		if !ok {
			t.Fatalf("receiver %q carries no %s", record.ID, iso9613.IndicatorLpAeqDW)
		}

		// A silent receiver would make the comparison vacuous.
		if value <= 0 {
			t.Fatalf("receiver %q %s = %v; the scene produced no level to compare", record.ID, iso9613.IndicatorLpAeqDW, value)
		}

		levels[record.ID] = value
	}

	return levels
}
