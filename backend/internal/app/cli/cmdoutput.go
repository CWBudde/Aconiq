package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

// outputFieldWarnings is the field every command reports its warnings under,
// in both its JSON output and its structured log line. It is one name because
// a consumer reading two commands' output should not have to learn two.
const outputFieldWarnings = "warnings"

// writeCommandOutput writes the payload as a single JSON object to w when
// jsonEnabled is true. When false it is a no-op — the caller is expected to
// fall through to the existing fmt.Fprintf calls.
func writeCommandOutput(w io.Writer, jsonEnabled bool, payload any) error {
	if !jsonEnabled {
		return nil
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	if err := enc.Encode(payload); err != nil {
		return fmt.Errorf("encode command output: %w", err)
	}

	return nil
}
