package projectfs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

func TestSaveModelWritesArtifactsAndManifest(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	proj, err := store.Init("Model", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	height := 4.0
	model := modelgeojson.Model{
		SchemaVersion: 1,
		ProjectCRS:    "EPSG:25832",
		ImportedAt:    time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		SourcePath:    "api:model",
		Features: []modelgeojson.Feature{
			{ID: "r1", Kind: modelgeojson.FeatureKindReceiver, HeightM: &height, GeometryType: "Point", Coordinates: []any{1.0, 2.0}},
		},
	}
	report := modelgeojson.Validate(model)

	err = store.SaveModel(&proj, model, report)
	if err != nil {
		t.Fatalf("save model: %v", err)
	}

	paths := store.ModelArtifactPaths()

	wantRelative := map[string]string{
		paths.Normalized: ".noise/model/model.normalized.geojson",
		paths.Dump:       ".noise/model/model.dump.json",
		paths.Validation: ".noise/model/validation-report.json",
	}

	for abs, rel := range wantRelative {
		if _, err := os.Stat(abs); err != nil {
			t.Fatalf("expected %s to exist: %v", abs, err)
		}

		if got := store.RelativePath(abs); got != rel {
			t.Fatalf("expected relative path %q, got %q", rel, got)
		}
	}

	var normalized modelgeojson.FeatureCollection

	payload, err := os.ReadFile(paths.Normalized)
	if err != nil {
		t.Fatalf("read normalized model: %v", err)
	}

	err = json.Unmarshal(payload, &normalized)
	if err != nil {
		t.Fatalf("decode normalized model: %v", err)
	}

	if len(normalized.Features) != 1 {
		t.Fatalf("expected 1 normalized feature, got %d", len(normalized.Features))
	}

	// The manifest on disk, not only the in-memory copy, must carry the refs.
	saved, err := store.Load()
	if err != nil {
		t.Fatalf("reload project: %v", err)
	}

	assertModelArtifactRefs(t, saved.Artifacts)
	assertModelArtifactRefs(t, proj.Artifacts)
}

func TestSaveModelReplacesExistingRefsInsteadOfDuplicating(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	proj, err := store.Init("Model", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	model := modelgeojson.Model{SchemaVersion: 1, ProjectCRS: "EPSG:25832", Features: []modelgeojson.Feature{}}
	report := modelgeojson.Validate(model)

	for range 2 {
		err = store.SaveModel(&proj, model, report)
		if err != nil {
			t.Fatalf("save model: %v", err)
		}
	}

	if len(proj.Artifacts) != 3 {
		t.Fatalf("expected exactly 3 model artifact refs after two saves, got %d", len(proj.Artifacts))
	}
}

func assertModelArtifactRefs(t *testing.T, artifacts []project.ArtifactRef) {
	t.Helper()

	want := map[string]struct{ kind, path string }{
		project.ArtifactIDModelNormalized: {project.ArtifactKindModelNormalizedGeoJSON, ".noise/model/model.normalized.geojson"},
		project.ArtifactIDModelDump:       {project.ArtifactKindModelDumpJSON, ".noise/model/model.dump.json"},
		project.ArtifactIDModelValidation: {project.ArtifactKindModelValidationReport, ".noise/model/validation-report.json"},
	}

	for id, expected := range want {
		found := false

		for _, ref := range artifacts {
			if ref.ID != id {
				continue
			}

			found = true

			if ref.Kind != expected.kind {
				t.Errorf("%s: expected kind %q, got %q", id, expected.kind, ref.Kind)
			}

			if ref.Path != expected.path {
				t.Errorf("%s: expected path %q, got %q", id, expected.path, ref.Path)
			}

			if ref.CreatedAt.IsZero() {
				t.Errorf("%s: expected created_at to be set", id)
			}
		}

		if !found {
			t.Errorf("artifact %s missing from manifest", id)
		}
	}
}

func TestSaveModelLeavesNoTemporaryFiles(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	proj, err := store.Init("Model", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	model := modelgeojson.Model{SchemaVersion: 1, ProjectCRS: "EPSG:25832", Features: []modelgeojson.Feature{}}

	err = store.SaveModel(&proj, model, modelgeojson.Validate(model))
	if err != nil {
		t.Fatalf("save model: %v", err)
	}

	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(store.ModelArtifactPaths().Normalized), "*.tmp"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	if len(leftovers) != 0 {
		t.Fatalf("expected the temporary files to be renamed away, found %v", leftovers)
	}
}

func TestSaveModelErrorMessageNamesTheStepNotThePath(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	proj, err := store.Init("Model", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	// A regular file in the model directory's place makes the first write fail.
	err = os.WriteFile(filepath.Join(store.Root(), ".noise", "model"), []byte("blocker"), 0o600)
	if err != nil {
		t.Fatalf("plant blocking file: %v", err)
	}

	model := modelgeojson.Model{SchemaVersion: 1, ProjectCRS: "EPSG:25832", Features: []modelgeojson.Feature{}}

	err = store.SaveModel(&proj, model, modelgeojson.Validate(model))
	if err == nil {
		t.Fatal("expected SaveModel to fail")
	}

	var appErr *domainerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected a domain error, got %T: %v", err, err)
	}

	if strings.Contains(appErr.Msg, store.Root()) {
		t.Fatalf("the message must not carry the absolute path: %q", appErr.Msg)
	}

	if !strings.Contains(err.Error(), store.Root()) {
		t.Fatalf("the wrapped cause should keep the path for the server-side log: %q", err.Error())
	}
}
