package projectfs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
)

// TestConcurrentSaveNeverLosesTheTemporaryFile drives the failure the fixed
// temp-file name made possible.
//
// Save wrote manifestPath()+".tmp" — one name, shared by every writer in every
// process. Two savers then run:
//
//	A: WriteFile(tmp, a)   creates and fills the temp file
//	B: WriteFile(tmp, b)   truncates the same file and refills it
//	A: Rename(tmp, dst)    succeeds, carrying B's bytes
//	B: Rename(tmp, dst)    fails — the temp file is gone
//
// so one writer reports "replace project manifest" against a file it wrote
// correctly, and a longer manifest can also be torn between the two writes.
// The `aconiq run` subprocess writes this same manifest, so a mutex in the
// server cannot close it; a unique temp name per write can.
func TestConcurrentSaveNeverLosesTheTemporaryFile(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Concurrency", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// A manifest large enough that the write is not one syscall, which is what
	// makes tearing reachable as well as the lost temp file.
	for i := range 400 {
		proj.Artifacts = append(proj.Artifacts, project.ArtifactRef{
			ID:   "artifact-" + strconv.Itoa(i),
			Kind: "model.normalized_geojson",
			Path: ".noise/model/model.normalized.geojson",
		})
	}

	const writers = 8

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		fails []error
	)

	record := func(err error) {
		mu.Lock()
		defer mu.Unlock()

		fails = append(fails, err)
	}

	for range writers {
		wg.Go(func() {
			for range 25 {
				saveErr := store.Save(proj)
				if saveErr != nil {
					record(saveErr)

					return
				}
			}
		})
	}

	wg.Wait()

	if len(fails) > 0 {
		t.Fatalf("concurrent Save failed %d times, first: %v", len(fails), fails[0])
	}

	payload, err := os.ReadFile(store.ManifestPath())
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var parsed project.Project

	err = json.Unmarshal(payload, &parsed)
	if err != nil {
		t.Fatalf("manifest is not valid JSON after concurrent saves (%d bytes): %v", len(payload), err)
	}

	if len(parsed.Artifacts) != len(proj.Artifacts) {
		t.Fatalf("manifest holds %d artifacts, want %d", len(parsed.Artifacts), len(proj.Artifacts))
	}
}

// TestConcurrentSaveLeavesNoTemporaryFileBehind pins the other half: whatever
// temp names a write picks, none of them survives it.
func TestConcurrentSaveLeavesNoTemporaryFileBehind(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	store, err := New(root)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Leftovers", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	var wg sync.WaitGroup

	for range 8 {
		wg.Go(func() {
			for range 25 {
				_ = store.Save(proj)
			}
		})
	}

	wg.Wait()

	entries, err := os.ReadDir(filepath.Dir(store.ManifestPath()))
	if err != nil {
		t.Fatalf("read control dir: %v", err)
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".tmp" || len(entry.Name()) > 4 && entry.Name()[:8] == "project." && entry.Name() != "project.json" {
			t.Fatalf("temporary file %q survived the writes", entry.Name())
		}
	}
}
