package engine

import (
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// The shared chunk cache is keyed on the format version, the chunk's
// receivers, the sources and the source-index cell size - and on nothing that
// says which standard computed the levels inside. Two runs of different
// standards over the same model therefore hashed to the same path, and the
// second read the first's answers back.
//
// It is latent while the engine hard-codes dummy/freefield, and it stops being
// latent the moment "Generalise the engine" parameterises the kernel. Pinning
// it now means that change does not also have to carry a cache-correctness
// fix.
func TestSharedChunkCacheKeySeparatesStandards(t *testing.T) {
	t.Parallel()

	chunk := receiverChunk{
		Index: 0,
		Receivers: []Receiver{
			{ID: "rx-0", Point: geo.Point2D{X: 1, Y: 2}, HeightM: 4},
		},
	}

	base := RunConfig{
		Sources:          []Source{{ID: "s1", Point: geo.Point2D{X: 10, Y: 10}, Emission: 91.2}},
		SourceIndexCellM: 50,
		StandardKey: StandardKey{
			StandardID: "rls19-road",
			Version:    "2019",
			Profile:    "default",
		},
	}

	other := base
	other.StandardKey.StandardID = "schall03"

	dir := t.TempDir()

	basePath, err := sharedChunkCachePath(dir, base, chunk)
	if err != nil {
		t.Fatalf("key for %s: %v", base.StandardKey.StandardID, err)
	}

	otherPath, err := sharedChunkCachePath(dir, other, chunk)
	if err != nil {
		t.Fatalf("key for %s: %v", other.StandardKey.StandardID, err)
	}

	if basePath == otherPath {
		t.Fatalf("rls19-road and schall03 share the cache entry %s", filepath.Base(basePath))
	}
}

// The version and the profile are part of the tuple too: a re-resolved
// standard is a different calculation even under the same id.
func TestSharedChunkCacheKeySeparatesVersionsAndProfiles(t *testing.T) {
	t.Parallel()

	chunk := receiverChunk{
		Index:     0,
		Receivers: []Receiver{{ID: "rx-0", Point: geo.Point2D{X: 1, Y: 2}, HeightM: 4}},
	}

	base := RunConfig{
		Sources:          []Source{{ID: "s1", Point: geo.Point2D{X: 10, Y: 10}, Emission: 91.2}},
		SourceIndexCellM: 50,
		StandardKey:      StandardKey{StandardID: "schall03", Version: "2014-anlage2", Profile: "default"},
	}

	dir := t.TempDir()

	basePath, err := sharedChunkCachePath(dir, base, chunk)
	if err != nil {
		t.Fatalf("base key: %v", err)
	}

	for _, variant := range []struct {
		name string
		cfg  RunConfig
	}{
		{"version", func() RunConfig { c := base; c.StandardKey.Version = "baseline-preview-rail-v1"; return c }()},
		{"profile", func() RunConfig { c := base; c.StandardKey.Profile = "strict"; return c }()},
	} {
		t.Run(variant.name, func(t *testing.T) {
			t.Parallel()

			path, err := sharedChunkCachePath(dir, variant.cfg, chunk)
			if err != nil {
				t.Fatalf("variant key: %v", err)
			}

			if path == basePath {
				t.Fatalf("a different %s shares the cache entry %s", variant.name, filepath.Base(path))
			}
		})
	}
}

// And the constraint that stops the obvious shortcut: DeterminismTag stays out
// of the key.
//
// PLAN.md gives the right conclusion for the wrong mechanism, so the accurate
// version is here rather than there. It says `aconiq bench` sets the tag to
// "bench-cold" and "bench-warm" "precisely so those two runs share the cache".
// They do share it, but not through the tag - the tag is a free-text label
// that reaches no cache path at all. They share it because both configs pass
// the same RunID, and computeOrLoadChunk consults the run-local cache
// (chunk-NNNNNN.json, keyed by run id and chunk index) before the shared one.
//
// The conclusion survives the correction. Two runs of the same calculation
// routinely carry different tags, so keying on it would make the shared cache
// miss wherever it should hit - which is every run after the first.
func TestSharedChunkCacheKeyIgnoresTheDeterminismTag(t *testing.T) {
	t.Parallel()

	chunk := receiverChunk{
		Index:     0,
		Receivers: []Receiver{{ID: "rx-0", Point: geo.Point2D{X: 1, Y: 2}, HeightM: 4}},
	}

	cold := RunConfig{
		Sources:          []Source{{ID: "s1", Point: geo.Point2D{X: 10, Y: 10}, Emission: 91.2}},
		SourceIndexCellM: 50,
		StandardKey:      StandardKey{StandardID: "dummy-freefield", Version: "v1", Profile: "default"},
		DeterminismTag:   "bench-cold",
	}

	warm := cold
	warm.DeterminismTag = "bench-warm"

	dir := t.TempDir()

	coldPath, err := sharedChunkCachePath(dir, cold, chunk)
	if err != nil {
		t.Fatalf("cold key: %v", err)
	}

	warmPath, err := sharedChunkCachePath(dir, warm, chunk)
	if err != nil {
		t.Fatalf("warm key: %v", err)
	}

	if coldPath != warmPath {
		t.Fatal("the determinism tag reached the cache key, so two runs of one calculation no longer share an entry")
	}
}
