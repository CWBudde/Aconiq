# CLI `--json` Structured Output Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extend the existing `--json` root flag so that all CLI commands emit structured JSON on stdout (in addition to JSON logs on stderr), making the CLI fully machine-drivable by LLMs and automation tools.

**Architecture:** Add a `cmdoutput` helper package that provides a `Write(cmd, state, payload)` function. When `state.Config.JSONLogs` is true, it JSON-encodes the payload to stdout. When false, each command prints its existing human-readable text. Each command builds a typed result struct and calls `cmdoutput.Write` instead of (or in addition to) its `fmt.Fprintf` calls.

**Tech Stack:** Go stdlib (`encoding/json`), Cobra (`cmd.OutOrStdout()`), existing `commandState` / `config.Config`.

---

### Task 1: Add `cmdoutput` helper

**Files:**

- Create: `backend/internal/app/cli/cmdoutput.go`
- Test: `backend/internal/app/cli/cmdoutput_test.go`

**Step 1: Write the failing test**

```go
// cmdoutput_test.go
package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	type testPayload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	var buf bytes.Buffer
	err := writeCommandOutput(&buf, true, testPayload{Name: "test", Count: 42})
	if err != nil {
		t.Fatalf("writeCommandOutput: %v", err)
	}

	var got testPayload
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != "test" || got.Count != 42 {
		t.Fatalf("got %+v, want {test 42}", got)
	}
}

func TestWriteJSONSkipsWhenDisabled(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := writeCommandOutput(&buf, false, map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("writeCommandOutput: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no output when JSON disabled, got %q", buf.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/app/cli/... -run TestWriteJSON -v`
Expected: FAIL — `writeCommandOutput` not defined

**Step 3: Write minimal implementation**

```go
// cmdoutput.go
package cli

import (
	"encoding/json"
	"io"
)

// writeCommandOutput writes the payload as a single JSON object to w when
// jsonEnabled is true. When false it is a no-op — the caller is expected to
// fall through to the existing fmt.Fprintf calls.
func writeCommandOutput(w io.Writer, jsonEnabled bool, payload any) error {
	if !jsonEnabled {
		return nil
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/app/cli/... -run TestWriteJSON -v`
Expected: PASS

**Step 5: Commit**

```
feat(cli): add writeCommandOutput helper for JSON stdout
```

---

### Task 2: JSON output for `init`

**Files:**

- Modify: `backend/internal/app/cli/init.go`
- Test: `backend/internal/app/cli/cmdoutput_test.go` (add integration test)

**Step 1: Write the failing test**

```go
func TestInitJSONOutput(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	var buf bytes.Buffer

	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--json", "--project", projectDir, "init", "--name", "TestJSON"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v\nraw: %s", err, buf.String())
	}

	if result["command"] != "init" {
		t.Fatalf("command = %v, want init", result["command"])
	}
	if result["project_name"] != "TestJSON" {
		t.Fatalf("project_name = %v, want TestJSON", result["project_name"])
	}
	if result["project_id"] == nil || result["project_id"] == "" {
		t.Fatal("expected non-empty project_id")
	}
	if result["manifest_path"] == nil || result["manifest_path"] == "" {
		t.Fatal("expected non-empty manifest_path")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/app/cli/... -run TestInitJSON -v`
Expected: FAIL — output is human text, not JSON

**Step 3: Implement JSON output in init.go**

In the `RunE` function of `newInitCommand`, after the existing `store.Init(...)` call, add:

```go
if state.Config.JSONLogs {
	err := writeCommandOutput(cmd.OutOrStdout(), true, map[string]any{
		"command":       "init",
		"project_id":    proj.ProjectID,
		"project_name":  proj.Name,
		"project_path":  store.Root(),
		"manifest_path": store.ManifestPath(),
		"crs":           proj.CRS,
	})
	return err
}

// Existing human-readable output below...
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/app/cli/... -run TestInitJSON -v`
Expected: PASS

**Step 5: Commit**

```
feat(cli): emit structured JSON from init when --json is set
```

---

### Task 3: JSON output for `validate`

**Files:**

- Modify: `backend/internal/app/cli/validate.go`

**Step 1: Write the failing test**

```go
func TestValidateJSONOutput(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase8", "model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "ValJSON", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

	var buf bytes.Buffer
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--json", "--project", projectDir, "validate"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	if result["command"] != "validate" {
		t.Fatalf("command = %v, want validate", result["command"])
	}
	if result["feature_count"] == nil {
		t.Fatal("expected feature_count")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/app/cli/... -run TestValidateJSON -v`
Expected: FAIL

**Step 3: Implement JSON output in validate.go**

Before the existing `fmt.Fprintf` calls, add the JSON early-return:

```go
if state.Config.JSONLogs {
	payload := map[string]any{
		"command":       "validate",
		"input":         relInput,
		"feature_count": len(model.Features),
		"errors":        report.ErrorCount(),
		"warnings":      report.WarningCount(),
	}
	if writeReport {
		payload["report_path"] = relativePath(store.Root(), reportPath)
	}
	writeErr := writeCommandOutput(cmd.OutOrStdout(), true, payload)
	if writeErr != nil {
		return writeErr
	}
	// Still return validation error if errors > 0
	if report.ErrorCount() > 0 {
		messages := make([]string, 0, len(report.Errors))
		for _, issue := range report.Errors {
			messages = append(messages, fmt.Sprintf("%s: %s", issue.Code, issue.Message))
		}
		return domainerrors.New(domainerrors.KindValidation, "cli.validate", summarizeValidationErrors(messages, 5), nil)
	}
	return nil
}
```

**Step 4: Run test**

Run: `cd backend && go test ./internal/app/cli/... -run TestValidateJSON -v`
Expected: PASS

**Step 5: Commit**

```
feat(cli): emit structured JSON from validate when --json is set
```

---

### Task 4: JSON output for `import`

**Files:**

- Modify: `backend/internal/app/cli/import.go`

This is the most complex command — multiple import modes produce different outputs. The JSON payload should be a union of all fields; absent fields are omitted.

**Step 1: Write the failing test**

```go
func TestImportJSONOutput(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase8", "model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "ImpJSON", "--crs", "EPSG:25832")

	var buf bytes.Buffer
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--json", "--project", projectDir, "import", "--input", modelPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("import: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	if result["command"] != "import" {
		t.Fatalf("command = %v, want import", result["command"])
	}
	if result["feature_count"] == nil {
		t.Fatal("expected feature_count")
	}
	if result["normalized_path"] == nil {
		t.Fatal("expected normalized_path")
	}
}
```

**Step 2: Run test**

Run: `cd backend && go test ./internal/app/cli/... -run TestImportJSON -v`
Expected: FAIL

**Step 3: Implement**

The approach: build an `importResult` map throughout the import flow, then emit it at the end. Add a helper that accumulates result fields:

In `runImport`, add a `result map[string]any` that gets passed through and populated by each sub-function. After all import steps complete:

```go
if state.Config.JSONLogs {
	return writeCommandOutput(cmd.OutOrStdout(), true, result)
}
```

Each sub-function (`runGeometryImport`, `runOSMImport`, `runTerrainImport`, `mergeTrafficCSV`) populates the result map with its fields instead of (or before) calling `fmt.Fprintf`. The human-readable print functions remain unchanged for the non-JSON path.

Key JSON fields per import mode:

- **geometry**: `command`, `input`, `feature_count`, `normalized_path`, `dump_path`, `report_path`, `warnings`, `crs_transform` (optional object with `from`/`to`)
- **terrain**: adds `terrain_input`, `terrain_bounds`, `terrain_stored_path`
- **traffic**: adds `traffic_matched`, `traffic_unmatched`
- **citygml**: adds `citygml_imported`, `citygml_total`, `citygml_skipped`, `citygml_details`
- **osm**: adds `osm_bbox`

**Step 4: Run test**

Run: `cd backend && go test ./internal/app/cli/... -run TestImportJSON -v`
Expected: PASS

**Step 5: Commit**

```
feat(cli): emit structured JSON from import when --json is set
```

---

### Task 5: JSON output for `run`

**Files:**

- Modify: `backend/internal/app/cli/run_pipeline.go`

**Step 1: Write the failing test**

```go
func TestRunJSONOutput(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase8", "model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "RunJSON", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

	var buf bytes.Buffer
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"--json", "--project", projectDir, "run",
		"--standard", "dummy-freefield",
		"--param", "grid_resolution_m=10",
		"--param", "grid_padding_m=0",
		"--param", "source_emission_db=90",
		"--param", "receiver_height_m=4",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("run: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	if result["command"] != "run" {
		t.Fatalf("command = %v, want run", result["command"])
	}
	if result["run_id"] == nil || result["run_id"] == "" {
		t.Fatal("expected non-empty run_id")
	}
	if result["status"] != "completed" {
		t.Fatalf("status = %v, want completed", result["status"])
	}
}
```

**Step 2: Run test**

Run: `cd backend && go test ./internal/app/cli/... -run TestRunJSON -v -timeout 60s`
Expected: FAIL

**Step 3: Implement**

At the end of `executeRunCommand` (around line 729), before the `fmt.Fprintf` calls:

```go
if state.Config.JSONLogs {
	return writeCommandOutput(cmd.OutOrStdout(), true, map[string]any{
		"command":         "run",
		"run_id":          run.ID,
		"status":          string(project.RunStatusCompleted),
		"scenario":        run.ScenarioID,
		"standard":        run.Standard.ID,
		"standard_version": run.Standard.Version,
		"standard_profile": run.Standard.Profile,
		"provenance_path": provenance.ManifestPath,
		"results_path":    relativePath(store.Root(), filepath.Join(runDir, "results")),
	})
}
```

**Step 4: Run test**

Run: `cd backend && go test ./internal/app/cli/... -run TestRunJSON -v -timeout 60s`
Expected: PASS

**Step 5: Commit**

```
feat(cli): emit structured JSON from run when --json is set
```

---

### Task 6: JSON output for `status`

**Files:**

- Modify: `backend/internal/app/cli/status.go`

**Step 1: Write the failing test**

```go
func TestStatusJSONOutput(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "StatusJSON")

	var buf bytes.Buffer
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--json", "--project", projectDir, "status"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	if result["command"] != "status" {
		t.Fatalf("command = %v, want status", result["command"])
	}
	if result["project_name"] != "StatusJSON" {
		t.Fatalf("project_name = %v, want StatusJSON", result["project_name"])
	}
}
```

**Step 2: Run test → FAIL**

**Step 3: Implement**

Build a structured status object with runs array:

```go
if state.Config.JSONLogs {
	type runEntry struct {
		ID              string `json:"id"`
		Status          string `json:"status"`
		ScenarioID      string `json:"scenario"`
		StandardID      string `json:"standard"`
		StandardVersion string `json:"standard_version"`
		StandardProfile string `json:"standard_profile"`
		StartedAt       string `json:"started_at"`
		FinishedAt      string `json:"finished_at"`
		LogPath         string `json:"log_path"`
	}

	runs := make([]runEntry, 0, len(proj.Runs))
	start := max(len(proj.Runs)-limit, 0)
	for _, r := range proj.Runs[start:] {
		runs = append(runs, runEntry{
			ID: r.ID, Status: string(r.Status),
			ScenarioID: r.ScenarioID,
			StandardID: r.Standard.ID, StandardVersion: r.Standard.Version,
			StandardProfile: r.Standard.Profile,
			StartedAt: r.StartedAt.Format(time.RFC3339),
			FinishedAt: r.FinishedAt.Format(time.RFC3339),
			LogPath: r.LogPath,
		})
	}

	payload := map[string]any{
		"command":          "status",
		"project_id":       proj.ProjectID,
		"project_name":     proj.Name,
		"project_path":     store.Root(),
		"manifest_version": proj.ManifestVersion,
		"crs":              proj.CRS,
		"scenario_count":   len(proj.Scenarios),
		"runs":             runs,
	}
	if hasLatest {
		payload["last_run_id"] = latestRun.ID
		payload["last_run_status"] = string(latestRun.Status)
	}
	return writeCommandOutput(cmd.OutOrStdout(), true, payload)
}
```

**Step 4: Run test → PASS**

**Step 5: Commit**

```
feat(cli): emit structured JSON from status when --json is set
```

---

### Task 7: JSON output for `export`

**Files:**

- Modify: `backend/internal/app/cli/export.go`

**Step 1: Write the failing test**

```go
func TestExportJSONOutput(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase8", "model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "ExpJSON", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run",
		"--standard", "dummy-freefield",
		"--param", "grid_resolution_m=10",
		"--param", "grid_padding_m=0",
		"--param", "source_emission_db=90",
		"--param", "receiver_height_m=4",
	)

	var buf bytes.Buffer
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--json", "--project", projectDir, "export"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("export: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	if result["command"] != "export" {
		t.Fatalf("command = %v, want export", result["command"])
	}
	if result["run_id"] == nil {
		t.Fatal("expected run_id")
	}
	if result["bundle_dir"] == nil {
		t.Fatal("expected bundle_dir")
	}
}
```

**Step 2: Run test → FAIL**

**Step 3: Implement**

The `exportSummary` struct already has all the data. Wrap it:

```go
if state.Config.JSONLogs {
	return writeCommandOutput(cmd.OutOrStdout(), true, map[string]any{
		"command":      "export",
		"run_id":       run.ID,
		"bundle_dir":   bundleDir,
		"summary_path": summaryPath,
		"summary":      summary,
	})
}
```

**Step 4: Run test → PASS**

**Step 5: Commit**

```
feat(cli): emit structured JSON from export when --json is set
```

---

### Task 8: JSON output for `bench`

**Files:**

- Modify: `backend/internal/app/cli/bench.go`

**Step 1: Write the failing test**

```go
func TestBenchJSONOutput(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "BenchJSON")

	var buf bytes.Buffer
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--json", "--project", projectDir, "bench", "--scenario", "micro"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("bench: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	if result["command"] != "bench" {
		t.Fatalf("command = %v, want bench", result["command"])
	}
	if result["bench_id"] == nil {
		t.Fatal("expected bench_id")
	}
}
```

**Step 2: Run test → FAIL**

**Step 3: Implement**

The `benchSummary` struct already has all the data:

```go
if state.Config.JSONLogs {
	return writeCommandOutput(cmd.OutOrStdout(), true, map[string]any{
		"command":      "bench",
		"bench_id":     summary.BenchID,
		"summary_path": summaryPath,
		"summary":      summary,
	})
}
```

**Step 4: Run test → PASS**

**Step 5: Commit**

```
feat(cli): emit structured JSON from bench when --json is set
```

---

### Task 9: Update `--json` flag help text

**Files:**

- Modify: `backend/internal/app/cli/root.go`

Update the flag description from "Emit logs as JSON" to "Emit structured JSON output (stdout) and JSON logs (stderr)".

**Step 1: Edit root.go line 89**

```go
rootCmd.PersistentFlags().BoolVar(&jsonLogs, "json", false, "Emit structured JSON output (stdout) and JSON logs (stderr)")
```

**Step 2: Commit**

```
docs(cli): update --json flag description to reflect stdout output
```

---

### Task 10: Run full test suite and lint

**Step 1:** `cd backend && go test ./internal/app/cli/... -v -timeout 120s`
**Step 2:** `just lint`
**Step 3:** `just fmt`
**Step 4:** Fix any issues
**Step 5:** Final commit

```
chore(cli): fix lint and formatting for JSON output feature
```

---

## JSON Output Schema Reference

Every command's JSON output includes a `"command"` field identifying which command produced it. Per-command fields:

| Command    | Key fields                                                                                                                                          |
| ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `init`     | `project_id`, `project_name`, `project_path`, `manifest_path`, `crs`                                                                                |
| `import`   | `feature_count`, `normalized_path`, `dump_path`, `report_path`, `warnings`, `crs_transform?`, `terrain_*?`, `traffic_*?`, `citygml_*?`, `osm_bbox?` |
| `validate` | `input`, `feature_count`, `errors`, `warnings`, `report_path?`                                                                                      |
| `run`      | `run_id`, `status`, `scenario`, `standard`, `standard_version`, `standard_profile`, `provenance_path`, `results_path`                               |
| `status`   | `project_id`, `project_name`, `project_path`, `manifest_version`, `crs`, `scenario_count`, `runs[]`, `last_run_id?`, `last_run_status?`             |
| `export`   | `run_id`, `bundle_dir`, `summary_path`, `summary` (full exportSummary object)                                                                       |
| `bench`    | `bench_id`, `summary_path`, `summary` (full benchSummary object)                                                                                    |
