package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// writeJSONFile had no test of its own until this one, which is part of how it
// stayed a bare os.WriteFile while its projectfs twin renamed a temporary file
// into place. The artifacts it writes - run summaries, validation reports,
// compare artifacts, bench output, both import reports - were therefore
// replaced atomically through the HTTP API and non-atomically through the CLI.
func TestWriteJSONFileReplacesAnArtifactWhole(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "run-summary.json")

	small := map[string]any{"status": "completed"}
	large := map[string]any{"status": "completed", "notes": strings.Repeat("x", 256*1024)}

	err := writeJSONFile(path, small)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	var wg sync.WaitGroup

	stop := make(chan struct{})

	wg.Go(func() {
		defer close(stop)

		for range 30 {
			for _, value := range []map[string]any{large, small} {
				writeErr := writeJSONFile(path, value)
				if writeErr != nil {
					t.Errorf("write: %v", writeErr)

					return
				}
			}
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}

			payload, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Errorf("read: %v", readErr)

				return
			}

			var decoded map[string]any

			readErr = json.Unmarshal(payload, &decoded)
			if readErr != nil {
				t.Errorf("read a partial artifact of %d bytes: %v", len(payload), readErr)

				return
			}
		}
	})

	wg.Wait()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}

	for _, entry := range entries {
		if entry.Name() != filepath.Base(path) {
			t.Fatalf("temporary file %q survived the writes", entry.Name())
		}
	}
}

// The directory is still created, which several callers rely on: they build a
// path under .noise/ for a subdirectory no run has needed yet.
func TestWriteJSONFileStillCreatesTheParentDirectory(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "model", "validation-report.json")

	err := writeJSONFile(path, map[string]any{"errors": []string{}})
	if err != nil {
		t.Fatalf("write into a directory that does not exist yet: %v", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if !strings.HasSuffix(string(payload), "\n") {
		t.Fatal("the artifact lost its trailing newline")
	}
}
