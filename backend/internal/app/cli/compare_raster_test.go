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

	"github.com/aconiq/backend/internal/io/soundplanimport"
	"github.com/aconiq/backend/internal/results"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// TestSyntheticRasterReceiverHeight pins where the synthetic raster receivers
// get their height. It used to be the first receiver in the model, which made
// every raster delta move whenever the receiver import changed — a coupling
// between two unrelated comparisons that nothing declared.
func TestSyntheticRasterReceiverHeight(t *testing.T) {
	t.Parallel()

	if got := syntheticRasterReceiverHeight(soundPlanImportReport{GridMapHeightM: 2.0}); got != 2.0 {
		t.Fatalf("receiver height = %f, want the bundle's grid-map height 2.0", got)
	}

	if got := syntheticRasterReceiverHeight(soundPlanImportReport{}); got != defaultGridMapReceiverHeightM {
		t.Fatalf("fallback receiver height = %f, want %f", got, defaultGridMapReceiverHeightM)
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

// TestPrepareAndFinalizeSoundPlanRasterCompareFallsBackToImportReport walks the
// whole prepare/finalize path for a project whose model carries **no**
// calc-area feature, so the SoundPLAN import report's CalcArea is what the
// synthesis uses. That is the fallback, not the primary path: the model's
// feature wins whenever there is one, which
// TestPrepareSoundPlanRasterCompareModelCalcAreaWins covers.
func TestPrepareAndFinalizeSoundPlanRasterCompareFallsBackToImportReport(t *testing.T) {
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

	prep, hasPrep, err := prepareSoundPlanRasterCompare(projectRoot, report, modelPath, "")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if !hasPrep || len(prep.syntheticReceiverIDs) == 0 {
		t.Fatal("expected prepared synthetic raster receivers")
	}

	if got, want := prep.report.Status, "heuristic_scanline_compare"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}

	if got, want := prep.report.CalcAreaSource, calcAreaSourceImportReport; got != want {
		t.Fatalf("calc area source = %q, want %q", got, want)
	}

	// Nothing to compare the report's area against, so no delta is recorded —
	// which is not the same as the two having agreed.
	if prep.report.CalcAreaBoundsDelta != nil {
		t.Fatalf("bounds delta = %v, want none when the model carries no area", *prep.report.CalcAreaBoundsDelta)
	}

	receiverID := prep.syntheticReceiverIDs[0]
	indicators := []string{schall03.IndicatorLrDay, schall03.IndicatorLrNight}

	table := results.ReceiverTable{
		IndicatorOrder: indicators,
		Units:          results.UniformUnits(indicators, "dB(A)"),
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

	if got, want := artifact.CalcAreaSource, calcAreaSourceImportReport; got != want {
		t.Fatalf("artifact calc area source = %q, want %q", got, want)
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

// rasterCompareProject writes a minimal project whose model carries one base
// receiver plus the given extra features, and one decodable single-cell GM
// payload. It returns the project root and the model path relative to it.
func rasterCompareProject(t *testing.T, extraFeatures ...any) (string, string) {
	t.Helper()

	projectRoot := t.TempDir()

	modelPath := filepath.Join(".noise", "model", "model.normalized.geojson")

	modelFile := filepath.Join(projectRoot, modelPath)
	if err := os.MkdirAll(filepath.Dir(modelFile), 0o750); err != nil {
		t.Fatalf("make model dir: %v", err)
	}

	features := append([]any{map[string]any{
		"type":       "Feature",
		"properties": map[string]any{"id": "base", "kind": "receiver", "height_m": 4.0},
		"geometry":   map[string]any{"type": "Point", "coordinates": []any{0, 0}},
	}}, extraFeatures...)

	payload, err := json.Marshal(map[string]any{"type": "FeatureCollection", "features": features})
	if err != nil {
		t.Fatalf("marshal model: %v", err)
	}

	if err := os.WriteFile(modelFile, payload, 0o600); err != nil {
		t.Fatalf("write model: %v", err)
	}

	writeRasterCompareGridMap(t, projectRoot, "RS01", "RRLK0010.GM",
		[]testGridCell{{ground: -1, day: 0, night: 0, flag: 1}, {ground: 110, day: 50, night: 40, flag: 1}})

	return projectRoot, modelPath
}

// writeRasterCompareGridMap lays one decodable GM payload out under a project's
// SoundPLAN root. A project holds several, and which of them a comparison reads
// is the thing the selection decides, so the cells are the caller's to choose.
func writeRasterCompareGridMap(t *testing.T, projectRoot, subFolder, gmFile string, cells []testGridCell) {
	t.Helper()

	gmDir := filepath.Join(projectRoot, "soundplan", subFolder)
	if err := os.MkdirAll(gmDir, 0o750); err != nil {
		t.Fatalf("make gm dir: %v", err)
	}

	if err := writeTestGridMapFile(filepath.Join(gmDir, gmFile), cells); err != nil {
		t.Fatalf("write gm: %v", err)
	}
}

// calcAreaFeature is one calc-area feature spanning [minX,maxX]×[minY,maxY],
// with an optional extra vertex on the top edge so that two areas can describe
// the same envelope with different vertex counts.
func calcAreaFeature(minX, minY, maxX, maxY float64, extraTopVertex bool) any {
	ring := []any{
		[]any{minX, minY},
		[]any{maxX, minY},
		[]any{maxX, maxY},
	}

	if extraTopVertex {
		ring = append(ring, []any{(minX + maxX) / 2, maxY})
	}

	ring = append(ring, []any{minX, maxY}, []any{minX, minY})

	return map[string]any{
		"type":       "Feature",
		"properties": map[string]any{"id": "drawn-area", "kind": "calc-area"},
		"geometry":   map[string]any{"type": "Polygon", "coordinates": []any{ring}},
	}
}

// syntheticRasterReceiverXY reads the synthesized raster receivers back out of
// the temporary model the preparation wrote, which is the only place their
// coordinates exist.
func syntheticRasterReceiverXY(t *testing.T, prep *rasterComparePreparation) map[string][2]float64 {
	t.Helper()

	payload, err := os.ReadFile(prep.tempModelPath)
	if err != nil {
		t.Fatalf("read temp model: %v", err)
	}

	// Coordinates stay raw: the same collection carries the calc-area polygon,
	// whose coordinates are nested arrays rather than a position.
	var collection struct {
		Features []struct {
			Properties map[string]any `json:"properties"`
			Geometry   struct {
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}

	if err := json.Unmarshal(payload, &collection); err != nil {
		t.Fatalf("decode temp model: %v", err)
	}

	out := make(map[string][2]float64)

	for _, feature := range collection.Features {
		id, _ := feature.Properties["id"].(string)
		if !strings.HasPrefix(id, soundPlanRasterReceiverPrefix) {
			continue
		}

		var position []float64
		if err := json.Unmarshal(feature.Geometry.Coordinates, &position); err != nil {
			t.Fatalf("decode position of %q: %v", id, err)
		}

		if len(position) < 2 {
			t.Fatalf("synthetic receiver %q has no position", id)
		}

		out[id] = [2]float64{position[0], position[1]}
	}

	return out
}

// TestPrepareSoundPlanRasterCompareModelCalcAreaWins is the primary path: the
// model's calc-area feature governs where the synthesized receivers go, even
// though the SoundPLAN import report carries an area of its own and a different
// one. The model is the extent a run computes over, so it is the extent the
// comparison must use.
//
// The two areas here differ in both envelope and vertex count, so the divergence
// warning fires and names the counts and the measured envelope delta.
func TestPrepareSoundPlanRasterCompareModelCalcAreaWins(t *testing.T) {
	t.Parallel()

	// The model's area spans 0..10 with an extra vertex on the top edge; the
	// import report's spans 0..5 with four. One GM row of one cell puts the
	// receiver at the centre of the area's span, so 5,5 against 2.5,2.5.
	projectRoot, modelPath := rasterCompareProject(t, calcAreaFeature(0, 0, 10, 10, true))

	report := soundPlanImportReport{
		SourcePath:      "soundplan",
		ProjectCRS:      "EPSG:25832",
		GridResolutionM: 5,
		CalcArea: &soundPlanImportCalcArea{Points: []soundPlanPoint{
			{X: 0, Y: 0}, {X: 5, Y: 0}, {X: 5, Y: 5}, {X: 0, Y: 5}, {X: 0, Y: 0},
		}},
		GridMaps: []soundplanimport.GridMapMetadata{{ResultSubFolder: "RS01", GMFile: "RRLK0010.GM", PointsTotal: 2}},
	}

	prep, hasPrep, err := prepareSoundPlanRasterCompare(projectRoot, report, modelPath, "")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	t.Cleanup(func() { cleanupRasterComparePreparation(prep) })

	if !hasPrep || len(prep.syntheticReceiverIDs) != 1 {
		t.Fatalf("expected one synthetic receiver, got %d", len(prep.syntheticReceiverIDs))
	}

	if got, want := prep.report.CalcAreaSource, calcAreaSourceModel; got != want {
		t.Fatalf("calc area source = %q, want %q", got, want)
	}

	positions := syntheticRasterReceiverXY(t, prep)

	position, ok := positions[prep.syntheticReceiverIDs[0]]
	if !ok {
		t.Fatalf("no position for %q", prep.syntheticReceiverIDs[0])
	}

	if math.Abs(position[0]-5) > 1e-9 || math.Abs(position[1]-5) > 1e-9 {
		t.Fatalf("receiver at (%.3f, %.3f); want the model's centre (5, 5), not the import report's (2.5, 2.5)",
			position[0], position[1])
	}

	if prep.report.CalcAreaBoundsDelta == nil {
		t.Fatal("expected a recorded bounds delta when both areas exist")
	}

	if got := *prep.report.CalcAreaBoundsDelta; math.Abs(got-5) > 1e-9 {
		t.Fatalf("bounds delta = %v m, want 5", got)
	}

	divergence := findWarning(prep.report.Warnings, "the model wins")
	if divergence == "" {
		t.Fatalf("expected a divergence warning, got %v", prep.report.Warnings)
	}

	for _, want := range []string{"drawn-area", "has 5 vertices", "has 4", "5 m"} {
		if !strings.Contains(divergence, want) {
			t.Fatalf("divergence warning %q does not name %q", divergence, want)
		}
	}
}

// TestPrepareSoundPlanRasterCompareAgreeingAreasAreSilent is the other half: the
// same shape from both sources warns about nothing, while the bounds delta is
// still recorded, because the artifact must say which area it used whether or
// not anything was wrong.
func TestPrepareSoundPlanRasterCompareAgreeingAreasAreSilent(t *testing.T) {
	t.Parallel()

	projectRoot, modelPath := rasterCompareProject(t, calcAreaFeature(0, 0, 10, 10, false))

	report := soundPlanImportReport{
		SourcePath:      "soundplan",
		ProjectCRS:      "EPSG:25832",
		GridResolutionM: 5,
		// Same square, but starting at a different vertex and wound the other
		// way — a vertex-by-vertex comparison would call this a divergence.
		CalcArea: &soundPlanImportCalcArea{Points: []soundPlanPoint{
			{X: 10, Y: 10}, {X: 10, Y: 0}, {X: 0, Y: 0}, {X: 0, Y: 10},
		}},
		GridMaps: []soundplanimport.GridMapMetadata{{ResultSubFolder: "RS01", GMFile: "RRLK0010.GM", PointsTotal: 2}},
	}

	prep, hasPrep, err := prepareSoundPlanRasterCompare(projectRoot, report, modelPath, "")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	t.Cleanup(func() { cleanupRasterComparePreparation(prep) })

	if !hasPrep {
		t.Fatal("expected a prepared comparison")
	}

	if got, want := prep.report.CalcAreaSource, calcAreaSourceModel; got != want {
		t.Fatalf("calc area source = %q, want %q", got, want)
	}

	if got := findWarning(prep.report.Warnings, "the model wins"); got != "" {
		t.Fatalf("unexpected divergence warning: %q", got)
	}

	if prep.report.CalcAreaBoundsDelta == nil {
		t.Fatal("expected the bounds delta to be recorded even when the areas agree")
	}

	if got := *prep.report.CalcAreaBoundsDelta; got != 0 {
		t.Fatalf("bounds delta = %v m, want 0", got)
	}
}

func findWarning(warnings []string, needle string) string {
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return warning
		}
	}

	return ""
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

	prep, hasPrep, err := prepareSoundPlanRasterCompare(projectRoot, report, modelPath, "")
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

// TestPrepareSoundPlanRasterCompareUnclosedImportReportArea is the fallback
// regression: a project with no calc-area feature, and an import report whose
// CalcArea.geo never repeats its first vertex. Before the ring was closed, every
// row lost the closing edge, found a single crossing and fell back to the
// bounding box with a warning.
func TestPrepareSoundPlanRasterCompareUnclosedImportReportArea(t *testing.T) {
	t.Parallel()

	projectRoot, modelPath := rasterCompareProject(t)

	report := soundPlanImportReport{
		SourcePath:      "soundplan",
		ProjectCRS:      "EPSG:25832",
		GridResolutionM: 5,
		CalcArea: &soundPlanImportCalcArea{Points: []soundPlanPoint{
			{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 6}, {X: 0, Y: 6},
		}},
		GridMaps: []soundplanimport.GridMapMetadata{{ResultSubFolder: "RS01", GMFile: "RRLK0010.GM", PointsTotal: 2}},
	}

	prep, hasPrep, err := prepareSoundPlanRasterCompare(projectRoot, report, modelPath, "")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	t.Cleanup(func() { cleanupRasterComparePreparation(prep) })

	if !hasPrep || len(prep.syntheticReceiverIDs) != 1 {
		t.Fatalf("expected one synthetic receiver, got %d", len(prep.syntheticReceiverIDs))
	}

	if got, want := prep.report.CalcAreaSource, calcAreaSourceImportReport; got != want {
		t.Fatalf("calc area source = %q, want %q", got, want)
	}

	if got, want := prep.report.CalcAreaRole, calcAreaRoleReceiverPlacement; got != want {
		t.Fatalf("calc area role = %q, want %q", got, want)
	}

	if got := findWarning(prep.report.Warnings, "could not be intersected with CalcArea"); got != "" {
		t.Fatalf("unexpected bounding-box fallback warning: %q", got)
	}

	positions := syntheticRasterReceiverXY(t, prep)

	position, ok := positions[prep.syntheticReceiverIDs[0]]
	if !ok {
		t.Fatalf("no position for %q", prep.syntheticReceiverIDs[0])
	}

	if math.Abs(position[0]-5) > 1e-9 {
		t.Fatalf("receiver x = %.3f, want the scanline centre 5", position[0])
	}
}

// TestPrepareSoundPlanRasterCompareGeographicCRSDeltaIsNotMetres walks the same
// diverging-areas scenario on a geographic project CRS. The number is unchanged,
// but neither the field nor the warning may call it metres.
func TestPrepareSoundPlanRasterCompareGeographicCRSDeltaIsNotMetres(t *testing.T) {
	t.Parallel()

	projectRoot, modelPath := rasterCompareProject(t, calcAreaFeature(0, 0, 10, 10, true))

	report := soundPlanImportReport{
		SourcePath:      "soundplan",
		ProjectCRS:      "EPSG:4326",
		GridResolutionM: 5,
		CalcArea: &soundPlanImportCalcArea{Points: []soundPlanPoint{
			{X: 0, Y: 0}, {X: 5, Y: 0}, {X: 5, Y: 5}, {X: 0, Y: 5}, {X: 0, Y: 0},
		}},
		GridMaps: []soundplanimport.GridMapMetadata{{ResultSubFolder: "RS01", GMFile: "RRLK0010.GM", PointsTotal: 2}},
	}

	prep, hasPrep, err := prepareSoundPlanRasterCompare(projectRoot, report, modelPath, "")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	t.Cleanup(func() { cleanupRasterComparePreparation(prep) })

	if !hasPrep {
		t.Fatal("expected a prepared comparison")
	}

	if got, want := prep.report.CalcAreaBoundsDeltaUnit, calcAreaDeltaUnitDegree; got != want {
		t.Fatalf("bounds delta unit = %q, want %q", got, want)
	}

	divergence := findWarning(prep.report.Warnings, "the model wins")
	if divergence == "" {
		t.Fatalf("expected a divergence warning, got %v", prep.report.Warnings)
	}

	if !strings.Contains(divergence, "5 degree") {
		t.Fatalf("divergence warning %q does not name the unit it measured in", divergence)
	}
}
