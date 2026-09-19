package jsonio_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/aconiq/backend/internal/jsonio"
)

// The writers this package was extracted from all produced
// `json.MarshalIndent(v, "", "  ")` plus a trailing newline, and their output is
// held byte-identical by golden files and by the run digests. Pin the exact
// bytes here rather than the intent, so a later "tidy up the encoder" cannot
// reach those files without this test going red first.
func TestMarshalIsTwoSpaceIndentWithATrailingNewline(t *testing.T) {
	t.Parallel()

	value := map[string]any{
		"name":  "rls19-road",
		"level": 55.5,
		"bands": []int{63, 125, 250},
	}

	got, err := jsonio.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	want := "{\n  \"bands\": [\n    63,\n    125,\n    250\n  ],\n  \"level\": 55.5,\n  \"name\": \"rls19-road\"\n}\n"
	if string(got) != want {
		t.Fatalf("Marshal =\n%q\nwant\n%q", got, want)
	}
}

// The trailing newline is the one byte that is not `encoding/json`'s, so assert
// it on its own: a value whose encoding is a bare scalar has no other structure
// to hide a missing newline behind.
func TestMarshalAppendsExactlyOneTrailingNewline(t *testing.T) {
	t.Parallel()

	got, err := jsonio.Marshal(42)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if string(got) != "42\n" {
		t.Fatalf("Marshal = %q, want %q", got, "42\n")
	}
}

// `json.MarshalIndent` escapes <, > and & into their \u003c, \u003e and
// \u0026 forms, and so does a `json.Encoder` with `SetIndent` — until someone
// calls `SetEscapeHTML(false)`. Files already on disk carry the escaped form,
// so pin it: a different implementation must not quietly rewrite an `&`.
func TestMarshalEscapesHTMLTheWayMarshalIndentDoes(t *testing.T) {
	t.Parallel()

	got, err := jsonio.Marshal(map[string]string{"note": "a<b & c>d"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	want := "{\n  \"note\": \"a\\u003cb \\u0026 c\\u003ed\"\n}\n"
	if string(got) != want {
		t.Fatalf("Marshal = %q, want %q", got, want)
	}
}

// The encoder normalizes nothing: a nil slice stays `null`, as it was in every
// file those writers wrote. Callers that want `[]` allocate an empty slice, as
// they did before.
func TestMarshalDoesNotNormalizeNilSlices(t *testing.T) {
	t.Parallel()

	var nilSlice []string

	got, err := jsonio.Marshal(struct {
		Items []string `json:"items"`
	}{Items: nilSlice})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	want := "{\n  \"items\": null\n}\n"
	if string(got) != want {
		t.Fatalf("Marshal = %q, want %q", got, want)
	}
}

// Every caller wraps the failure in its own taxonomy — `domainerrors.New` with
// its own op string in `app/cli` and `io/projectfs`, `fmt.Errorf` elsewhere —
// so the encoder must hand back a plain error and no half-written bytes.
func TestMarshalReturnsNoBytesWhenEncodingFails(t *testing.T) {
	t.Parallel()

	got, err := jsonio.Marshal(make(chan int))
	if err == nil {
		t.Fatalf("Marshal(chan) = %q, want an error", got)
	}

	if got != nil {
		t.Fatalf("Marshal(chan) returned %q, want nil bytes", got)
	}

	var unsupported *json.UnsupportedTypeError
	if !errors.As(err, &unsupported) {
		t.Fatalf("Marshal(chan) error = %v, want it to wrap *json.UnsupportedTypeError", err)
	}
}

// Agreement with the standard library, spelled out: whatever a caller passes,
// Marshal's output is MarshalIndent's plus one newline and nothing else.
func TestMarshalAgreesWithMarshalIndentPlusNewline(t *testing.T) {
	t.Parallel()

	values := []any{
		nil,
		"",
		0.0,
		[]any{},
		map[string]any{},
		map[string]any{"a": []any{1.0, "two", nil}, "b": map[string]any{"c": true}},
	}

	for _, value := range values {
		reference, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatalf("MarshalIndent(%v): %v", value, err)
		}

		got, err := jsonio.Marshal(value)
		if err != nil {
			t.Fatalf("Marshal(%v): %v", value, err)
		}

		if string(got) != string(reference)+"\n" {
			t.Fatalf("Marshal(%v) = %q, want %q", value, got, string(reference)+"\n")
		}
	}
}
