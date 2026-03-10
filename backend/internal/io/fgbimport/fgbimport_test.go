package fgbimport

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogama/flatgeobuf/flatgeobuf"
	"github.com/gogama/flatgeobuf/flatgeobuf/flat"
	flatbuffers "github.com/google/flatbuffers/go"
)

// --- FlatGeobuf test file helpers ---

// buildHeader creates a FlatBuffer-encoded Header with the given geometry type and columns.
func buildHeader(geomType flat.GeometryType, columns []testColumn, featureCount int) *flat.Header {
	bldr := flatbuffers.NewBuilder(256)

	// Build column offsets.
	colOffsets := make([]flatbuffers.UOffsetT, len(columns))
	for i := len(columns) - 1; i >= 0; i-- {
		nameOff := bldr.CreateString(columns[i].name)
		flat.ColumnStart(bldr)
		flat.ColumnAddName(bldr, nameOff)
		flat.ColumnAddType(bldr, columns[i].colType)
		colOffsets[i] = flat.ColumnEnd(bldr)
	}

	flat.HeaderStartColumnsVector(bldr, len(columns))

	for i := len(colOffsets) - 1; i >= 0; i-- {
		bldr.PrependUOffsetT(colOffsets[i])
	}

	colsVec := bldr.EndVector(len(columns))

	flat.HeaderStart(bldr)
	flat.HeaderAddGeometryType(bldr, geomType)
	flat.HeaderAddColumns(bldr, colsVec)
	flat.HeaderAddIndexNodeSize(bldr, 0)
	flat.HeaderAddFeaturesCount(bldr, uint64(featureCount))
	hdrOff := flat.HeaderEnd(bldr)

	bldr.FinishSizePrefixed(hdrOff)

	buf := bldr.FinishedBytes()

	return flat.GetSizePrefixedRootAsHeader(buf, 0)
}

type testColumn struct {
	name    string
	colType flat.ColumnType
}

// buildFeature creates a FlatBuffer-encoded Feature with xy geometry and properties.
func buildFeature(geomType flat.GeometryType, xy []float64, ends []uint32, props []byte) flat.Feature {
	bldr := flatbuffers.NewBuilder(256)

	// Build geometry.
	flat.GeometryStartXyVector(bldr, len(xy))

	for i := len(xy) - 1; i >= 0; i-- {
		bldr.PrependFloat64(xy[i])
	}

	xyVec := bldr.EndVector(len(xy))

	var endsVec flatbuffers.UOffsetT

	if len(ends) > 0 {
		flat.GeometryStartEndsVector(bldr, len(ends))

		for i := len(ends) - 1; i >= 0; i-- {
			bldr.PrependUint32(ends[i])
		}

		endsVec = bldr.EndVector(len(ends))
	}

	flat.GeometryStart(bldr)
	flat.GeometryAddXy(bldr, xyVec)
	flat.GeometryAddType(bldr, geomType)

	if len(ends) > 0 {
		flat.GeometryAddEnds(bldr, endsVec)
	}

	geomOff := flat.GeometryEnd(bldr)

	// Build properties.
	var propsOff flatbuffers.UOffsetT

	if len(props) > 0 {
		propsOff = bldr.CreateByteVector(props)
	}

	flat.FeatureStart(bldr)
	flat.FeatureAddGeometry(bldr, geomOff)

	if len(props) > 0 {
		flat.FeatureAddProperties(bldr, propsOff)
	}

	featOff := flat.FeatureEnd(bldr)

	bldr.FinishSizePrefixed(featOff)

	return *flat.GetSizePrefixedRootAsFeature(bldr.FinishedBytes(), 0)
}

// encodeProps encodes properties using PropWriter matching the given column schema.
func encodeProps(columns []testColumn, values []any) []byte {
	var buf bytes.Buffer
	pw := flatgeobuf.NewPropWriter(&buf)

	for i, val := range values {
		if val == nil {
			continue
		}

		_, _ = pw.WriteUShort(uint16(i))

		switch columns[i].colType {
		case flat.ColumnTypeString, flat.ColumnTypeDateTime:
			_, _ = pw.WriteString(val.(string))
		case flat.ColumnTypeDouble:
			_, _ = pw.WriteDouble(val.(float64))
		case flat.ColumnTypeFloat:
			_, _ = pw.WriteFloat(val.(float32))
		case flat.ColumnTypeInt:
			_, _ = pw.WriteInt(val.(int32))
		case flat.ColumnTypeUInt:
			_, _ = pw.WriteUInt(val.(uint32))
		case flat.ColumnTypeLong:
			_, _ = pw.WriteLong(val.(int64))
		case flat.ColumnTypeBool:
			_, _ = pw.WriteBool(val.(bool))
		}
	}

	return buf.Bytes()
}

// createTestFGB writes a complete FlatGeobuf file to disk with the given features.
func createTestFGB(t *testing.T, geomType flat.GeometryType, columns []testColumn, features []flat.Feature) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.fgb")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test fgb: %v", err)
	}

	defer f.Close()

	hdr := buildHeader(geomType, columns, len(features))
	w := flatgeobuf.NewFileWriter(f)

	_, err = w.Header(hdr)
	if err != nil {
		t.Fatalf("write header: %v", err)
	}

	if len(features) > 0 {
		_, err = w.Data(features)
		if err != nil {
			t.Fatalf("write features: %v", err)
		}
	}

	return path
}

// --- Tests ---

func TestRead_PointFeatures(t *testing.T) {
	columns := []testColumn{
		{name: "kind", colType: flat.ColumnTypeString},
		{name: "height_m", colType: flat.ColumnTypeDouble},
		{name: "name", colType: flat.ColumnTypeString},
	}

	props1 := encodeProps(columns, []any{"building", 12.5, "Bldg A"})
	props2 := encodeProps(columns, []any{"source", nil, "Road B"})

	feat1 := buildFeature(flat.GeometryTypePoint, []float64{8.6, 47.3}, nil, props1)
	feat2 := buildFeature(flat.GeometryTypePoint, []float64{9.1, 48.0}, nil, props2)

	path := createTestFGB(t, flat.GeometryTypePoint, columns, []flat.Feature{feat1, feat2})

	fc, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if fc.Type != "FeatureCollection" {
		t.Errorf("expected FeatureCollection, got %q", fc.Type)
	}

	if len(fc.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(fc.Features))
	}

	f0 := fc.Features[0]
	if f0.Geometry.Type != "Point" {
		t.Errorf("feature 0: expected Point, got %q", f0.Geometry.Type)
	}

	coords, ok := f0.Geometry.Coordinates.([]any)
	if !ok || len(coords) != 2 {
		t.Fatalf("feature 0: expected [x, y], got %T %v", f0.Geometry.Coordinates, f0.Geometry.Coordinates)
	}

	if coords[0].(float64) != 8.6 || coords[1].(float64) != 47.3 {
		t.Errorf("feature 0: unexpected coords: %v", coords)
	}

	if f0.Properties["kind"] != "building" {
		t.Errorf("feature 0: expected kind='building', got %v", f0.Properties["kind"])
	}

	if f0.Properties["height_m"] != 12.5 {
		t.Errorf("feature 0: expected height_m=12.5, got %v", f0.Properties["height_m"])
	}

	f1 := fc.Features[1]
	if f1.Geometry.Type != "Point" {
		t.Errorf("feature 1: expected Point, got %q", f1.Geometry.Type)
	}

	if f1.Properties["name"] != "Road B" {
		t.Errorf("feature 1: expected name='Road B', got %v", f1.Properties["name"])
	}
}

func TestRead_LineStringFeature(t *testing.T) {
	columns := []testColumn{
		{name: "kind", colType: flat.ColumnTypeString},
	}

	props := encodeProps(columns, []any{"source"})
	feat := buildFeature(flat.GeometryTypeLineString, []float64{0, 0, 1, 1, 2, 0}, nil, props)

	path := createTestFGB(t, flat.GeometryTypeLineString, columns, []flat.Feature{feat})

	fc, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(fc.Features))
	}

	f := fc.Features[0]
	if f.Geometry.Type != "LineString" {
		t.Errorf("expected LineString, got %q", f.Geometry.Type)
	}

	pts, ok := f.Geometry.Coordinates.([]any)
	if !ok || len(pts) != 3 {
		t.Fatalf("expected 3 points, got %T %v", f.Geometry.Coordinates, f.Geometry.Coordinates)
	}
}

func TestRead_PolygonFeature(t *testing.T) {
	columns := []testColumn{
		{name: "kind", colType: flat.ColumnTypeString},
		{name: "height_m", colType: flat.ColumnTypeDouble},
	}

	// Square polygon: (0,0), (1,0), (1,1), (0,1), (0,0)
	xy := []float64{0, 0, 1, 0, 1, 1, 0, 1, 0, 0}
	ends := []uint32{5}

	props := encodeProps(columns, []any{"building", 10.0})
	feat := buildFeature(flat.GeometryTypePolygon, xy, ends, props)

	path := createTestFGB(t, flat.GeometryTypePolygon, columns, []flat.Feature{feat})

	fc, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(fc.Features))
	}

	f := fc.Features[0]
	if f.Geometry.Type != "Polygon" {
		t.Errorf("expected Polygon, got %q", f.Geometry.Type)
	}

	rings, ok := f.Geometry.Coordinates.([]any)
	if !ok || len(rings) != 1 {
		t.Fatalf("expected 1 ring, got %T %v", f.Geometry.Coordinates, f.Geometry.Coordinates)
	}

	ring, ok := rings[0].([]any)
	if !ok || len(ring) != 5 {
		t.Fatalf("expected 5 points in ring, got %T %v", rings[0], rings[0])
	}
}

func TestRead_EmptyFile(t *testing.T) {
	path := createTestFGB(t, flat.GeometryTypePoint, nil, nil)

	fc, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(fc.Features) != 0 {
		t.Errorf("expected 0 features, got %d", len(fc.Features))
	}
}

func TestRead_FileNotFound(t *testing.T) {
	_, err := Read(filepath.Join(t.TempDir(), "does_not_exist.fgb"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestRead_PropertyTypes(t *testing.T) {
	columns := []testColumn{
		{name: "str_col", colType: flat.ColumnTypeString},
		{name: "int_col", colType: flat.ColumnTypeInt},
		{name: "dbl_col", colType: flat.ColumnTypeDouble},
		{name: "bool_col", colType: flat.ColumnTypeBool},
	}

	props := encodeProps(columns, []any{"hello", int32(42), 3.14, true})
	feat := buildFeature(flat.GeometryTypePoint, []float64{1, 2}, nil, props)

	path := createTestFGB(t, flat.GeometryTypePoint, columns, []flat.Feature{feat})

	fc, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	p := fc.Features[0].Properties
	if p["str_col"] != "hello" {
		t.Errorf("str_col: got %v", p["str_col"])
	}

	if p["int_col"] != float64(42) {
		t.Errorf("int_col: expected float64(42), got %T %v", p["int_col"], p["int_col"])
	}

	if p["dbl_col"] != 3.14 {
		t.Errorf("dbl_col: got %v", p["dbl_col"])
	}

	if p["bool_col"] != true {
		t.Errorf("bool_col: got %v", p["bool_col"])
	}
}

func TestRead_IDExtraction(t *testing.T) {
	columns := []testColumn{
		{name: "id", colType: flat.ColumnTypeString},
		{name: "kind", colType: flat.ColumnTypeString},
	}

	props := encodeProps(columns, []any{"feat-42", "source"})
	feat := buildFeature(flat.GeometryTypePoint, []float64{1, 2}, nil, props)

	path := createTestFGB(t, flat.GeometryTypePoint, columns, []flat.Feature{feat})

	fc, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if fc.Features[0].ID != "feat-42" {
		t.Errorf("expected ID 'feat-42', got %v", fc.Features[0].ID)
	}
}
