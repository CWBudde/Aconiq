package golden

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/jsonio"
)

// UpdateEnabled reports whether tests should rewrite golden snapshots.
func UpdateEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("UPDATE_GOLDEN")))
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// SkipAMD64BitsEnv is the switch `just test-arm64` sets; see SkipAMD64Bits.
const SkipAMD64BitsEnv = "ACONIQ_SKIP_AMD64_GOLDENS"

// SkipAMD64Bits reports whether the caller is to leave out a comparison
// against a golden that holds amd64's floating-point bits, and logs the
// omission when it does.
//
// Go contracts multiply-adds into one fused instruction on arm64, and the
// standard library's math assembly differs there, so such a golden cannot
// hold on that machine. A test that compares against one skips that
// comparison alone and keeps every other assertion: two identical runs must
// still agree, a run must still complete. The switch is honoured only off
// amd64, so the jobs that check these goldens can never skip them, whatever
// their environment says.
func SkipAMD64Bits(t *testing.T) bool {
	t.Helper()

	if runtime.GOARCH == "amd64" || os.Getenv(SkipAMD64BitsEnv) != "1" {
		return false
	}

	t.Logf("not compared against the golden, which holds amd64's bits (%s=1 on %s)", SkipAMD64BitsEnv, runtime.GOARCH)

	return true
}

// AssertJSONSnapshot compares a JSON-serializable value against a golden file.
// If UPDATE_GOLDEN is enabled, the file is rewritten instead.
func AssertJSONSnapshot(t *testing.T, snapshotPath string, got any) {
	t.Helper()

	serialized, err := jsonio.Marshal(got)
	if err != nil {
		t.Fatalf("marshal snapshot value: %v", err)
	}

	AssertBytesSnapshot(t, snapshotPath, serialized)
}

// AssertBytesSnapshot compares raw bytes against a golden file.
// If UPDATE_GOLDEN is enabled, the file is rewritten instead.
//
// Mismatches are printed with %q rather than %s on purpose. A golden that
// differs only in a trailing newline, a CR before an LF, or a stray BOM is
// invisible when rendered raw, and byte contracts are exactly where those
// differences matter.
func AssertBytesSnapshot(t *testing.T, snapshotPath string, got []byte) {
	t.Helper()

	if UpdateEnabled() {
		err := os.MkdirAll(filepath.Dir(snapshotPath), 0o750)
		if err != nil {
			t.Fatalf("create snapshot directory: %v", err)
		}

		err = os.WriteFile(snapshotPath, got, 0o600)
		if err != nil {
			t.Fatalf("write snapshot file: %v", err)
		}

		return
	}

	expected, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read snapshot file %q: %v\nHint: run with UPDATE_GOLDEN=1 to create it.", snapshotPath, err)
	}

	if !bytes.Equal(expected, got) {
		t.Fatalf("snapshot mismatch for %s\nexpected:\n%q\ngot:\n%q\nHint: run `just update-golden` if change is intentional.", snapshotPath, expected, got)
	}
}
