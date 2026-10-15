package projectfs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
)

func newTerrainStore(t *testing.T) (Store, project.Project) {
	t.Helper()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	proj, err := store.Init("Terrain", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	return store, proj
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

func TestTerrainHashReportsNotFoundBeforeAnyImport(t *testing.T) {
	t.Parallel()

	store, _ := newTerrainStore(t)

	_, err := store.TerrainHash()

	var appErr *domainerrors.AppError
	if !errors.As(err, &appErr) || appErr.Kind != domainerrors.KindNotFound {
		t.Fatalf("expected a NotFound error, got %v", err)
	}
}

// SaveTerrain registers the artifact `aconiq run` looks up, at the path the
// format names, and the hash follows the file through a replacement — the
// remembered hash must not outlive the file it was computed from.
func TestSaveTerrainRegistersTheArtifactAndTheHashFollowsTheFile(t *testing.T) {
	t.Parallel()

	store, proj := newTerrainStore(t)

	first := []byte("first terrain")

	err := store.SaveTerrain(&proj, first, "")
	if err != nil {
		t.Fatalf("save terrain: %v", err)
	}

	var ref project.ArtifactRef

	for _, a := range proj.Artifacts {
		if a.ID == project.ArtifactIDTerrain {
			ref = a
		}
	}

	if ref.Kind != project.ArtifactKindTerrainGeoTIFF || ref.Path != ".noise/model/terrain.tif" {
		t.Fatalf("terrain ref = %+v", ref)
	}

	sum, err := store.TerrainHash()
	if err != nil || sum != sha256Hex(first) {
		t.Fatalf("terrain hash = %q, %v; want %s", sum, err, sha256Hex(first))
	}

	second := []byte("second terrain, longer")

	err = store.SaveTerrain(&proj, second, sum)
	if err != nil {
		t.Fatalf("replace terrain: %v", err)
	}

	sum, err = store.TerrainHash()
	if err != nil || sum != sha256Hex(second) {
		t.Fatalf("terrain hash after replace = %q, %v; want %s", sum, err, sha256Hex(second))
	}
}

func TestSaveTerrainRefusesAStaleHashAndLeavesTheTerrainUntouched(t *testing.T) {
	t.Parallel()

	store, proj := newTerrainStore(t)

	original := []byte("original terrain")

	err := store.SaveTerrain(&proj, original, ExpectNone)
	if err != nil {
		t.Fatalf("first save expecting none: %v", err)
	}

	manifest, err := os.ReadFile(store.ManifestPath())
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	for _, expected := range []string{ExpectNone, sha256Hex([]byte("something else"))} {
		err = store.SaveTerrain(&proj, []byte("replacement"), expected)

		var mismatch *HashMismatchError
		if !errors.As(err, &mismatch) || mismatch.Current != sha256Hex(original) || mismatch.Subject != "terrain" {
			t.Fatalf("expecting %s: expected a terrain mismatch naming the original, got %v", expected, err)
		}
	}

	onDisk, err := os.ReadFile(store.TerrainPath())
	if err != nil || !bytes.Equal(onDisk, original) {
		t.Fatalf("a refused save changed terrain.tif: %q, %v", onDisk, err)
	}

	after, err := os.ReadFile(store.ManifestPath())
	if err != nil || !bytes.Equal(after, manifest) {
		t.Fatalf("a refused save changed the manifest: %v", err)
	}

	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(store.TerrainPath()), "*.tmp"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("a refused save left staged files behind: %v, %v", leftovers, err)
	}
}
