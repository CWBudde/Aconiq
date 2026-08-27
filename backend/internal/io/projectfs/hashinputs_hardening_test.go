package projectfs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
)

// Input files are untrusted third-party data of unbounded size (a national DTM
// is routinely gigabytes), so provenance hashing must stream them rather than
// read them into memory whole.
func TestHashInputs_DoesNotBufferWholeFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "big.bin")

	const size = 64 << 20 // 64 MiB

	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i)
	}

	err := os.WriteFile(path, content, 0o600)
	if err != nil {
		t.Fatalf("write input file: %v", err)
	}

	var before, after runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&before)

	store := Store{root: root}

	hashes, err := store.hashInputs([]string{"big.bin"})
	if err != nil {
		t.Fatalf("hashInputs: %v", err)
	}

	runtime.ReadMemStats(&after)

	want := sha256.Sum256(content)
	if hashes["big.bin"] != hex.EncodeToString(want[:]) {
		t.Fatalf("hash = %s, want %s", hashes["big.bin"], hex.EncodeToString(want[:]))
	}

	allocated := after.TotalAlloc - before.TotalAlloc
	if allocated > size/2 {
		t.Fatalf("hashing allocated %d bytes for a %d byte file; it is not streaming", allocated, size)
	}
}

func TestHashInputs_DeduplicatesRepeatedPaths(t *testing.T) {
	root := t.TempDir()

	err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o600)
	if err != nil {
		t.Fatalf("write input file: %v", err)
	}

	store := Store{root: root}

	hashes, err := store.hashInputs([]string{"a.txt", "a.txt", " a.txt ", ""})
	if err != nil {
		t.Fatalf("hashInputs: %v", err)
	}

	if len(hashes) != 1 {
		t.Fatalf("hashes = %v, want a single entry", hashes)
	}
}

func TestHashInputs_MissingFileIsUserInput(t *testing.T) {
	store := Store{root: t.TempDir()}

	_, err := store.hashInputs([]string{"absent.bin"})
	if err == nil {
		t.Fatal("expected an error for a missing input file")
	}

	var appErr *domainerrors.AppError
	if !errors.As(err, &appErr) || appErr.Kind != domainerrors.KindUserInput {
		t.Fatalf("error %v is not a user-input domain error", err)
	}
}
