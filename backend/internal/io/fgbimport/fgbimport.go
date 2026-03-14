// Package fgbimport reads FlatGeobuf (.fgb) files and converts features
// into the project model format.
package fgbimport

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/gogama/flatgeobuf/flatgeobuf"
	"github.com/gogama/flatgeobuf/flatgeobuf/flat"
)

// ReadResult holds the result of reading a FlatGeobuf file.
type ReadResult struct {
	Collection modelgeojson.FeatureCollection
	EPSGCode   int // 0 if CRS could not be determined
}

// Read reads all features from a FlatGeobuf file and returns a GeoJSON-compatible
// FeatureCollection ready for Normalize.
func Read(path string) (modelgeojson.FeatureCollection, error) {
	result, err := ReadWithCRS(path)
	if err != nil {
		return modelgeojson.FeatureCollection{}, err
	}

	return result.Collection, nil
}

// ReadWithCRS reads all features from a FlatGeobuf file and also extracts the CRS
// from the file header.
func ReadWithCRS(path string) (ReadResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return ReadResult{}, fmt.Errorf("fgb: open %q: %w", path, err)
	}

	defer f.Close()

	r := flatgeobuf.NewFileReader(f)

	hdr, err := r.Header()
	if err != nil {
		return ReadResult{}, fmt.Errorf("fgb: read header: %w", err)
	}

	flatFeatures, err := r.DataRem()
	if err != nil {
		return ReadResult{}, fmt.Errorf("fgb: read features: %w", err)
	}

	headerGeomType := hdr.GeometryType()
	epsg := extractHeaderCRS(hdr)

	features := make([]modelgeojson.GeoJSONFeature, 0, len(flatFeatures))

	for i := range flatFeatures {
		feat, convertErr := convertFeature(&flatFeatures[i], hdr, headerGeomType, i)
		if convertErr != nil {
			return ReadResult{}, fmt.Errorf("fgb: feature %d: %w", i, convertErr)
		}

		if feat != nil {
			features = append(features, *feat)
		}
	}

	return ReadResult{
		Collection: modelgeojson.FeatureCollection{
			Type:     "FeatureCollection",
			Features: features,
		},
		EPSGCode: epsg,
	}, nil
}

// extractHeaderCRS reads the CRS from a FlatGeobuf header. Returns the EPSG code, or 0.
func extractHeaderCRS(hdr *flat.Header) int {
	crs := new(flat.Crs)

	crs = hdr.Crs(crs)
	if crs == nil {
		return 0
	}

	code := int(crs.Code())
	if code > 0 {
		return code
	}

	return 0
}

func convertFeature(feat *flat.Feature, hdr *flat.Header, headerGeomType flat.GeometryType, index int) (*modelgeojson.GeoJSONFeature, error) {
	geom := new(flat.Geometry)
	if feat.Geometry(geom) == nil {
		return nil, nil //nolint:nilnil // skip features without geometry
	}

	geomType, coords, err := geometryToGeoJSON(geom, headerGeomType)
	if err != nil {
		return nil, err
	}

	if geomType == "" {
		return nil, nil //nolint:nilnil // skip empty geometry
	}

	props, err := readProperties(feat, hdr)
	if err != nil {
		return nil, fmt.Errorf("read properties: %w", err)
	}

	featureID := extractID(props, index)

	return &modelgeojson.GeoJSONFeature{
		Type:       "Feature",
		ID:         featureID,
		Properties: props,
		Geometry: modelgeojson.Geometry{
			Type:        geomType,
			Coordinates: coords,
		},
	}, nil
}

func geometryToGeoJSON(geom *flat.Geometry, headerType flat.GeometryType) (string, any, error) {
	gt := geom.Type()
	if gt == flat.GeometryTypeUnknown {
		gt = headerType
	}

	switch gt {
	case flat.GeometryTypePoint:
		return "Point", decodePoint(geom), nil
	case flat.GeometryTypeLineString:
		return "LineString", decodeCoordSequence(geom, 0, geom.XyLength()/2), nil
	case flat.GeometryTypePolygon:
		return "Polygon", decodePolygon(geom), nil
	case flat.GeometryTypeMultiPoint:
		return "MultiPoint", decodeMultiPoint(geom), nil
	case flat.GeometryTypeMultiLineString:
		return "MultiLineString", decodeMultiLineString(geom), nil
	case flat.GeometryTypeMultiPolygon:
		return "MultiPolygon", decodeMultiPolygon(geom), nil
	default:
		return "", nil, fmt.Errorf("unsupported geometry type: %s", gt)
	}
}

func decodePoint(geom *flat.Geometry) []any {
	if geom.XyLength() < 2 {
		return nil
	}

	return []any{geom.Xy(0), geom.Xy(1)}
}

// decodeCoordSequence extracts a slice of [x, y] coordinate pairs from xy array.
// start and end are in coordinate-pair counts (not xy-index).
func decodeCoordSequence(geom *flat.Geometry, start, end int) []any {
	pts := make([]any, 0, end-start)

	for i := start; i < end; i++ {
		x := geom.Xy(i * 2)
		y := geom.Xy(i*2 + 1)
		pts = append(pts, []any{x, y})
	}

	return pts
}

func decodePolygon(geom *flat.Geometry) []any {
	numEnds := geom.EndsLength()
	totalPairs := geom.XyLength() / 2

	if numEnds == 0 {
		// Single ring: all coordinates form one ring.
		return []any{decodeCoordSequence(geom, 0, totalPairs)}
	}

	rings := make([]any, 0, numEnds)
	start := 0

	for i := range numEnds {
		end := int(geom.Ends(i))
		rings = append(rings, decodeCoordSequence(geom, start, end))
		start = end
	}

	return rings
}

func decodeMultiPoint(geom *flat.Geometry) []any {
	n := geom.XyLength() / 2
	pts := make([]any, 0, n)

	for i := range n {
		pts = append(pts, []any{geom.Xy(i * 2), geom.Xy(i*2 + 1)})
	}

	return pts
}

func decodeMultiLineString(geom *flat.Geometry) []any {
	if geom.PartsLength() > 0 {
		return decodePartsCoords(geom)
	}

	// Use ends array to split coordinate sequences into linestrings.
	numEnds := geom.EndsLength()
	if numEnds == 0 {
		return []any{decodeCoordSequence(geom, 0, geom.XyLength()/2)}
	}

	lines := make([]any, 0, numEnds)
	start := 0

	for i := range numEnds {
		end := int(geom.Ends(i))
		lines = append(lines, decodeCoordSequence(geom, start, end))
		start = end
	}

	return lines
}

func decodeMultiPolygon(geom *flat.Geometry) []any {
	n := geom.PartsLength()
	if n == 0 {
		// Fallback: treat as single polygon.
		return []any{decodePolygon(geom)}
	}

	polys := make([]any, 0, n)
	part := new(flat.Geometry)

	for i := range n {
		if !geom.Parts(part, i) {
			continue
		}

		polys = append(polys, decodePolygon(part))
	}

	return polys
}

func decodePartsCoords(geom *flat.Geometry) []any {
	n := geom.PartsLength()
	parts := make([]any, 0, n)
	part := new(flat.Geometry)

	for i := range n {
		if !geom.Parts(part, i) {
			continue
		}

		parts = append(parts, decodeCoordSequence(part, 0, part.XyLength()/2))
	}

	return parts
}

func readProperties(feat *flat.Feature, hdr *flat.Header) (map[string]any, error) {
	propBytes := feat.PropertiesBytes()
	if len(propBytes) == 0 {
		return make(map[string]any), nil
	}

	pr := flatgeobuf.NewPropReader(bytes.NewReader(propBytes))
	props := make(map[string]any)
	numCols := hdr.ColumnsLength()

	for {
		colIdx, err := pr.ReadUShort()
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("read property column index: %w", err)
		}

		if int(colIdx) >= numCols {
			return nil, fmt.Errorf("property column index %d exceeds schema (%d columns)", colIdx, numCols)
		}

		var col flat.Column
		if !hdr.Columns(&col, int(colIdx)) {
			return nil, fmt.Errorf("column %d not found in schema", colIdx)
		}

		val, readErr := readPropertyValue(pr, col.Type())
		if readErr != nil {
			return nil, fmt.Errorf("read property %q: %w", string(col.Name()), readErr)
		}

		props[string(col.Name())] = normalizeValue(val)
	}

	return props, nil
}

func readPropertyValue(pr *flatgeobuf.PropReader, colType flat.ColumnType) (any, error) {
	switch colType {
	case flat.ColumnTypeByte:
		return pr.ReadByte()
	case flat.ColumnTypeUByte:
		return pr.ReadUByte()
	case flat.ColumnTypeBool:
		return pr.ReadBool()
	case flat.ColumnTypeShort:
		return pr.ReadShort()
	case flat.ColumnTypeUShort:
		return pr.ReadUShort()
	case flat.ColumnTypeInt:
		return pr.ReadInt()
	case flat.ColumnTypeUInt:
		return pr.ReadUInt()
	case flat.ColumnTypeLong:
		return pr.ReadLong()
	case flat.ColumnTypeULong:
		return pr.ReadULong()
	case flat.ColumnTypeFloat:
		return pr.ReadFloat()
	case flat.ColumnTypeDouble:
		return pr.ReadDouble()
	case flat.ColumnTypeString, flat.ColumnTypeDateTime:
		return pr.ReadString()
	case flat.ColumnTypeJson, flat.ColumnTypeBinary:
		return pr.ReadBinary()
	default:
		return nil, fmt.Errorf("unsupported column type %d", colType)
	}
}

func normalizeValue(val any) any {
	if val == nil {
		return nil
	}

	if b, ok := val.([]byte); ok {
		return string(b)
	}

	// Promote integer types to float64 for JSON compatibility.
	switch v := val.(type) {
	case int8:
		return float64(v)
	case uint8:
		return float64(v)
	case int16:
		return float64(v)
	case uint16:
		return float64(v)
	case int32:
		return float64(v)
	case uint32:
		return float64(v)
	case int64:
		return float64(v)
	case uint64:
		return float64(v)
	case float32:
		return float64(v)
	default:
		return val
	}
}

func extractID(props map[string]any, index int) string {
	for _, key := range []string{"fid", "id", "FID", "ID"} {
		if val, ok := props[key]; ok {
			return formatID(val)
		}
	}

	return strconv.Itoa(index)
}

func formatID(val any) string {
	if val == nil {
		return ""
	}

	switch v := val.(type) {
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
