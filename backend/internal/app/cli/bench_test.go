package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBenchGeneratesSummaryAndReusesCache(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	cacheDir := filepath.Join(projectDir, ".noise", "cache")

	cmd := newRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--project", projectDir,
		"--cache-dir", cacheDir,
		"bench",
		"--scenario", "micro",
		"--workers", "2",
		"--chunk-size", "32",
		"--keep-last", "2",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("run bench command: %v", err)
	}

	if !strings.Contains(stdout.String(), "Benchmark suite") {
		t.Fatalf("expected benchmark summary in stdout, got %q", stdout.String())
	}

	summaryPath := latestBenchSummaryPath(t, cacheDir)

	payload, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}

	var summary benchSummary
	err = json.Unmarshal(payload, &summary)
	if err != nil {
		t.Fatalf("decode summary: %v", err)
	}

	if len(summary.ScenarioResults) != 1 {
		t.Fatalf("expected one scenario result, got %d", len(summary.ScenarioResults))
	}

	result := summary.ScenarioResults[0]
	if result.Scenario.Name != "micro" {
		t.Fatalf("unexpected scenario: %s", result.Scenario.Name)
	}

	if result.WarmCache.UsedCachedChunks == 0 {
		t.Fatalf("expected warm cache run to reuse chunk cache, got %+v", result.WarmCache)
	}

	if !result.NumericDrift.HashMatch {
		t.Fatalf("expected identical hash for numeric drift check, got %+v", result.NumericDrift)
	}

	if result.NumericDrift.MaxAbsLevelDeltaDB != 0 {
		t.Fatalf("expected zero drift, got %+v", result.NumericDrift)
	}
}

func TestBenchPrunesOlderSuites(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	cacheDir := filepath.Join(projectDir, ".noise", "cache")
	benchRoot := filepath.Join(cacheDir, "bench")

	for _, suiteID := range []string{
		"20260308T000000.000000001Z",
		"20260308T000000.000000002Z",
		"20260308T000000.000000003Z",
	} {
		err := os.MkdirAll(filepath.Join(benchRoot, suiteID), 0o755)
		if err != nil {
			t.Fatalf("create suite %s: %v", suiteID, err)
		}
	}

	pruned, err := pruneBenchSuites(benchRoot, 2)
	if err != nil {
		t.Fatalf("prune bench suites: %v", err)
	}

	if len(pruned) != 1 {
		t.Fatalf("expected one pruned suite, got %d (%v)", len(pruned), pruned)
	}

	entries, err := os.ReadDir(benchRoot)
	if err != nil {
		t.Fatalf("read bench root: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected two suites to remain, got %d", len(entries))
	}
}

func latestBenchSummaryPath(t *testing.T, cacheDir string) string {
	t.Helper()

	benchRoot := filepath.Join(cacheDir, "bench")

	entries, err := os.ReadDir(benchRoot)
	if err != nil {
		t.Fatalf("read bench root: %v", err)
	}

	latest := ""

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if entry.Name() > latest {
			latest = entry.Name()
		}
	}

	if latest == "" {
		t.Fatal("expected at least one benchmark suite")
	}

	return filepath.Join(benchRoot, latest, "summary.json")
}
