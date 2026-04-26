package cli

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
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
		"soundplan-raster-r001-c001": {x: 0, y: 5, row: 0, col: 0},
		"soundplan-raster-r002-c001": {x: 2.5, y: 0, row: 1, col: 0},
		"soundplan-raster-r002-c002": {x: 7.5, y: 0, row: 1, col: 1},
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
	if err := os.MkdirAll(filepath.Dir(modelFile), 0o755); err != nil {
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
	if err := os.MkdirAll(gmDir, 0o755); err != nil {
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

	prep, err := prepareSoundPlanRasterCompare(projectRoot, report, modelPath)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if prep == nil || len(prep.syntheticReceiverIDs) == 0 {
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
	if err := os.MkdirAll(filepath.Dir(modelFile), 0o755); err != nil {
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
	if err := os.MkdirAll(gmDir, 0o755); err != nil {
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

	prep, err := prepareSoundPlanRasterCompare(projectRoot, report, modelPath)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if prep == nil || len(prep.syntheticReceiverIDs) == 0 {
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
