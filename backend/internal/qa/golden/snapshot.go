package golden

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// AssertJSONSnapshot compares a JSON-serializable value against a golden file.
// If UPDATE_GOLDEN is enabled, the file is rewritten instead.
func AssertJSONSnapshot(t *testing.T, snapshotPath string, got any) {
	t.Helper()

	serialized, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal snapshot value: %v", err)
	}

	serialized = append(serialized, '\n')

	if UpdateEnabled() {
		err := os.MkdirAll(filepath.Dir(snapshotPath), 0o755)
		if err != nil {
			t.Fatalf("create snapshot directory: %v", err)
		}

		err = os.WriteFile(snapshotPath, serialized, 0o644)
		if err != nil {
			t.Fatalf("write snapshot file: %v", err)
		}

		return
	}

	expected, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read snapshot file %q: %v\nHint: run with UPDATE_GOLDEN=1 to create it.", snapshotPath, err)
	}

	if !bytes.Equal(expected, serialized) {
		t.Fatalf("snapshot mismatch for %s\nexpected:\n%s\ngot:\n%s\nHint: run `just update-golden` if change is intentional.", snapshotPath, expected, serialized)
	}
}
