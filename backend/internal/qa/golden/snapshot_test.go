package golden

import "testing"

func TestAssertJSONSnapshot(t *testing.T) {
	t.Parallel()

	got := map[string]any{
		"command":     "aconiq run",
		"duration_ms": 123,
		"status":      "ok",
	}

	AssertJSONSnapshot(t, "testdata/run-summary.golden.json", got)
}
