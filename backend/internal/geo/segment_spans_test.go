package geo

import (
	"math"
	"testing"
)

// squareWithHole is the polygon [0,10]² with a [3,7]² hole, the same shape
// TestPointInPolygonWithHole uses, so a span answer can be read against a
// point answer already pinned.
func squareWithHole() [][]Point2D {
	return [][]Point2D{
		{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}},
		{{3, 3}, {7, 3}, {7, 7}, {3, 7}, {3, 3}},
	}
}

func TestSegmentPolygonSpans(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		a, b     Point2D
		polygons [][][]Point2D
		want     []SegmentSpan
	}{
		{
			name:     "no polygons leaves one uncovered span",
			a:        Point2D{X: 0, Y: 0},
			b:        Point2D{X: 10, Y: 0},
			polygons: nil,
			want:     []SegmentSpan{{Start: 0, End: 1, Polygon: -1}},
		},
		{
			name:     "fully inside is one covered span",
			a:        Point2D{X: 1, Y: 1},
			b:        Point2D{X: 2, Y: 2},
			polygons: [][][]Point2D{squareWithHole()},
			want:     []SegmentSpan{{Start: 0, End: 1, Polygon: 0}},
		},
		{
			name:     "fully outside is one uncovered span",
			a:        Point2D{X: 20, Y: 20},
			b:        Point2D{X: 30, Y: 30},
			polygons: [][][]Point2D{squareWithHole()},
			want:     []SegmentSpan{{Start: 0, End: 1, Polygon: -1}},
		},
		{
			name:     "crossing one edge splits in two",
			a:        Point2D{X: -10, Y: 1},
			b:        Point2D{X: 10, Y: 1},
			polygons: [][][]Point2D{squareWithHole()},
			want: []SegmentSpan{
				{Start: 0, End: 0.5, Polygon: -1},
				{Start: 0.5, End: 1, Polygon: 0},
			},
		},
		{
			name:     "crossing the hole splits in three",
			a:        Point2D{X: 0, Y: 5},
			b:        Point2D{X: 10, Y: 5},
			polygons: [][][]Point2D{squareWithHole()},
			want: []SegmentSpan{
				{Start: 0, End: 0.3, Polygon: 0},
				{Start: 0.3, End: 0.7, Polygon: -1},
				{Start: 0.7, End: 1, Polygon: 0},
			},
		},
		{
			name: "the first polygon wins where two overlap",
			a:    Point2D{X: 0, Y: 1},
			b:    Point2D{X: 10, Y: 1},
			polygons: [][][]Point2D{
				{{{4, 0}, {10, 0}, {10, 2}, {4, 2}, {4, 0}}},
				{{{0, 0}, {6, 0}, {6, 2}, {0, 2}, {0, 0}}},
			},
			want: []SegmentSpan{
				{Start: 0, End: 0.4, Polygon: 1},
				{Start: 0.4, End: 1, Polygon: 0},
			},
		},
		{
			name:     "a degenerate segment is classified at its point",
			a:        Point2D{X: 1, Y: 1},
			b:        Point2D{X: 1, Y: 1},
			polygons: [][][]Point2D{squareWithHole()},
			want:     []SegmentSpan{{Start: 0, End: 1, Polygon: 0}},
		},
		{
			name:     "an unclosed ring is cut on its closing edge",
			a:        Point2D{X: -1, Y: 1},
			b:        Point2D{X: 1, Y: 1},
			polygons: [][][]Point2D{{{{0, 0}, {4, 0}, {4, 4}, {0, 4}}}},
			want: []SegmentSpan{
				{Start: 0, End: 0.5, Polygon: -1},
				{Start: 0.5, End: 1, Polygon: 0},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := SegmentPolygonSpans(tc.a, tc.b, tc.polygons)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d spans %v, want %d %v", len(got), got, len(tc.want), tc.want)
			}

			for i, want := range tc.want {
				if got[i].Polygon != want.Polygon ||
					math.Abs(got[i].Start-want.Start) > 1e-9 ||
					math.Abs(got[i].End-want.End) > 1e-9 {
					t.Fatalf("span[%d] = %+v, want %+v", i, got[i], want)
				}
			}
		})
	}
}

// TestSegmentPolygonSpansCoverTheWholeSegment pins the contract the ground-zone
// weighting depends on: the spans partition [0,1] with no gap and no overlap,
// so a length-weighted mean over them needs no normalisation beyond the
// segment's own length.
func TestSegmentPolygonSpansCoverTheWholeSegment(t *testing.T) {
	t.Parallel()

	polygons := [][][]Point2D{
		squareWithHole(),
		{{{8, 0}, {20, 0}, {20, 3}, {8, 3}, {8, 0}}},
	}

	spans := SegmentPolygonSpans(Point2D{X: -5, Y: 1}, Point2D{X: 25, Y: 1}, polygons)

	if spans[0].Start != 0 {
		t.Fatalf("first span starts at %v, want 0", spans[0].Start)
	}

	if spans[len(spans)-1].End != 1 {
		t.Fatalf("last span ends at %v, want 1", spans[len(spans)-1].End)
	}

	total := 0.0

	for i, span := range spans {
		if span.Length() <= 0 {
			t.Fatalf("span[%d] = %+v has no length", i, span)
		}

		if i > 0 && span.Start != spans[i-1].End {
			t.Fatalf("span[%d] starts at %v, but span[%d] ended at %v", i, span.Start, i-1, spans[i-1].End)
		}

		total += span.Length()
	}

	if math.Abs(total-1) > 1e-12 {
		t.Fatalf("spans cover %v of the segment, want 1", total)
	}
}

// TestSegmentPolygonSpansTouchingAVertex covers the case that motivated
// deduplicating the cut parameters: a ray leaving through a corner is reported
// by both edges meeting there, and two cuts at the same t would otherwise emit
// a zero-length span between them.
func TestSegmentPolygonSpansTouchingAVertex(t *testing.T) {
	t.Parallel()

	polygons := [][][]Point2D{{{{0, 0}, {4, 0}, {4, 4}, {0, 4}, {0, 0}}}}

	spans := SegmentPolygonSpans(Point2D{X: -4, Y: 8}, Point2D{X: 8, Y: -4}, polygons)

	for i, span := range spans {
		if span.Length() <= spanParameterEpsilon {
			t.Fatalf("span[%d] = %+v is degenerate", i, span)
		}
	}
}
