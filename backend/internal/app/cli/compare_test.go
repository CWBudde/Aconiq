package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func TestCompareSoundPlanReceivers(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	soundPlanDir := soundPlanInteropPath(t)

	mustRunCLI(t, "--project", projectDir, "init", "--name", "CompareSoundPLAN", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--from-soundplan", soundPlanDir)
	mustRunCLI(t, "--project", projectDir, "compare")

	reportPath := filepath.Join(projectDir, ".noise", "artifacts", "soundplan-receiver-compare.json")
	payload, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read compare report: %v", err)
	}

	var report soundPlanCompareReport
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("decode compare report: %v", err)
	}

	if report.Command != "compare" {
		t.Fatalf("command = %q, want compare", report.Command)
	}

	if report.RunID == "" {
		t.Fatal("expected run id in compare report")
	}

	if report.MatchedReceiverCount == 0 {
		t.Fatal("expected at least one matched receiver")
	}

	if report.MatchedReceiverCount+report.UnmatchedAconiqCount != 77 {
		t.Fatalf("matched + unmatched Aconiq receivers = %d, want 77", report.MatchedReceiverCount+report.UnmatchedAconiqCount)
	}

	if report.Stats[schall03.IndicatorLrDay].Count != report.MatchedReceiverCount {
		t.Fatalf("day stats count = %d, want %d", report.Stats[schall03.IndicatorLrDay].Count, report.MatchedReceiverCount)
	}

	if report.Raster == nil {
		t.Fatal("expected raster metadata section in compare report")
	}

	if report.Raster.Status != "metadata_only" {
		t.Fatalf("raster status = %q, want metadata_only", report.Raster.Status)
	}

	if len(report.Raster.SoundPlanRuns) != 4 {
		t.Fatalf("raster run count = %d, want 4", len(report.Raster.SoundPlanRuns))
	}

	manifestPayload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "project.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var proj project.Project
	if err := json.Unmarshal(manifestPayload, &proj); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}

	if _, ok := latestRun(proj.Runs); !ok {
		t.Fatal("expected compare to create a run")
	}
}
