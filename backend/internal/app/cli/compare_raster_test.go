package cli

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/soundplanimport"
	"github.com/aconiq/backend/internal/report/results"
	"github.com/aconiq/backend/internal/standards/schall03"
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
		// The property is gone after the first save from the map, which must
		// read as "elevation 0", never as "no calculation area".
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

// TestCalcAreaHorizontalSpanNeedsAClosedRing is why the model's calculation area
// wins rather than merely being preferred. calcAreaHorizontalSpan's edge loop
// runs to len(Points)-1 and never emits the closing edge, so it is correct for a
// closed ring and drops one edge for an open one — and the import report's point
// list is verbatim ParseCalcAreaFile output with nothing enforcing closure.
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

func TestReceiverHeightFromModel(t *testing.T) {
	t.Parallel()

	model := modelgeojson.Model{Features: []modelgeojson.Feature{
		{Kind: "receiver", HeightM: float64Ptr(3), ID: "r0", GeometryType: "Point", Coordinates: []any{0, 0}},
		{Kind: "receiver", HeightM: float64Ptr(5), ID: "r1", GeometryType: "Point", Coordinates: []any{0, 0}},
	}}

	if got := receiverHeightFromModel(model); got != 3 {
		t.Fatalf("receiver height = %f, want 3", got)
	}

	none := modelgeojson.Model{Features: []modelgeojson.Feature{{Kind: "building", HeightM: float64Ptr(10), ID: "b", GeometryType: "Polygon", Coordinates: []any{[]any{}}}}}
	if got := receiverHeightFromModel(none); got != 4.0 {
		t.Fatalf("fallback receiver height = %f, want 4.0", got)
	}
}

func TestBuildHeuristicRasterReceiversUsesAreaAndRows(t *testing.T) {
	t.Parallel()

	// The CalcArea is 6 m tall while the two rows span (2-1)*5 = 5 m, so both row
	// centres fall strictly inside the polygon and are intersected normally. See
	// TestBuildHeuristicRasterReceiversRowOnTopEdgeFallsBack for the degenerate
	// case where the row grid exactly fills the bounding box.
	area := &soundplanimport.CalcArea{Points: []soundplanimport.Point3D{
		{X: 0, Y: 0, Z: 0},
		{X: 10, Y: 0, Z: 0},
		{X: 10, Y: 6, Z: 0},
		{X: 0, Y: 6, Z: 0},
		{X: 0, Y: 0, Z: 0},
	}}

	rows := [][]soundplanimport.GridMapCell{
		{{}},
		{{}, {}},
	}

	receivers, ids, warnings := buildHeuristicRasterReceivers(area, 5, 4, rows)
	if warnings != nil {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	if len(receivers) != 3 {
		t.Fatalf("receiver count = %d, want 3", len(receivers))
	}

	if len(ids) != len(receivers) {
		t.Fatalf("id count mismatch: %d vs %d", len(ids), len(receivers))
	}

	expected := map[string]struct {
		x   float64
		y   float64
		row int
		col int
	}{
		"soundplan-raster-r001-c001": {x: 5, y: 5.5, row: 0, col: 0},
		"soundplan-raster-r002-c001": {x: 2.5, y: 0.5, row: 1, col: 0},
		"soundplan-raster-r002-c002": {x: 7.5, y: 0.5, row: 1, col: 1},
	}

	for _, receiver := range receivers {
		exp, ok := expected[receiver.ID]
		if !ok {
			t.Fatalf("unexpected receiver id %q", receiver.ID)
		}

		if receiver.Row != exp.row || receiver.Col != exp.col {
			t.Fatalf("%s row/col=%d,%d want %d,%d", receiver.ID, receiver.Row, receiver.Col, exp.row, exp.col)
		}

		if math.Abs(receiver.X-exp.x) > 1e-9 || math.Abs(receiver.Y-exp.y) > 1e-9 {
			t.Fatalf("%s position=(%.2f,%.2f), want (%.2f,%.2f)", receiver.ID, receiver.X, receiver.Y, exp.x, exp.y)
		}
	}
}

// TestBuildHeuristicRasterReceiversRowOnTopEdgeFallsBack pins current behaviour for
// the knife-edge case where the row grid exactly fills the CalcArea bounding box:
// heuristicRasterRowCenters then places row 0 at y == maxY, and the half-open
// scanline rule in calcAreaHorizontalSpan (y >= maxY rejects every edge) finds no
// intersections, so the row silently falls back to the bounding-box span.
//
// For a rectangle the fallback span equals the true span, so only the warning is
// observable. For a non-rectangular CalcArea the fallback would be wrong. Fixing
// the boundary rule is tracked in PLAN.md (Priority 13); this test exists so that
// the fix cannot land unnoticed.
func TestBuildHeuristicRasterReceiversRowOnTopEdgeFallsBack(t *testing.T) {
	t.Parallel()

	// Height 5 m, two rows at 5 m spacing => topCenter == maxY == 5.
	area := &soundplanimport.CalcArea{Points: []soundplanimport.Point3D{
		{X: 0, Y: 0, Z: 0},
		{X: 10, Y: 0, Z: 0},
		{X: 10, Y: 5, Z: 0},
		{X: 0, Y: 5, Z: 0},
		{X: 0, Y: 0, Z: 0},
	}}

	rows := [][]soundplanimport.GridMapCell{
		{{}},
		{{}, {}},
	}

	receivers, _, warnings := buildHeuristicRasterReceivers(area, 5, 4, rows)

	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one bounding-box fallback warning", warnings)
	}

	if !strings.Contains(warnings[0], "row 0 could not be intersected with CalcArea") {
		t.Fatalf("unexpected warning %q", warnings[0])
	}

	// The fallback still yields the geometrically correct centre here, because the
	// bounding box of a rectangle is the rectangle.
	for _, receiver := range receivers {
		if receiver.Row != 0 {
			continue
		}

		if math.Abs(receiver.X-5) > 1e-9 || math.Abs(receiver.Y-5) > 1e-9 {
			t.Fatalf("row 0 position=(%.2f,%.2f), want (5.00,5.00)", receiver.X, receiver.Y)
		}
	}
}

func TestBuildMetadataAlignedRasterReceiversUsesOriginSpacing(t *testing.T) {
	t.Parallel()

	area := &soundplanimport.CalcArea{Points: []soundplanimport.Point3D{
		{X: 100, Y: 190, Z: 0},
		{X: 160, Y: 190, Z: 0},
		{X: 160, Y: 220, Z: 0},
		{X: 100, Y: 220, Z: 0},
		{X: 100, Y: 190, Z: 0},
	}}

	meta := soundplanimport.GridMapMetadata{
		OriginX:          120,
		OriginY:          300,
		SpacingX:         10,
		SpacingY:         5,
		DeclaredRowCount: 2,
	}

	rows := [][]soundplanimport.GridMapCell{
		{{}, {}},
		{{}},
	}

	receivers, ids, warnings := buildMetadataAlignedRasterReceivers(meta, area, 4, rows)
	if len(receivers) != 3 {
		t.Fatalf("receiver count = %d, want 3", len(receivers))
	}

	if len(ids) != len(receivers) {
		t.Fatalf("id count mismatch: %d vs %d", len(ids), len(receivers))
	}

	if warnings != nil {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	expected := []struct {
		id  string
		x   float64
		y   float64
		row int
		col int
	}{
		{"soundplan-raster-r001-c001", 120, 300, 0, 0},
		{"soundplan-raster-r001-c002", 130, 300, 0, 1},
		{"soundplan-raster-r002-c001", 120, 305, 1, 0},
	}

	for i, receiver := range receivers {
		exp := expected[i]
		if receiver.ID != exp.id {
			t.Fatalf("%d id = %q, want %q", i, receiver.ID, exp.id)
		}

		if receiver.Row != exp.row || receiver.Col != exp.col {
			t.Fatalf("%s row/col = %d,%d, want %d,%d", receiver.ID, receiver.Row, receiver.Col, exp.row, exp.col)
		}

		if receiver.X != exp.x || receiver.Y != exp.y {
			t.Fatalf("%s xy=(%v,%v), want (%v,%v)", receiver.ID, receiver.X, receiver.Y, exp.x, exp.y)
		}
	}
}

func TestMetadataAlignedRasterReceiversPrefersAreaOverlap(t *testing.T) {
	t.Parallel()

	area := &soundplanimport.CalcArea{Points: []soundplanimport.Point3D{
		{X: 0, Y: 293, Z: 0},
		{X: 20, Y: 293, Z: 0},
		{X: 20, Y: 303, Z: 0},
		{X: 0, Y: 303, Z: 0},
		{X: 0, Y: 293, Z: 0},
	}}

	meta := soundplanimport.GridMapMetadata{
		OriginX:          100,
		OriginY:          300,
		SpacingX:         10,
		SpacingY:         10,
		DeclaredRowCount: 2,
	}

	rows := [][]soundplanimport.GridMapCell{
		{{}},
		{{}},
	}

	receivers, _, warnings := buildMetadataAlignedRasterReceivers(meta, area, 4, rows)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	if len(receivers) != 2 {
		t.Fatalf("receiver count = %d, want 2", len(receivers))
	}

	if receivers[0].Y != 300 {
		t.Fatalf("first row y = %.2f, want 300", receivers[0].Y)
	}

	if receivers[1].Y != 290 {
		t.Fatalf("second row y = %.2f, want 290", receivers[1].Y)
	}
}

func TestPrepareAndFinalizeSoundPlanRasterCompare(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()

	modelPath := filepath.Join(".noise", "model", "model.normalized.geojson")

	modelFile := filepath.Join(projectRoot, modelPath)
	if err := os.MkdirAll(filepath.Dir(modelFile), 0o750); err != nil {
		t.Fatalf("make model dir: %v", err)
	}

	featureCollection := map[string]any{
		"type": "FeatureCollection",
		"features": []any{map[string]any{
			"type":       "Feature",
			"properties": map[string]any{"id": "base", "kind": "receiver", "height_m": 4.0},
			"geometry":   map[string]any{"type": "Point", "coordinates": []any{0, 0}},
		}},
	}

	payload, err := json.Marshal(featureCollection)
	if err != nil {
		t.Fatalf("marshal model: %v", err)
	}

	if err := os.WriteFile(modelFile, payload, 0o600); err != nil {
		t.Fatalf("write model: %v", err)
	}

	soundPlanRoot := filepath.Join(projectRoot, "soundplan")

	gmDir := filepath.Join(soundPlanRoot, "RS01")
	if err := os.MkdirAll(gmDir, 0o750); err != nil {
		t.Fatalf("make gm dir: %v", err)
	}

	gmPath := filepath.Join(gmDir, "RRLK0010.GM")
	if err := writeTestGridMapFile(gmPath, []testGridCell{{ground: -1, day: 0, night: 0, flag: 1}, {ground: 110, day: 50, night: 40, flag: 1}}); err != nil {
		t.Fatalf("write gm: %v", err)
	}

	report := soundPlanImportReport{
		SourcePath:      filepath.Base(soundPlanRoot),
		ProjectCRS:      "EPSG:25832",
		GridResolutionM: 5,
		CalcArea:        &soundPlanImportCalcArea{Points: []soundPlanPoint{{X: 0, Y: 0, Z: 0}, {X: 5, Y: 0, Z: 0}, {X: 5, Y: 5, Z: 0}, {X: 0, Y: 5, Z: 0}, {X: 0, Y: 0, Z: 0}}},
		GridMaps:        []soundplanimport.GridMapMetadata{{ResultSubFolder: "RS01", GMFile: "RRLK0010.GM", PointsTotal: 2}},
	}

	prep, hasPrep, err := prepareSoundPlanRasterCompare(projectRoot, report, modelPath)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if !hasPrep || len(prep.syntheticReceiverIDs) == 0 {
		t.Fatal("expected prepared synthetic raster receivers")
	}

	if got, want := prep.report.Status, "heuristic_scanline_compare"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}

	receiverID := prep.syntheticReceiverIDs[0]
	table := results.ReceiverTable{
		IndicatorOrder: []string{schall03.IndicatorLrDay, schall03.IndicatorLrNight},
		Unit:           "dB(A)",
		Records: []results.ReceiverRecord{{
			ID:      receiverID,
			X:       0,
			Y:       0,
			HeightM: 4,
			Values:  map[string]float64{schall03.IndicatorLrDay: 51, schall03.IndicatorLrNight: 41},
		}},
	}

	reportOut, artifact, err := finalizeSoundPlanRasterCompare(projectRoot, prep, table, 0.5)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}

	if reportOut == nil {
		t.Fatal("expected raster compare report")
	}

	if reportOut.Alignment != "calcarea_scanlines_centered" {
		t.Fatalf("alignment = %q", reportOut.Alignment)
	}

	if reportOut.ArtifactPath != defaultRasterCompareArtifactPath {
		t.Fatalf("artifact path = %q", reportOut.ArtifactPath)
	}

	if artifact == nil {
		t.Fatal("expected raster artifact")
	}

	if len(artifact.Runs) != 1 {
		t.Fatalf("artifact run count = %d", len(artifact.Runs))
	}

	if got := artifact.Runs[0].ComparedCellCount; got != 1 {
		t.Fatalf("compared cell count = %d", got)
	}

	t.Cleanup(func() {
		_ = os.Remove(filepath.Join(projectRoot, filepath.FromSlash(defaultRasterCompareArtifactPath)))

		cleanupRasterComparePreparation(prep)
	})
}

func TestPrepareSoundPlanRasterCompareUsesMetadataWhenCalcAreaMissing(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()

	modelPath := filepath.Join(".noise", "model", "model.normalized.geojson")

	modelFile := filepath.Join(projectRoot, modelPath)
	if err := os.MkdirAll(filepath.Dir(modelFile), 0o750); err != nil {
		t.Fatalf("make model dir: %v", err)
	}

	featureCollection := map[string]any{
		"type": "FeatureCollection",
		"features": []any{map[string]any{
			"type":       "Feature",
			"properties": map[string]any{"id": "base", "kind": "receiver", "height_m": 4.0},
			"geometry":   map[string]any{"type": "Point", "coordinates": []any{0, 0}},
		}},
	}

	payload, err := json.Marshal(featureCollection)
	if err != nil {
		t.Fatalf("marshal model: %v", err)
	}

	if err := os.WriteFile(modelFile, payload, 0o600); err != nil {
		t.Fatalf("write model: %v", err)
	}

	soundPlanRoot := filepath.Join(projectRoot, "soundplan")

	gmDir := filepath.Join(soundPlanRoot, "RS01")
	if err := os.MkdirAll(gmDir, 0o750); err != nil {
		t.Fatalf("make gm dir: %v", err)
	}

	gmPath := filepath.Join(gmDir, "RRLK0010.GM")
	if err := writeTestGridMapFile(gmPath, []testGridCell{{ground: -1, day: 0, night: 0, flag: 1}, {ground: 110, day: 50, night: 40, flag: 1}}); err != nil {
		t.Fatalf("write gm: %v", err)
	}

	report := soundPlanImportReport{
		SourcePath:      filepath.Base(soundPlanRoot),
		ProjectCRS:      "EPSG:25832",
		GridResolutionM: 5,
		CalcArea:        nil,
		GridMaps:        []soundplanimport.GridMapMetadata{{ResultSubFolder: "RS01", GMFile: "RRLK0010.GM", PointsTotal: 2, OriginX: 100, OriginY: 200, SpacingX: 10, SpacingY: 10}},
	}

	prep, hasPrep, err := prepareSoundPlanRasterCompare(projectRoot, report, modelPath)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if !hasPrep || len(prep.syntheticReceiverIDs) == 0 {
		t.Fatal("expected prepared synthetic raster receivers")
	}

	if got, want := prep.report.Alignment, soundPlanRasterMetadataAlignment; got != want {
		t.Fatalf("alignment = %q, want %q", got, want)
	}

	if got, want := prep.report.Status, "heuristic_scanline_compare"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}

	t.Cleanup(func() {
		_ = os.Remove(filepath.Join(projectRoot, filepath.FromSlash(defaultRasterCompareArtifactPath)))

		cleanupRasterComparePreparation(prep)
	})
}

type testGridCell struct {
	ground float32
	day    float32
	night  float32
	flag   byte
}

func writeTestGridMapFile(path string, cells []testGridCell) error {
	var buf bytes.Buffer
	for _, cell := range cells {
		if err := binary.Write(&buf, binary.LittleEndian, cell.ground); err != nil {
			return err
		}

		if err := binary.Write(&buf, binary.LittleEndian, cell.day); err != nil {
			return err
		}

		if err := binary.Write(&buf, binary.LittleEndian, cell.night); err != nil {
			return err
		}

		if err := buf.WriteByte(cell.flag); err != nil {
			return err
		}
	}

	return os.WriteFile(path, buf.Bytes(), 0o600)
}
