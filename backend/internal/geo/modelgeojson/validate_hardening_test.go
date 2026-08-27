package modelgeojson

import (
	"strings"
	"testing"
	"time"
)

// zigzagLine builds a linestring with n vertices that has no self-intersection,
// so the check runs to completion instead of stopping at the first hit.
func zigzagLine(n int) []any {
	pts := make([]any, 0, n)
	for i := range n {
		pts = append(pts, []any{float64(i), float64(i % 2)})
	}

	return pts
}

func modelWithLine(pts []any) Model {
	return Model{
		SchemaVersion: 1,
		ProjectCRS:    "EPSG:25832",
		ImportedAt:    time.Now().UTC(),
		Features: []Feature{{
			ID:           "line-1",
			Kind:         FeatureKindSource,
			SourceType:   SourceTypeLine,
			GeometryType: GeometryTypeLineString,
			Coordinates:  pts,
		}},
	}
}

func hasIssue(issues []ValidationIssue, codeSuffix string) bool {
	for _, issue := range issues {
		if strings.HasSuffix(issue.Code, codeSuffix) {
			return true
		}
	}

	return false
}

// The self-intersection test compares every pair of segments. Without a bound
// a few hundred kilobytes of GeoJSON turns into 10^10 segment comparisons, so
// oversized geometries are reported as unchecked rather than walked.
func TestValidate_SelfIntersectionCheckIsBounded(t *testing.T) {
	model := modelWithLine(zigzagLine(maxSelfIntersectionPoints + 1))

	start := time.Now()
	report := Validate(model)
	elapsed := time.Since(start)

	if !hasIssue(report.Warnings, ".skipped") {
		t.Fatalf("expected a 'skipped' warning, got warnings %+v", report.Warnings)
	}

	if !report.Valid {
		t.Fatalf("oversized geometry must not be rejected outright: %+v", report.Errors)
	}

	// A quadratic walk over this many points takes minutes, not milliseconds.
	if elapsed > 5*time.Second {
		t.Fatalf("validation took %s; the self-intersection bound did not trip", elapsed)
	}
}

// Geometries below the bound must still be checked exactly as before.
func TestValidate_SelfIntersectionStillDetected(t *testing.T) {
	model := modelWithLine([]any{
		[]any{0.0, 0.0},
		[]any{10.0, 10.0},
		[]any{0.0, 10.0},
		[]any{10.0, 0.0},
	})

	report := Validate(model)

	if !hasIssue(report.Errors, "self_intersection") {
		t.Fatalf("expected a self-intersection error, got %+v", report.Errors)
	}
}

func TestValidate_LineJustBelowBoundIsChecked(t *testing.T) {
	model := modelWithLine(zigzagLine(maxSelfIntersectionPoints))

	report := Validate(model)

	if hasIssue(report.Warnings, ".skipped") {
		t.Fatal("a geometry at the limit must still be checked")
	}
}

func FuzzNormalize(f *testing.F) {
	f.Add([]byte(`{"type":"FeatureCollection","features":[]}`))
	f.Add([]byte(`{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"id":"a","kind":"building","height_m":3},"geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,0]]]}}]}`))
	f.Add([]byte(`{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"id":"s","kind":"source","source_type":"line"},"geometry":{"type":"LineString","coordinates":[[0,0],[1,1]]}}]}`))
	f.Add([]byte(`{"type":"FeatureCollection","features":[{"geometry":{"type":"MultiPolygon","coordinates":[[[[0,0]]]]}}]}`))
	f.Add([]byte(`{`))

	f.Fuzz(func(_ *testing.T, data []byte) {
		model, err := Normalize(data, "EPSG:25832", "fuzz.geojson")
		if err != nil {
			return
		}

		_ = Validate(model)
	})
}
