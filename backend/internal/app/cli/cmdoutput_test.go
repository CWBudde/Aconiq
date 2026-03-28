package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWriteCommandOutputJSON(t *testing.T) {
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

	err = json.Unmarshal(buf.Bytes(), &got)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Name != "test" || got.Count != 42 {
		t.Fatalf("got %+v, want {test 42}", got)
	}
}

func TestWriteCommandOutputNoOpWhenDisabled(t *testing.T) {
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

func TestInitJSONOutput(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	var buf bytes.Buffer

	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--json", "--project", projectDir, "init", "--name", "TestJSON"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	var result map[string]any

	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
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

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	var result map[string]any

	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	if result["command"] != "validate" {
		t.Fatalf("command = %v, want validate", result["command"])
	}

	if result["feature_count"] == nil {
		t.Fatal("expected feature_count")
	}
}

func TestImportJSONOutput(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase8", "model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "ImpJSON", "--crs", "EPSG:25832")

	var buf bytes.Buffer

	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--json", "--project", projectDir, "import", "--input", modelPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	var result map[string]any

	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
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

	if result["dump_path"] == nil {
		t.Fatal("expected dump_path")
	}

	if result["report_path"] == nil {
		t.Fatal("expected report_path")
	}
}

func TestImportSoundPlanJSONOutput(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	soundPlanDir := soundPlanInteropPath(t)

	mustRunCLI(t, "--project", projectDir, "init", "--name", "ImpSoundPlanJSON", "--crs", "EPSG:25832")

	var buf bytes.Buffer

	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--json", "--project", projectDir, "import", "--from-soundplan", soundPlanDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("import soundplan: %v", err)
	}

	var result map[string]any

	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	if result["command"] != "import" {
		t.Fatalf("command = %v, want import", result["command"])
	}

	if result["soundplan_source"] == nil || result["soundplan_source"] == "" {
		t.Fatal("expected soundplan_source")
	}

	if result["soundplan_report_path"] == nil || result["soundplan_report_path"] == "" {
		t.Fatal("expected soundplan_report_path")
	}
}

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

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	var result map[string]any

	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
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

func TestStatusJSONOutput(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "StatusJSON")

	var buf bytes.Buffer

	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--json", "--project", projectDir, "status"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	var result map[string]any

	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	if result["command"] != "status" {
		t.Fatalf("command = %v, want status", result["command"])
	}

	if result["project_name"] != "StatusJSON" {
		t.Fatalf("project_name = %v, want StatusJSON", result["project_name"])
	}
}

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

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	var result map[string]any

	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
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

func TestBenchJSONOutput(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "BenchJSON")

	var buf bytes.Buffer

	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--json", "--project", projectDir, "bench", "--scenario", "micro"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("bench: %v", err)
	}

	var result map[string]any

	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	if result["command"] != "bench" {
		t.Fatalf("command = %v, want bench", result["command"])
	}

	if result["bench_id"] == nil {
		t.Fatal("expected bench_id")
	}
}
