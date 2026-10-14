package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/runestimate"
)

func runCLIOutput(t *testing.T, args ...string) string {
	t.Helper()

	var out bytes.Buffer

	cmd := newRootCommand()
	cmd.SetArgs(args)
	cmd.SetOut(&out)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("aconiq %v: %v", args, err)
	}

	return out.String()
}

// projectSnapshot is everything an estimate must leave alone: the manifest,
// byte for byte, and the list of run directories.
func projectSnapshot(t *testing.T, projectDir string) (string, []string) {
	t.Helper()

	manifest, err := os.ReadFile(filepath.Join(projectDir, ".noise", "project.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(projectDir, ".noise", "runs"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read runs directory: %v", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	return string(manifest), names
}

func TestRunEstimateCreatesNoRun(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase17", "rls19_road_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Estimate", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

	manifestBefore, runsBefore := projectSnapshot(t, projectDir)

	out := runCLIOutput(t, "--project", projectDir, "--json", "run", "--standard", "rls19-road",
		"--estimate", "--estimate-budget", "100ms", "--workers", "2")

	var payload struct {
		Mode     string               `json:"mode"`
		Standard string               `json:"standard"`
		Estimate runestimate.Estimate `json:"estimate"`
	}

	err := json.Unmarshal([]byte(out), &payload)
	if err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}

	estimate := payload.Estimate
	if payload.Mode != "estimate" || !estimate.Available || estimate.StandardID != "rls19-road" {
		t.Fatalf("unexpected payload: %s", out)
	}

	if estimate.Inputs.Receivers == 0 || estimate.Inputs.Workers != 2 || estimate.Inputs.Units == 0 ||
		estimate.Probe.MeasuredReceivers == 0 || estimate.WallSeconds.High <= 0 {
		t.Fatalf("estimate is missing its measurements: %s", out)
	}

	manifestAfter, runsAfter := projectSnapshot(t, projectDir)
	if manifestAfter != manifestBefore || strings.Join(runsAfter, ",") != strings.Join(runsBefore, ",") {
		t.Fatalf("--estimate changed the project: runs %v -> %v", runsBefore, runsAfter)
	}

	// The human form says the same, and says that nothing was run.
	text := runCLIOutput(t, "--project", projectDir, "run", "--standard", "rls19-road",
		"--estimate", "--estimate-budget", "50ms")
	if !strings.Contains(text, "Estimate for rls19-road: ") || !strings.Contains(text, "No run was created.") {
		t.Fatalf("text output:\n%s", text)
	}

	if manifestAfter, _ = projectSnapshot(t, projectDir); manifestAfter != manifestBefore {
		t.Fatal("--estimate changed the manifest")
	}
}

// ISO 9613-2 and Schall 03 answered "no estimate available" until they had a
// kernel probe; both chains of Schall 03 have one, and the fixture below runs
// the normative one, which is what its engine resolves to on track data. They
// walk their receivers on one goroutine, so the estimate prices one worker
// whatever --workers says.
func TestRunEstimateCoversISO9613AndSchall03(t *testing.T) {
	t.Parallel()

	for _, standard := range []string{"iso9613", "schall03"} {
		t.Run(standard, func(t *testing.T) {
			t.Parallel()

			projectDir := t.TempDir()

			mustRunCLI(t, "--project", projectDir, "init", "--name", "Estimate", "--crs", "EPSG:25832")
			mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, registryRunFixtures[standard]...))

			manifestBefore, runsBefore := projectSnapshot(t, projectDir)

			out := runCLIOutput(t, "--project", projectDir, "--json", "run", "--standard", standard,
				"--estimate", "--estimate-budget", "100ms", "--workers", "4")

			var payload struct {
				Estimate runestimate.Estimate `json:"estimate"`
			}

			err := json.Unmarshal([]byte(out), &payload)
			if err != nil {
				t.Fatalf("decode %q: %v", out, err)
			}

			estimate := payload.Estimate
			if !estimate.Available || estimate.StandardID != standard || estimate.WallSeconds == nil {
				t.Fatalf("expected an estimate for %s: %s", standard, out)
			}

			if estimate.Inputs.Receivers == 0 || estimate.Inputs.Workers != 1 || estimate.Inputs.Units == 0 ||
				estimate.Probe.MeasuredReceivers == 0 || estimate.WallSeconds.High <= 0 {
				t.Fatalf("estimate is missing its measurements: %s", out)
			}

			manifestAfter, runsAfter := projectSnapshot(t, projectDir)
			if manifestAfter != manifestBefore || strings.Join(runsAfter, ",") != strings.Join(runsBefore, ",") {
				t.Fatalf("--estimate changed the project: runs %v -> %v", runsBefore, runsAfter)
			}
		})
	}
}

func TestRunEstimateWithoutAProbeSaysSo(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, registryRunFixtures["cnossos-road"]...)

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Estimate", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

	manifestBefore, _ := projectSnapshot(t, projectDir)

	out := runCLIOutput(t, "--project", projectDir, "--json", "run", "--standard", "cnossos-road", "--experimental", "--estimate")

	var payload struct {
		Estimate runestimate.Estimate `json:"estimate"`
	}

	err := json.Unmarshal([]byte(out), &payload)
	if err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}

	if payload.Estimate.Available || !strings.Contains(payload.Estimate.Reason, "no estimate available") ||
		payload.Estimate.WallSeconds != nil {
		t.Fatalf("a standard without a probe must say so: %s", out)
	}

	if manifestAfter, _ := projectSnapshot(t, projectDir); manifestAfter != manifestBefore {
		t.Fatal("--estimate changed the manifest")
	}
}

func TestRunEstimateRefusesWhatTheRunRefuses(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase17", "rls19_road_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Estimate", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

	runErr := runCLI("--project", projectDir, "run", "--standard", "rls19-road", "--param", "segment_length_mode=bogus")
	estimateErr := runCLI("--project", projectDir, "run", "--standard", "rls19-road", "--param", "segment_length_mode=bogus", "--estimate")

	if runErr == nil || estimateErr == nil {
		t.Fatalf("both should refuse: run %v, estimate %v", runErr, estimateErr)
	}

	stripArgs := func(err error) string {
		_, message, _ := strings.Cut(err.Error(), "]: ")

		return message
	}

	if stripArgs(runErr) != stripArgs(estimateErr) {
		t.Fatalf("run refused with %q, estimate with %q", stripArgs(runErr), stripArgs(estimateErr))
	}
}
