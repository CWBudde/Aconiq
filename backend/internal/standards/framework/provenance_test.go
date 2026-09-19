package framework

import (
	"maps"
	"testing"
)

func TestStampKeyParametersRecordsOnlyThePresentKeys(t *testing.T) {
	t.Parallel()

	metadata := map[string]string{"model_version": "v1"}
	params := map[string]string{
		"receiver_height_m": "4",
		"min_distance_m":    "",
		"not_declared":      "7",
	}

	got := StampKeyParameters(metadata, params, []string{
		"receiver_height_m",
		"min_distance_m",
		"never_supplied",
	})

	want := map[string]string{
		"model_version":                   "v1",
		"key_parameter.receiver_height_m": "4",
		// An empty value is a value: the parameter was supplied and is
		// recorded as supplied.
		"key_parameter.min_distance_m": "",
	}

	if !maps.Equal(got, want) {
		t.Fatalf("unexpected metadata: got %v want %v", got, want)
	}
}

func TestStampKeyParametersReturnsTheSameMapItWasGiven(t *testing.T) {
	t.Parallel()

	// Modules write `return framework.StampKeyParameters(metadata, ...)`, so
	// the base map they built has to be the map that comes back.
	metadata := map[string]string{}

	got := StampKeyParameters(metadata, map[string]string{"a": "1"}, []string{"a"})
	if len(metadata) != 1 || metadata["key_parameter.a"] != "1" {
		t.Fatalf("the base map was not stamped in place: %v", metadata)
	}

	got["b"] = "2"

	if metadata["b"] != "2" {
		t.Fatalf("expected the returned map to be the base map, got a copy")
	}
}

func TestStampKeyParametersToleratesAnEmptyKeyList(t *testing.T) {
	t.Parallel()

	metadata := map[string]string{"model_version": "v1"}

	got := StampKeyParameters(metadata, map[string]string{"a": "1"}, nil)
	if len(got) != 1 {
		t.Fatalf("expected the base map untouched, got %v", got)
	}
}
