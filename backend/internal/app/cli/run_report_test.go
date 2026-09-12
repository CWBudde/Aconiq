package cli

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/app/config"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/spf13/cobra"
)

// The completion event is how an operator ties a log line back to the results
// on disk. Determinism is only auditable if the output hash travels with every
// record of a run, so it is pinned here rather than left to survive the next
// refactor by luck.
func TestRunCompletionEventCarriesTheOutputHash(t *testing.T) {
	t.Parallel()

	const outputHash = "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"

	var logs bytes.Buffer

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	state := commandState{
		Config: config.Config{},
		Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
	}

	prepared := preparedRun{
		store:      store,
		run:        project.Run{ID: "run-1", ScenarioID: "scenario-1", Standard: project.StandardRef{ID: "cnossos-road"}},
		provenance: project.ProvenanceManifest{ManifestPath: "provenance.json"},
		runDir:     store.Root(),
	}

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})

	err = reportRunCompletion(cmd, state, prepared, runModuleResult{outputHash: outputHash})
	if err != nil {
		t.Fatalf("report run completion: %v", err)
	}

	var event map[string]any

	line := strings.TrimSpace(logs.String())
	if line == "" {
		t.Fatal("no completion event was logged")
	}

	err = json.Unmarshal([]byte(line), &event)
	if err != nil {
		t.Fatalf("decode completion event %q: %v", line, err)
	}

	if event["msg"] != "run completed" {
		t.Fatalf("unexpected completion event: %v", event)
	}

	if event["output_hash"] != outputHash {
		t.Fatalf("completion event must carry the output hash, got %v", event["output_hash"])
	}
}
