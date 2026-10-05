package cli

import (
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/soundplanimport"
)

func TestCalcAreaFromImportReportCopiesPoints(t *testing.T) {
	t.Parallel()

	input := &soundPlanImportCalcArea{Points: []soundPlanPoint{{X: 1, Y: 2, Z: 3}}}
	output := calcAreaFromImportReport(input)

	if output == nil || len(output.Points) != 1 {
		t.Fatalf("unexpected output len = %d", len(output.Points))
	}

	if output.Points[0] != (soundplanimport.Point3D{X: 1, Y: 2, Z: 3}) {
		t.Fatalf("point mismatch: %+v", output.Points[0])
	}

	output.Points[0].X = 99
	if input.Points[0].X == 99 {
		t.Fatal("calc area output must not alias input points")
	}
}

// calcAreaModel builds a model carrying one calc-area feature with the given
// outer ring and properties, in the shape modelgeojson.Normalize produces.
func calcAreaModel(ring []any, properties map[string]any) modelgeojson.Model {
	return modelgeojson.Model{Features: []modelgeojson.Feature{{
		ID:           "calc-area-1",
		Kind:         modelgeojson.FeatureKindCalcArea,
		Properties:   properties,
		GeometryType: modelgeojson.GeometryTypePolygon,
		Coordinates:  []any{ring},
	}}}
}

func closedSquareRing() []any {
	return []any{
		[]any{0.0, 0.0},
		[]any{10.0, 0.0},
		[]any{10.0, 10.0},
		[]any{0.0, 10.0},
		[]any{0.0, 0.0},
	}
}

func TestCalcAreaFromModelReadsClosedRing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		properties map[string]any
		wantZ      float64
	}{
		{"with base elevation", map[string]any{"soundplan_base_elevation_m": 117.5}, 117.5},
		// An area that carries no base elevation must read as "elevation 0",
		// never as "no calculation area". The property is absent whenever none
		// was recorded — a drawn area, or an import whose source had none.
		{"without base elevation", map[string]any{}, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			area, featureID, warnings := calcAreaFromModel(calcAreaModel(closedSquareRing(), tc.properties))
			if area == nil {
				t.Fatal("expected a calculation area")
			}

			if featureID != "calc-area-1" {
				t.Fatalf("feature id = %q, want calc-area-1", featureID)
			}

			if len(warnings) != 0 {
				t.Fatalf("unexpected warnings: %v", warnings)
			}

			if len(area.Points) != 5 {
				t.Fatalf("point count = %d, want 5 (the closing vertex is kept)", len(area.Points))
			}

			if area.Points[0] != (soundplanimport.Point3D{X: 0, Y: 0, Z: tc.wantZ}) {
				t.Fatalf("first point = %+v, want z = %v", area.Points[0], tc.wantZ)
			}

			if area.Points[4] != area.Points[0] {
				t.Fatalf("ring is not closed: first=%+v last=%+v", area.Points[0], area.Points[4])
			}
		})
	}
}

func TestCalcAreaFromModelWarnsOnInteriorRings(t *testing.T) {
	t.Parallel()

	hole := []any{
		[]any{2.0, 2.0},
		[]any{4.0, 2.0},
		[]any{4.0, 4.0},
		[]any{2.0, 4.0},
		[]any{2.0, 2.0},
	}

	model := modelgeojson.Model{Features: []modelgeojson.Feature{{
		ID:           "calc-area-1",
		Kind:         modelgeojson.FeatureKindCalcArea,
		GeometryType: modelgeojson.GeometryTypePolygon,
		Coordinates:  []any{closedSquareRing(), hole},
	}}}

	area, _, warnings := calcAreaFromModel(model)
	if area == nil {
		t.Fatal("expected the outer ring to still produce an area")
	}

	if len(area.Points) != 5 {
		t.Fatalf("point count = %d, want the 5 outer-ring points only", len(area.Points))
	}

	if len(warnings) != 1 || !strings.Contains(warnings[0], "interior ring") {
		t.Fatalf("warnings = %v, want one naming the interior ring", warnings)
	}
}

func TestCalcAreaFromModelReturnsNilWithoutCalcArea(t *testing.T) {
	t.Parallel()

	model := modelgeojson.Model{Features: []modelgeojson.Feature{
		{ID: "r0", Kind: modelgeojson.FeatureKindReceiver, HeightM: float64Ptr(4), GeometryType: modelgeojson.GeometryTypePoint, Coordinates: []any{0, 0}},
	}}

	area, featureID, warnings := calcAreaFromModel(model)
	if area != nil {
		t.Fatalf("expected nil area, got %+v", area)
	}

	if featureID != "" || warnings != nil {
		t.Fatalf("expected a silent nil fallback signal, got id=%q warnings=%v", featureID, warnings)
	}
}

// TestCalcAreaHorizontalSpanNeedsAClosedRing pins the precondition both sources
// of a calculation area now have to meet. calcAreaHorizontalSpan's edge loop
// runs to len(Points)-1 and never emits the closing edge, so it is correct for a
// closed ring and drops one edge for an open one. The model's ring gets closure
// from validation; the import report's point list is verbatim ParseCalcAreaFile
// output, so calcAreaFromImportReport closes it — see
// TestCalcAreaFromImportReportClosesTheRing.
//
// The same quadrilateral is fed in both spellings. The open one loses its left
// edge, finds a single crossing, and reports no span at all; the closed one
// answers correctly.
func TestCalcAreaHorizontalSpanNeedsAClosedRing(t *testing.T) {
	t.Parallel()

	open := &soundplanimport.CalcArea{Points: []soundplanimport.Point3D{
		{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10},
	}}
	closed := &soundplanimport.CalcArea{Points: []soundplanimport.Point3D{
		{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}, {X: 0, Y: 0},
	}}

	for _, y := range []float64{1, 5, 9} {
		if _, _, ok := calcAreaHorizontalSpan(open, y); ok {
			t.Fatalf("open ring at y=%v: expected no span, the closing edge is never emitted", y)
		}

		left, right, ok := calcAreaHorizontalSpan(closed, y)
		if !ok || left != 0 || right != 10 {
			t.Fatalf("closed ring at y=%v: span=(%v,%v) ok=%v, want (0,10) ok=true", y, left, right, ok)
		}
	}
}

// TestCalcAreaHorizontalSpanOpenNotchIsSilentlyWrong is the failure mode that
// makes the missing closing edge worse than a loud one. A rectangle degrades
// loudly — every row reports no span and falls back to the bounding box with a
// warning. From six vertices up, the dropped edge shifts the even/odd pairing
// instead of removing a crossing, and the scanline returns a plausible span that
// is wrong: here it returns the notch gap, so receivers land in exactly the
// region the drawing excludes.
func TestCalcAreaHorizontalSpanOpenNotchIsSilentlyWrong(t *testing.T) {
	t.Parallel()

	notch := []soundplanimport.Point3D{
		{X: 0, Y: 0},
		{X: 10, Y: 0},
		{X: 10, Y: 10},
		{X: 6, Y: 10},
		{X: 6, Y: 4},
		{X: 4, Y: 4},
		{X: 4, Y: 10},
		{X: 0, Y: 10},
	}

	open := &soundplanimport.CalcArea{Points: notch}
	closed := &soundplanimport.CalcArea{Points: append(append([]soundplanimport.Point3D(nil), notch...), soundplanimport.Point3D{X: 0, Y: 0})}

	left, right, ok := calcAreaHorizontalSpan(open, 7)
	if !ok || left != 4 || right != 6 {
		t.Fatalf("open notch at y=7: span=(%v,%v) ok=%v, want the wrong-but-plausible (4,6)", left, right, ok)
	}

	left, right, ok = calcAreaHorizontalSpan(closed, 7)
	if !ok || left != 0 || right != 4 {
		t.Fatalf("closed notch at y=7: span=(%v,%v) ok=%v, want the real lobe (0,4)", left, right, ok)
	}
}

// TestCalcAreaFromImportReportClosesTheRing covers the fallback's precondition.
// Nothing upstream of the import report closes CalcArea.geo, and
// calcAreaHorizontalSpan needs a closed ring, so the conversion closes it. The
// notch case is the one that matters: left open it returns the notch gap as the
// row span and puts receivers inside the region the drawing excludes.
func TestCalcAreaFromImportReportClosesTheRing(t *testing.T) {
	t.Parallel()

	openRectangle := &soundPlanImportCalcArea{Points: []soundPlanPoint{
		{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10},
	}}

	closed := calcAreaFromImportReport(openRectangle)
	if len(closed.Points) != 5 {
		t.Fatalf("point count = %d, want 5 (four vertices plus the closing repeat)", len(closed.Points))
	}

	if closed.Points[4] != closed.Points[0] {
		t.Fatalf("last point %+v does not repeat the first %+v", closed.Points[4], closed.Points[0])
	}

	for _, y := range []float64{1, 5, 9} {
		left, right, ok := calcAreaHorizontalSpan(closed, y)
		if !ok || left != 0 || right != 10 {
			t.Fatalf("y=%v: span=(%v,%v) ok=%v, want (0,10) ok=true", y, left, right, ok)
		}
	}

	// An already-closed list is left alone rather than gaining a second repeat,
	// which would add a zero-length edge.
	alreadyClosed := calcAreaFromImportReport(&soundPlanImportCalcArea{Points: []soundPlanPoint{
		{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}, {X: 0, Y: 0},
	}})
	if len(alreadyClosed.Points) != 5 {
		t.Fatalf("closed input point count = %d, want 5", len(alreadyClosed.Points))
	}

	// Closure is decided in 2D: CalcArea.geo's z varies along the outline, so a
	// z difference at the repeated vertex must not count as "open".
	closedInPlan := calcAreaFromImportReport(&soundPlanImportCalcArea{Points: []soundPlanPoint{
		{X: 0, Y: 0, Z: 100}, {X: 10, Y: 0, Z: 101}, {X: 10, Y: 10, Z: 102}, {X: 0, Y: 10, Z: 103}, {X: 0, Y: 0, Z: 104},
	}})
	if len(closedInPlan.Points) != 5 {
		t.Fatalf("plan-closed input point count = %d, want 5", len(closedInPlan.Points))
	}

	openNotch := calcAreaFromImportReport(&soundPlanImportCalcArea{Points: []soundPlanPoint{
		{X: 0, Y: 0},
		{X: 10, Y: 0},
		{X: 10, Y: 10},
		{X: 6, Y: 10},
		{X: 6, Y: 4},
		{X: 4, Y: 4},
		{X: 4, Y: 10},
		{X: 0, Y: 10},
	}})

	left, right, ok := calcAreaHorizontalSpan(openNotch, 7)
	if !ok || left != 0 || right != 4 {
		t.Fatalf("notch at y=7: span=(%v,%v) ok=%v, want the real lobe (0,4), not the notch gap (4,6)", left, right, ok)
	}
}

// TestCalcAreaBoundsDeltaUnitFollowsProjectCRS pins that the recorded envelope
// delta is never called metres on a CRS whose axes are not metres. Under the
// CLI's default EPSG:4326 it is degrees, where 0.001 is roughly 111 m.
func TestCalcAreaBoundsDeltaUnitFollowsProjectCRS(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"EPSG:25832": calcAreaDeltaUnitMetre,
		"EPSG:31467": calcAreaDeltaUnitMetre,
		"EPSG:4326":  calcAreaDeltaUnitDegree,
		"EPSG:4258":  calcAreaDeltaUnitDegree,
		"EPSG:1":     calcAreaDeltaUnitUnknown,
		"WKT:custom": calcAreaDeltaUnitUnknown,
		"":           calcAreaDeltaUnitUnknown,
		"nonsense":   calcAreaDeltaUnitUnknown,
	}

	for crs, want := range cases {
		if got := calcAreaBoundsDeltaUnit(crs); got != want {
			t.Errorf("calcAreaBoundsDeltaUnit(%q) = %q, want %q", crs, got, want)
		}
	}
}
