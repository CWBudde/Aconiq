package projectfs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/buildinfo"
	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
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

func TestStoreAccessorsAndExists(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	store, err := New(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if store.Root() != root {
		t.Fatalf("unexpected root %q", store.Root())
	}

	wantManifest := filepath.Join(root, ".noise", "project.json")
	if store.ManifestPath() != wantManifest {
		t.Fatalf("unexpected manifest path %q", store.ManifestPath())
	}

	exists, err := store.Exists()
	if err != nil {
		t.Fatalf("exists before init: %v", err)
	}

	if exists {
		t.Fatal("expected manifest to be absent before init")
	}

	_, err = store.Init("", "")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	exists, err = store.Exists()
	if err != nil {
		t.Fatalf("exists after init: %v", err)
	}

	if !exists {
		t.Fatal("expected manifest to exist after init")
	}
}

func TestNewRequiresProjectPath(t *testing.T) {
	t.Parallel()

	_, err := New("")
	if err == nil {
		t.Fatal("expected missing path error")
	}
}

func TestInitRejectsAlreadyInitializedProject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	store, err := New(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Demo", "EPSG:25832")
	if err != nil {
		t.Fatalf("initial init: %v", err)
	}

	_, err = store.Init("Again", "EPSG:25832")
	if err == nil {
		t.Fatal("expected already initialized error")
	}
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	store, err := New(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Load()
	if err == nil {
		t.Fatal("expected missing manifest error")
	} else {
		var appErr *domainerrors.AppError
		if !errors.As(err, &appErr) || appErr.Kind != domainerrors.KindNotFound {
			t.Fatalf("expected not found app error, got %v", err)
		}
	}

	err = os.MkdirAll(filepath.Dir(store.ManifestPath()), 0o750)
	if err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}

	err = os.WriteFile(store.ManifestPath(), []byte("{not-json"), 0o600)
	if err != nil {
		t.Fatalf("write invalid manifest: %v", err)
	}

	_, err = store.Load()
	if err == nil {
		t.Fatal("expected decode error")
	} else {
		var appErr *domainerrors.AppError
		if !errors.As(err, &appErr) || appErr.Kind != domainerrors.KindValidation {
			t.Fatalf("expected validation app error, got %v", err)
		}
	}

	invalidManifest := project.Project{Name: "Missing version"}

	payload, err := json.Marshal(invalidManifest)
	if err != nil {
		t.Fatalf("marshal invalid manifest: %v", err)
	}

	err = os.WriteFile(store.ManifestPath(), payload, 0o600)
	if err != nil {
		t.Fatalf("write invalid-version manifest: %v", err)
	}

	_, err = store.Load()
	if err == nil {
		t.Fatal("expected manifest version error")
	}
}

func TestSaveCreatesManifestDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	store, err := New(filepath.Join(root, "nested", "project"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	err = store.Save(project.Project{
		ManifestVersion: 1,
		ProjectID:       "proj-1",
		Name:            "Saved",
		CRS:             "EPSG:4326",
	})
	if err != nil {
		t.Fatalf("save project: %v", err)
	}

	_, err = os.Stat(store.ManifestPath())
	if err != nil {
		t.Fatalf("expected manifest file: %v", err)
	}
}

func TestCreateRunWritesProvenanceAndLog(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	store, err := New(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	{
		_, err := store.Init("", "")
		if err != nil {
			t.Fatalf("init project: %v", err)
		}
	}

	inputPath := filepath.Join(root, "input.geojson")

	err = os.WriteFile(inputPath, []byte(`{"type":"FeatureCollection","features":[]}`), 0o600)
	if err != nil {
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
		Metadata: map[string]string{
			"data_pack_version": "builtin-v1",
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
	{
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(run.ProvenancePath)))
		if err != nil {
			t.Fatalf("provenance file missing: %v", err)
		}
	}
	{
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(run.LogPath)))
		if err != nil {
			t.Fatalf("run log missing: %v", err)
		}
	}

	if len(provenance.InputHashes) != 1 {
		t.Fatalf("expected one input hash, got %d", len(provenance.InputHashes))
	}

	if provenance.Standard.ID != "dummy-freefield" {
		t.Fatalf("unexpected provenance standard: %q", provenance.Standard.ID)
	}

	if provenance.Metadata["data_pack_version"] != "builtin-v1" {
		t.Fatalf("unexpected provenance metadata: %#v", provenance.Metadata)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(loaded.Runs) != 1 {
		t.Fatalf("expected one run in manifest, got %d", len(loaded.Runs))
	}
}

func TestCreateRunDefaultsAndErrors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	store, err := New(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, _, err = store.CreateRun(CreateRunSpec{})
	if err == nil {
		t.Fatal("expected create run to fail before init")
	}

	proj, err := store.Init("", "")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	run, provenance, err := store.CreateRun(CreateRunSpec{
		Standard: project.StandardRef{
			ID: "dummy-freefield",
		},
	})
	if err != nil {
		t.Fatalf("create run with defaults: %v", err)
	}

	if run.ScenarioID != "default" {
		t.Fatalf("expected default scenario, got %q", run.ScenarioID)
	}

	if run.Status != project.RunStatusCompleted {
		t.Fatalf("expected default completed status, got %q", run.Status)
	}

	if run.Standard.Version != "v0" || run.Standard.Profile != "default" {
		t.Fatalf("unexpected defaulted standard %#v", run.Standard)
	}

	if provenance.ManifestPath == "" || !strings.HasSuffix(provenance.ManifestPath, "/provenance.json") {
		t.Fatalf("unexpected provenance manifest path %q", provenance.ManifestPath)
	}

	logData, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(run.LogPath)))
	if err != nil {
		t.Fatalf("read run log: %v", err)
	}

	if string(logData) != "run executed\n" {
		t.Fatalf("unexpected default log contents %q", string(logData))
	}

	missingScenarioProj := proj

	missingScenarioProj.Scenarios = nil

	err = store.Save(missingScenarioProj)
	if err != nil {
		t.Fatalf("save missing-scenario manifest: %v", err)
	}

	_, _, err = store.CreateRun(CreateRunSpec{ScenarioID: "missing"})
	if err == nil {
		t.Fatal("expected missing scenario error")
	}

	unassignedProj := proj

	unassignedProj.Scenarios[0].Standard = project.StandardRef{}

	err = store.Save(unassignedProj)
	if err != nil {
		t.Fatalf("save unassigned manifest: %v", err)
	}

	_, _, err = store.CreateRun(CreateRunSpec{})
	if err == nil {
		t.Fatal("expected missing standard error")
	}
}

func TestCreateRunInputHashErrors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	store, err := New(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("", "")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	_, _, err = store.CreateRun(CreateRunSpec{
		Standard: project.StandardRef{ID: "dummy-freefield"},
		InputPaths: []string{
			"missing.geojson",
		},
	})
	if err == nil {
		t.Fatal("expected missing input file error")
	}
}

// Provenance must name the tool that produced it. The version is stamped at
// link time and falls back to the embedded build info, so this asserts that it
// is present and not the old hardcoded placeholder rather than a fixed value
// that only `just build` could supply.
func TestCreateRunWritesToolIdentityIntoProvenance(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	store, err := New(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Provenance Project", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	run, provenance, err := store.CreateRun(CreateRunSpec{
		Standard: project.StandardRef{ID: "dummy-freefield", Version: "v1"},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	if provenance.ToolName != buildinfo.Name {
		t.Fatalf("provenance tool name = %q, want %q", provenance.ToolName, buildinfo.Name)
	}

	if strings.TrimSpace(provenance.ToolVersion) == "" {
		t.Fatal("provenance tool version must not be empty")
	}

	if provenance.ToolVersion != buildinfo.Version() {
		t.Fatalf("provenance tool version = %q, want %q", provenance.ToolVersion, buildinfo.Version())
	}

	payload, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance file: %v", err)
	}

	var persisted struct {
		ToolName    string `json:"tool_name"`
		ToolVersion string `json:"tool_version"`
	}

	err = json.Unmarshal(payload, &persisted)
	if err != nil {
		t.Fatalf("decode provenance file: %v", err)
	}

	if persisted.ToolName != buildinfo.Name || strings.TrimSpace(persisted.ToolVersion) == "" {
		t.Fatalf("provenance.json carries no usable tool identity: %+v", persisted)
	}
}

func TestCreateRunPersistsStandardDataOutsideInputHashes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	store, err := New(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("", "")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	inputPath := filepath.Join(root, "input.geojson")

	err = os.WriteFile(inputPath, []byte(`{"type":"FeatureCollection","features":[]}`), 0o600)
	if err != nil {
		t.Fatalf("write input file: %v", err)
	}

	standardData := project.StandardDataRef{
		Algorithm:    "sha256",
		Digest:       "aaaa",
		EvidenceTier: "normative",
		Tables: []project.StandardDataTableRef{
			{Name: "anlage2/tabelle-06-geschwindigkeitsfaktor", Digest: "bbbb"},
		},
	}

	run, _, err := store.CreateRun(CreateRunSpec{
		ScenarioID:   "default",
		Standard:     project.StandardRef{ID: "schall03", Version: "2014-anlage2", Profile: "rail-planning-preview"},
		StandardData: standardData,
		InputPaths:   []string{"input.geojson"},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	// The spec is the caller's; mutating it afterwards must not reach the file.
	standardData.Digest = "mutated"
	standardData.Tables[0].Digest = "mutated"

	payload, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	var written project.ProvenanceManifest

	err = json.Unmarshal(payload, &written)
	if err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	if written.StandardData == nil {
		t.Fatal("expected standard_data in the written provenance")
	}

	if written.StandardData.Digest != "aaaa" || written.StandardData.Algorithm != "sha256" {
		t.Fatalf("unexpected standard data: %#v", written.StandardData)
	}

	if len(written.StandardData.Tables) != 1 || written.StandardData.Tables[0].Digest != "bbbb" {
		t.Fatalf("unexpected standard data tables: %#v", written.StandardData.Tables)
	}

	// input_hashes is defined as input-file path to SHA-256; a coefficient
	// table is not a file anyone supplied and must never appear there.
	if len(written.InputHashes) != 1 {
		t.Fatalf("expected only the input file in input_hashes, got %#v", written.InputHashes)
	}

	if _, ok := written.InputHashes["input.geojson"]; !ok {
		t.Fatalf("expected the input file in input_hashes, got %#v", written.InputHashes)
	}
}

func TestMergeRunProvenanceMetadataKeepsStandardData(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	store, err := New(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("", "")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	run, _, err := store.CreateRun(CreateRunSpec{
		ScenarioID: "default",
		Standard:   project.StandardRef{ID: "schall03", Version: "2014-anlage2", Profile: "rail-planning-preview"},
		StandardData: project.StandardDataRef{
			Algorithm:    "sha256",
			Digest:       "aaaa",
			EvidenceTier: "normative",
			Tables:       []project.StandardDataTableRef{{Name: "anlage2/tabelle-09-bruecken", Digest: "bbbb"}},
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	err = store.MergeRunProvenanceMetadata(run.ID, map[string]string{"schall03_engine": "normative"})
	if err != nil {
		t.Fatalf("merge provenance metadata: %v", err)
	}

	payload, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	var written project.ProvenanceManifest

	err = json.Unmarshal(payload, &written)
	if err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	if written.Metadata["schall03_engine"] != "normative" {
		t.Fatalf("expected the merged engine key, got %#v", written.Metadata)
	}

	if written.StandardData == nil || written.StandardData.Digest != "aaaa" {
		t.Fatalf("the merge dropped the standard data digest: %#v", written.StandardData)
	}
}
