package golden

import (
	"runtime"
	"testing"
)

func TestAssertJSONSnapshot(t *testing.T) {
	t.Parallel()

	got := map[string]any{
		"command":     "aconiq run",
		"duration_ms": 123,
		"status":      "ok",
	}

	AssertJSONSnapshot(t, "testdata/run-summary.golden.json", got)
}

func TestAssertBytesSnapshot(t *testing.T) {
	t.Parallel()

	// Deliberately whitespace-heavy: the trailing newline and the embedded CR
	// are the byte-level differences a %s-rendered mismatch would hide.
	//
	// A lone CR rather than a CRLF on purpose. Git normalizes CRLF to LF in a
	// text file on commit, which would rewrite the golden out from under this
	// test on the next clone; a CR not followed by LF is left alone.
	got := []byte("id,x\rR1,1\n")

	AssertBytesSnapshot(t, "testdata/bytes.golden.txt", got)
}

// Not parallel: the switch is process-wide environment state.
func TestSkipAMD64BitsOnlyOffAMD64AndOnlyWhenAsked(t *testing.T) {
	t.Setenv(SkipAMD64BitsEnv, "")

	if SkipAMD64Bits(t) {
		t.Fatal("skipped an amd64-bit golden without being asked")
	}

	t.Setenv(SkipAMD64BitsEnv, "1")

	if got, want := SkipAMD64Bits(t), runtime.GOARCH != "amd64"; got != want {
		t.Fatalf("SkipAMD64Bits() = %v on %s with %s=1, want %v: only a machine whose bits the golden does not hold may skip it",
			got, runtime.GOARCH, SkipAMD64BitsEnv, want)
	}
}
