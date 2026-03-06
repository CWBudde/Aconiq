package projectfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/soundplan/soundplan/backend/internal/domain/project"
)

func TestInitAndLoadProject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	created, err := store.Init("Demo Project", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if loaded.ProjectID != created.ProjectID {
		t.Fatalf("expected project ID %q, got %q", created.ProjectID, loaded.ProjectID)
	}
	if loaded.Name != "Demo Project" {
		t.Fatalf("unexpected project name: %q", loaded.Name)
	}
	if loaded.CRS != "EPSG:25832" {
		t.Fatalf("unexpected CRS: %q", loaded.CRS)
	}
	if loaded.Storage.Kind != project.StorageKindJSONV1 {
		t.Fatalf("unexpected storage kind: %q", loaded.Storage.Kind)
	}
	if len(loaded.Scenarios) != 1 || loaded.Scenarios[0].ID != "default" {
		t.Fatalf("expected one default scenario, got %#v", loaded.Scenarios)
	}
}

func TestCreateRunWritesProvenanceAndLog(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if _, err := store.Init("", ""); err != nil {
		t.Fatalf("init project: %v", err)
	}

	inputPath := filepath.Join(root, "input.geojson")
	if err := os.WriteFile(inputPath, []byte(`{"type":"FeatureCollection","features":[]}`), 0o644); err != nil {
		t.Fatalf("write input file: %v", err)
	}

	run, provenance, err := store.CreateRun(CreateRunSpec{
		ScenarioID: "default",
		Standard: project.StandardRef{
			ID:      "dummy-freefield",
			Version: "v1",
			Profile: "default",
		},
		Parameters: map[string]string{
			"receiver_set": "grid-a",
		},
		InputPaths: []string{"input.geojson"},
		LogLines:   []string{"starting run", "finished run"},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	if run.ID == "" {
		t.Fatal("run ID must be set")
	}
	if run.ProvenancePath == "" {
		t.Fatal("run provenance path must be set")
	}

	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(run.ProvenancePath))); err != nil {
		t.Fatalf("provenance file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(run.LogPath))); err != nil {
		t.Fatalf("run log missing: %v", err)
	}

	if len(provenance.InputHashes) != 1 {
		t.Fatalf("expected one input hash, got %d", len(provenance.InputHashes))
	}
	if provenance.Standard.ID != "dummy-freefield" {
		t.Fatalf("unexpected provenance standard: %q", provenance.Standard.ID)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	if len(loaded.Runs) != 1 {
		t.Fatalf("expected one run in manifest, got %d", len(loaded.Runs))
	}
}
