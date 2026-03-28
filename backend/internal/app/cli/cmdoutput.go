package cli

import (
	"encoding/json"
	"io"
)

// writeCommandOutput writes the payload as a single JSON object to w when
// jsonEnabled is true. When false it is a no-op — the caller is expected to
// fall through to the existing fmt.Fprintf calls.
func writeCommandOutput(w io.Writer, jsonEnabled bool, payload any) error {
	if !jsonEnabled {
		return nil
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(payload)
}
