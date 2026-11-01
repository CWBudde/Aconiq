package modelgeojson

import (
	"strings"
	"testing"
)

// parkingSource is an RLS-19 parking area someone could have traced from an
// orthophoto, with the properties a test adds.
func parkingSource(props map[string]any) Feature {
	return Feature{
		ID:           "p1",
		Kind:         FeatureKindSource,
		SourceType:   SourceTypeArea,
		GeometryType: GeometryTypePolygon,
		Coordinates: []any{[]any{
			[]any{500000.0, 5800000.0},
			[]any{500020.0, 5800000.0},
			[]any{500020.0, 5800010.0},
			[]any{500000.0, 5800000.0},
		}},
		Properties: props,
	}
}

func orthophotoEvidence() map[string]any {
	return map[string]any{
		"kind":           "orthophoto",
		"source_id":      "https://opendata.lgln.niedersachsen.de/doorman/noauth/dop_wms?REQUEST=GetMap",
		"content_sha256": strings.Repeat("ab", 32),
		"retrieved_at":   "2026-11-01T12:00:00Z",
		"licence":        "CC-BY-4.0",
	}
}

// A source mapped from imagery carries the review fields a window does, in
// the same shapes; a source without them was taken from plans or an import
// and stays valid as it always was.
func TestSourceReviewFieldsValidate(t *testing.T) {
	t.Parallel()

	for _, props := range []map[string]any{
		nil,
		{PropertyReviewStatus: "proposed", PropertyEvidence: []any{orthophotoEvidence()}},
		{PropertyReviewStatus: "confirmed"},
		{PropertyReviewStatus: "rejected", PropertyEvidence: []any{}},
	} {
		assertCleanReport(t, Validate(facadeModel(parkingSource(props))))
	}
}

func TestSourceReviewFieldsAreRefusedMalformed(t *testing.T) {
	t.Parallel()

	for name, props := range map[string]map[string]any{
		"unknown status":   {PropertyReviewStatus: "maybe"},
		"status not text":  {PropertyReviewStatus: true},
		"evidence object":  {PropertyEvidence: orthophotoEvidence()},
		"evidence entries": {PropertyEvidence: []any{"orthophoto"}},
		"evidence hash":    {PropertyEvidence: []any{map[string]any{"kind": "orthophoto", "content_sha256": "ABC"}}},
	} {
		report := Validate(facadeModel(parkingSource(props)))

		found := issuesWithCode(report.Errors, CodeSourceReviewInvalid)
		if report.Valid || len(found) != 1 || found[0].FeatureID != "p1" {
			t.Errorf("%s: want one %s error on p1, got valid=%v errors=%#v", name, CodeSourceReviewInvalid, report.Valid, report.Errors)
		}
	}
}
