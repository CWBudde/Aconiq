package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
	"github.com/aconiq/backend/internal/standards/framework"
	"github.com/aconiq/backend/internal/standards/iso9613"
)

// evidenceTierRunCase drives one standard end to end so that the tier can be
// checked in every place a run discloses it.
type evidenceTierRunCase struct {
	name       string
	standardID string
	modelParts []string
	wantTier   framework.EvidenceTier
}

// runArgs appends whatever the case's tier demands of a run — the scaffold
// opt-in, today — to the arguments the caller assembled.
func (c evidenceTierRunCase) runArgs(args ...string) []string {
	return append(args, tierRunArgs[c.wantTier]...)
}

func evidenceTierRunCases() []evidenceTierRunCase {
	return []evidenceTierRunCase{
		{
			name:       "normative",
			standardID: iso9613.StandardID,
			modelParts: []string{"phase19", "iso9613_industry_model.geojson"},
			wantTier:   framework.EvidenceTierNormative,
		},
		{
			name:       "scaffold",
			standardID: cnossosroad.StandardID,
			modelParts: []string{"phase10", "road_model.geojson"},
			wantTier:   framework.EvidenceTierScaffold,
		},
	}
}

// A run must not be able to hide how far its numbers may be trusted: the tier
// has to reach the operator's terminal, the run log, the provenance manifest
// and the run summary alike.
func TestRunDisclosesEvidenceTier(t *testing.T) {
	t.Parallel()

	for _, testCase := range evidenceTierRunCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			projectDir := t.TempDir()
			modelPath := testdataPath(t, testCase.modelParts...)

			mustRunCLI(t, "--project", projectDir, "init", "--name", "Tier", "--crs", "EPSG:25832")
			mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

			var buf bytes.Buffer

			cmd := newRootCommand()
			cmd.SetOut(&buf)
			cmd.SetArgs(testCase.runArgs("--project", projectDir, "run", "--standard", testCase.standardID))

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("run %s: %v", testCase.standardID, err)
			}

			wantBanner := "Evidence tier: " + testCase.wantTier.Headline()
			if !strings.Contains(buf.String(), wantBanner) {
				t.Fatalf("run output carries no evidence tier banner %q:\n%s", wantBanner, buf.String())
			}

			run := latestProjectRun(t, projectDir)

			logPayload, readErr := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.LogPath)))
			if readErr != nil {
				t.Fatalf("read run.log: %v", readErr)
			}

			wantLogLine := "evidence_tier=" + string(testCase.wantTier)
			if !strings.Contains(string(logPayload), wantLogLine) {
				t.Fatalf("run.log carries no %q line:\n%s", wantLogLine, logPayload)
			}

			provenance := readRunProvenance(t, projectDir, run)
			if got := provenance.Metadata["evidence_tier"]; got != string(testCase.wantTier) {
				t.Fatalf("provenance evidence_tier = %q, want %q", got, testCase.wantTier)
			}

			summary := readRunSummary(t, filepath.Join(projectDir, ".noise", "runs", run.ID, "results", "run-summary.json"))
			if got := summary["evidence_tier"]; got != string(testCase.wantTier) {
				t.Fatalf("run summary evidence_tier = %v, want %q", got, testCase.wantTier)
			}
		})
	}
}

// In JSON mode the banner would corrupt the payload, so the tier travels inside
// it instead.
func TestRunJSONOutputReportsEvidenceTier(t *testing.T) {
	t.Parallel()

	for _, testCase := range evidenceTierRunCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			projectDir := t.TempDir()
			modelPath := testdataPath(t, testCase.modelParts...)

			mustRunCLI(t, "--project", projectDir, "init", "--name", "TierJSON", "--crs", "EPSG:25832")
			mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

			var buf bytes.Buffer

			cmd := newRootCommand()
			cmd.SetOut(&buf)
			cmd.SetArgs(testCase.runArgs("--json", "--project", projectDir, "run", "--standard", testCase.standardID))

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("run %s: %v", testCase.standardID, err)
			}

			var payload map[string]any

			err = json.Unmarshal(buf.Bytes(), &payload)
			if err != nil {
				t.Fatalf("decode run output: %v\nraw: %s", err, buf.String())
			}

			if got := payload["evidence_tier"]; got != string(testCase.wantTier) {
				t.Fatalf("evidence_tier = %v, want %q", got, testCase.wantTier)
			}
		})
	}
}

func TestStatusReportsEvidenceTier(t *testing.T) {
	t.Parallel()

	for _, testCase := range evidenceTierRunCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			projectDir := t.TempDir()
			modelPath := testdataPath(t, testCase.modelParts...)

			mustRunCLI(t, "--project", projectDir, "init", "--name", "TierStatus", "--crs", "EPSG:25832")
			mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
			mustRunCLI(t, testCase.runArgs("--project", projectDir, "run", "--standard", testCase.standardID)...)

			entries := statusRunEntriesJSON(t, projectDir)
			if len(entries) != 1 {
				t.Fatalf("expected one status run entry, got %d", len(entries))
			}

			if entries[0].StandardEvidenceTier != string(testCase.wantTier) {
				t.Fatalf("standard_evidence_tier = %q, want %q", entries[0].StandardEvidenceTier, testCase.wantTier)
			}

			var human bytes.Buffer

			cmd := newRootCommand()
			cmd.SetOut(&human)
			cmd.SetArgs([]string{"--project", projectDir, "status"})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("status: %v", err)
			}

			if !strings.Contains(human.String(), "tier="+string(testCase.wantTier)) {
				t.Fatalf("status table carries no tier column for %s:\n%s", testCase.wantTier, human.String())
			}
		})
	}
}

// A project may outlive the registration of the standard one of its runs used.
// Status must still render, with no tier claimed for that run.
func TestStatusReportsUnknownEvidenceTierForUnregisteredStandard(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase19", "iso9613_industry_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "TierGone", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", iso9613.StandardID)

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	proj.Runs[len(proj.Runs)-1].Standard.ID = "retired-standard"

	err = store.Save(proj)
	if err != nil {
		t.Fatalf("save project: %v", err)
	}

	entries := statusRunEntriesJSON(t, projectDir)
	if len(entries) != 1 {
		t.Fatalf("expected one status run entry, got %d", len(entries))
	}

	if entries[0].StandardEvidenceTier != "" {
		t.Fatalf("standard_evidence_tier = %q, want empty for an unregistered standard", entries[0].StandardEvidenceTier)
	}

	var human bytes.Buffer

	cmd := newRootCommand()
	cmd.SetOut(&human)
	cmd.SetArgs([]string{"--project", projectDir, "status"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if !strings.Contains(human.String(), "tier=unknown") {
		t.Fatalf("status table does not mark the unregistered standard as unknown:\n%s", human.String())
	}
}

func statusRunEntriesJSON(t *testing.T, projectDir string) []statusRunEntry {
	t.Helper()

	var buf bytes.Buffer

	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--json", "--project", projectDir, "status"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("status --json: %v", err)
	}

	var payload struct {
		Runs []statusRunEntry `json:"runs"`
	}

	err = json.Unmarshal(buf.Bytes(), &payload)
	if err != nil {
		t.Fatalf("decode status output: %v\nraw: %s", err, buf.String())
	}

	return payload.Runs
}

func latestProjectRun(t *testing.T, projectDir string) project.Run {
	t.Helper()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) == 0 {
		t.Fatal("expected at least one run")
	}

	return proj.Runs[len(proj.Runs)-1]
}

func readRunProvenance(t *testing.T, projectDir string, run project.Run) project.ProvenanceManifest {
	t.Helper()

	payload, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	var provenance project.ProvenanceManifest

	err = json.Unmarshal(payload, &provenance)
	if err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	return provenance
}

// A scaffold-tier standard emits levels that look like every other run's, so it
// may not be reachable by accident. The refusal has to be a user-input error —
// the CLI's exit code 2 depends on it — and it has to leave the project
// untouched: the gate sits ahead of the run record, not after it.
func TestRunRefusesScaffoldStandardWithoutExperimentalOptIn(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase10", "road_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "TierGate", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

	err := runCLI("--project", projectDir, "run", "--standard", cnossosroad.StandardID)
	if err == nil {
		t.Fatal("expected the run to be refused without --experimental")
	}

	var appErr *domainerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error is not a domain error: %v", err)
	}

	if appErr.Kind != domainerrors.KindUserInput {
		t.Fatalf("error kind = %q, want %q", appErr.Kind, domainerrors.KindUserInput)
	}

	for _, want := range []string{cnossosroad.StandardID, string(framework.EvidenceTierScaffold), "no normative coefficients", "--experimental"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal does not mention %q: %v", want, err)
		}
	}

	assertNoRunPersisted(t, projectDir)
}

// The gate must not fire before the parameters are checked: a request that is
// wrong in both ways reports the parameter defect first, so that the tier
// disclosure is the last thing standing between the operator and the run they
// actually get.
func TestRunReportsParameterErrorBeforeExperimentalOptIn(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase10", "road_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "TierGateParams", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

	err := runCLI("--project", projectDir, "run", "--standard", cnossosroad.StandardID, "--param", "not_allowed=1")
	if err == nil {
		t.Fatal("expected the run to fail on the unknown parameter")
	}

	if !strings.Contains(err.Error(), "not_allowed") {
		t.Fatalf("expected the unknown parameter to be reported first: %v", err)
	}

	assertNoRunPersisted(t, projectDir)
}

// The acknowledgement is all that stands between the operator and the run; with
// it given, a scaffold standard runs to completion like any other.
func TestRunAcceptsScaffoldStandardWithExperimentalOptIn(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase10", "road_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "TierGateOptIn", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", cnossosroad.StandardID, "--experimental")

	run := latestProjectRun(t, projectDir)
	if run.Status != project.RunStatusCompleted {
		t.Fatalf("run status = %q, want %q", run.Status, project.RunStatusCompleted)
	}

	if run.Standard.ID != cnossosroad.StandardID {
		t.Fatalf("run standard = %q, want %q", run.Standard.ID, cnossosroad.StandardID)
	}
}

// Only the scaffold tier is gated. Gating any other tier would be a false
// disclosure — and gating the test fixture would break `aconiq run` with no
// --standard at all, since that is the default.
func TestRunNeedsNoOptInOutsideTheScaffoldTier(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		standardID string
		modelParts []string
		wantTier   framework.EvidenceTier
	}{
		{
			name:       "normative",
			standardID: iso9613.StandardID,
			modelParts: []string{"phase19", "iso9613_industry_model.geojson"},
			wantTier:   framework.EvidenceTierNormative,
		},
		{
			name:       "preview",
			standardID: "beb-exposure",
			modelParts: []string{"phase16", "beb_model.geojson"},
			wantTier:   framework.EvidenceTierPreview,
		},
		{
			name:       "test fixture",
			standardID: freefield.StandardID,
			modelParts: []string{"phase8", "model.geojson"},
			wantTier:   framework.EvidenceTierTestFixture,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if testCase.wantTier.RequiresExperimentalOptIn() {
				t.Fatalf("tier %q must not require the experimental opt-in", testCase.wantTier)
			}

			projectDir := t.TempDir()
			modelPath := testdataPath(t, testCase.modelParts...)

			mustRunCLI(t, "--project", projectDir, "init", "--name", "TierUngated", "--crs", "EPSG:25832")
			mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
			mustRunCLI(t, "--project", projectDir, "run", "--standard", testCase.standardID)

			run := latestProjectRun(t, projectDir)
			if run.Status != project.RunStatusCompleted {
				t.Fatalf("run status = %q, want %q", run.Status, project.RunStatusCompleted)
			}
		})
	}
}

// `aconiq run` with no --standard must keep working: its default is the test
// fixture, which the gate does not cover.
func TestRunDefaultStandardStaysUngated(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase8", "model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "TierDefault", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run")

	run := latestProjectRun(t, projectDir)
	if run.Standard.ID != freefield.StandardID {
		t.Fatalf("default run standard = %q, want %q", run.Standard.ID, freefield.StandardID)
	}

	if run.Status != project.RunStatusCompleted {
		t.Fatalf("run status = %q, want %q", run.Status, project.RunStatusCompleted)
	}
}

// A refused run must leave neither a manifest entry nor a run directory behind.
func assertNoRunPersisted(t *testing.T, projectDir string) {
	t.Helper()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) != 0 {
		t.Fatalf("expected no run in the manifest, got %d", len(proj.Runs))
	}

	entries, err := os.ReadDir(filepath.Join(projectDir, ".noise", "runs"))
	if err != nil {
		t.Fatalf("read runs dir: %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("expected no run directory, got %d", len(entries))
	}
}
