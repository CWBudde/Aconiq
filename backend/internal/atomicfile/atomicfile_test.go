package atomicfile

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestWriteFileNeverLeavesAReaderAPartialFile is the property the in-place
// os.WriteFile it replaces cannot hold.
//
// A reader running alongside a writer sees the destination at some arbitrary
// moment. With os.WriteFile that moment can fall between the truncate and the
// last byte, so the reader gets a prefix of the new contents - valid bytes,
// invalid JSON. With a rename it can only fall before or after, so the reader
// gets one whole version or the other.
func TestWriteFileNeverLeavesAReaderAPartialFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "artifact.json")

	short := []byte(strings.Repeat("a", 64))
	long := []byte(strings.Repeat("b", 512*1024))

	err := WriteFile(path, short)
	if err != nil {
		t.Fatalf("seed the file: %v", err)
	}

	var wg sync.WaitGroup

	stop := make(chan struct{})

	wg.Go(func() {
		for range 50 {
			writeErr := WriteFile(path, long)
			if writeErr != nil {
				t.Errorf("write long: %v", writeErr)

				break
			}

			writeErr = WriteFile(path, short)
			if writeErr != nil {
				t.Errorf("write short: %v", writeErr)

				break
			}
		}

		close(stop)
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

			if len(payload) != len(short) && len(payload) != len(long) {
				t.Errorf("read a partial file of %d bytes, want %d or %d", len(payload), len(short), len(long))

				return
			}
		}
	})

	wg.Wait()
}

// TestWriteFileLeavesNoTemporaryFileBehind pins the other half: whatever
// temporary names a burst of writes picks, none of them survives it.
func TestWriteFileLeavesNoTemporaryFileBehind(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "artifact.json")

	var wg sync.WaitGroup

	for range 8 {
		wg.Go(func() {
			for range 25 {
				_ = WriteFile(path, []byte("{}"))
			}
		})
	}

	wg.Wait()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}

	for _, entry := range entries {
		if entry.Name() == filepath.Base(path) {
			continue
		}

		t.Fatalf("temporary file %q survived the writes", entry.Name())
	}
}

// TestWriteFileReplacesInPlaceWithoutChangingTheMode pins that swapping the
// mechanism did not swap the permissions the previous writers wrote.
func TestWriteFileReplacesInPlaceWithoutChangingTheMode(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "artifact.json")

	err := WriteFile(path, []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(payload) != `{"a":1}` {
		t.Fatalf("contents = %q", payload)
	}
}

// TestWriteFileFailsWhenTheDirectoryDoesNotExist pins the documented contract
// that this does not create the parent - the caller owns the mode it gets.
func TestWriteFileFailsWhenTheDirectoryDoesNotExist(t *testing.T) {
	t.Parallel()

	err := WriteFile(filepath.Join(t.TempDir(), "missing", "artifact.json"), []byte("{}"))
	if err == nil {
		t.Fatal("expected an error writing into a directory that does not exist")
	}
}
