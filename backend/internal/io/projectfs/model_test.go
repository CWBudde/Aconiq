package projectfs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

// Everything the model hash is good for rests on this: the normalized file is a
// pure function of the model's content. ToFeatureCollection stamps no timestamp
// and no source path, writeJSONFile marshals with sorted map keys, so saving the
// same model twice writes the same bytes and yields the same receipt.
func TestSaveModelIsByteDeterministic(t *testing.T) {
	t.Parallel()

	model := sampleModel()

	first := hashOfSavedModel(t, model)
	second := hashOfSavedModel(t, model)

	if first != second {
		t.Fatalf("identical models hashed differently: %s vs %s", first, second)
	}

	if len(first) != 64 {
		t.Fatalf("expected a bare 64-character hex digest, got %q", first)
	}

	if first != strings.ToLower(first) {
		t.Fatalf("expected lowercase hex, got %q", first)
	}
}

func TestModelHashChangesWithTheModel(t *testing.T) {
	t.Parallel()

	other := sampleModel()
	other.Features[0].ID = "r2"

	if hashOfSavedModel(t, sampleModel()) == hashOfSavedModel(t, other) {
		t.Fatal("two different models produced the same hash")
	}
}

func TestReadModelAndModelHashReportNotFoundBeforeAnySave(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("No Model", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	for name, call := range map[string]func() error{
		"ReadModel": func() error { _, callErr := store.ReadModel(); return callErr },
		"ModelHash": func() error { _, callErr := store.ModelHash(); return callErr },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			callErr := call()

			var appErr *domainerrors.AppError
			if !errors.As(callErr, &appErr) {
				t.Fatalf("expected a domain error, got %T: %v", callErr, callErr)
			}

			if appErr.Kind != domainerrors.KindNotFound {
				t.Fatalf("expected KindNotFound, got %q", appErr.Kind)
			}
		})
	}
}

func TestReadModelReturnsTheStoredBytes(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	proj, err := store.Init("Read Model", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	model := sampleModel()

	err = store.SaveModel(&proj, model, modelgeojson.Validate(model))
	if err != nil {
		t.Fatalf("save model: %v", err)
	}

	raw, err := store.ReadModel()
	if err != nil {
		t.Fatalf("read model: %v", err)
	}

	onDisk, err := os.ReadFile(store.ModelArtifactPaths().Normalized)
	if err != nil {
		t.Fatalf("read normalized model: %v", err)
	}

	if !bytes.Equal(raw, onDisk) {
		t.Fatal("ReadModel did not return the stored bytes verbatim")
	}
}

// The hash a client stores is a receipt for bytes it was handed. Reading the
// file and hashing the path are two reads, and a save between them would pair
// one model with another model's hash — so the hash has to come from the bytes
// that were returned.
func TestReadModelWithHashPairsTheBytesWithTheirOwnHash(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	proj, err := store.Init("Read Model", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	model := sampleModel()

	err = store.SaveModel(&proj, model, modelgeojson.Validate(model))
	if err != nil {
		t.Fatalf("save model: %v", err)
	}

	raw, hash, err := store.ReadModelWithHash()
	if err != nil {
		t.Fatalf("read model with hash: %v", err)
	}

	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != hash {
		t.Fatal("the reported hash is not the hash of the reported bytes")
	}

	// The same spelling ModelHash and provenance.json use, so a client can
	// compare the two without normalising either.
	fileHash, err := store.ModelHash()
	if err != nil {
		t.Fatalf("model hash: %v", err)
	}

	if fileHash != hash {
		t.Fatalf("ReadModelWithHash says %q, ModelHash says %q", hash, fileHash)
	}
}

func TestReadModelWithHashReportsNotFoundBeforeAnySave(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("No Model", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	_, _, err = store.ReadModelWithHash()

	var appErr *domainerrors.AppError
	if !errors.As(err, &appErr) || appErr.Kind != domainerrors.KindNotFound {
		t.Fatalf("expected a not-found domain error, got %v", err)
	}
}

func sampleModel() modelgeojson.Model {
	height := 4.0

	return modelgeojson.Model{
		SchemaVersion: 1,
		ProjectCRS:    "EPSG:25832",
		ImportedAt:    time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		SourcePath:    "api:model",
		Features: []modelgeojson.Feature{
			{ID: "r1", Kind: modelgeojson.FeatureKindReceiver, HeightM: &height, GeometryType: "Point", Coordinates: []any{1.0, 2.0}},
		},
	}
}

// hashOfSavedModel saves model into a fresh project and returns its receipt.
func hashOfSavedModel(t *testing.T, model modelgeojson.Model) string {
	t.Helper()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	proj, err := store.Init("Hash", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	err = store.SaveModel(&proj, model, modelgeojson.Validate(model))
	if err != nil {
		t.Fatalf("save model: %v", err)
	}

	sum, err := store.ModelHash()
	if err != nil {
		t.Fatalf("model hash: %v", err)
	}

	return sum
}
