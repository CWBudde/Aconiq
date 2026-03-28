package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWriteCommandOutputJSON(t *testing.T) {
	t.Parallel()

	type testPayload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	var buf bytes.Buffer

	err := writeCommandOutput(&buf, true, testPayload{Name: "test", Count: 42})
	if err != nil {
		t.Fatalf("writeCommandOutput: %v", err)
	}

	var got testPayload

	err = json.Unmarshal(buf.Bytes(), &got)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Name != "test" || got.Count != 42 {
		t.Fatalf("got %+v, want {test 42}", got)
	}
}

func TestWriteCommandOutputNoOpWhenDisabled(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := writeCommandOutput(&buf, false, map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("writeCommandOutput: %v", err)
	}

	if buf.Len() != 0 {
		t.Fatalf("expected no output when JSON disabled, got %q", buf.String())
	}
}
