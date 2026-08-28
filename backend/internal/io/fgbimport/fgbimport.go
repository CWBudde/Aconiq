// Package fgbimport reads FlatGeobuf (.fgb) files and converts features
// into the project model format.
package fgbimport

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/gogama/flatgeobuf/flatgeobuf"
	"github.com/gogama/flatgeobuf/flatgeobuf/flat"
	flatbuffers "github.com/google/flatbuffers/go"
)

// Sizes of the FlatBuffers vector elements the geometry decoders read. A
// FlatGeobuf file is untrusted input and the reader library does not run a
// FlatBuffers verifier, so every vector length taken from the buffer is
// checked against the buffer's own size before it is used to size a slice or
// to index into the data.
const (
	xyElemBytes    = 8 // float64 coordinate
	endsElemBytes  = 4 // uint32 ring/part end index
	partsElemBytes = 4 // uoffset to a nested geometry
)

// Framing of the FlatGeobuf data section, which this package walks itself
// rather than leaving to github.com/gogama/flatgeobuf.
//
// The library offers two ways to read features, and both size an allocation
// from a number the file supplies without checking it against the file:
//
//   - FileReader.DataRem does make([]flat.Feature, features_count) from the
//     header's unvalidated uint64, so a 65-byte file can ask for a
//     terabyte-scale slice.
//   - FileReader.Data avoids that, but its per-feature readFeature does
//     make([]byte, 4+featureLen) straight from the 32-bit length prefix, so a
//     116-byte file can ask for 4 GiB. Both were reached by FuzzReadFGB.
//
// The data section is a flat sequence of length-prefixed feature buffers, so
// reading it here costs a dozen lines and lets every allocation be bounded by
// the bytes actually left in the file. Everything structural — magic number,
// header, positioning past the spatial index — stays with the library.
const (
	// featureLengthPrefixBytes is the size of the little-endian uint32 that
	// prefixes each feature buffer in the data section.
	featureLengthPrefixBytes = 4

	// minFeatureBytes is the smallest feature buffer the format allows: a
	// FlatBuffers root uoffset and nothing else. The library applies the same
	// floor.
	minFeatureBytes = flatbuffers.SizeUOffsetT
)

// aconiqPackagePrefix identifies this repository's own packages in a stack
// trace. See libraryFaultSite.
const aconiqPackagePrefix = "github.com/aconiq/backend/"

// xyPairCount returns the number of coordinate pairs the geometry's xy vector
// holds, rejecting a declared length the underlying buffer cannot contain.
func xyPairCount(geom *flat.Geometry) (int, error) {
	n := geom.XyLength()

	bufLen := len(geom.Table().Bytes)
	if n < 0 || n > bufLen/xyElemBytes {
		return 0, fmt.Errorf("xy vector declares %d values, more than the %d byte buffer can hold", n, bufLen)
	}

	return n / 2, nil
}

// endsCount returns the length of the geometry's ends vector, rejecting a
// declared length the underlying buffer cannot contain.
func endsCount(geom *flat.Geometry) (int, error) {
	n := geom.EndsLength()

	bufLen := len(geom.Table().Bytes)
	if n < 0 || n > bufLen/endsElemBytes {
		return 0, fmt.Errorf("ends vector declares %d values, more than the %d byte buffer can hold", n, bufLen)
	}

	return n, nil
}

// partsCount returns the length of the geometry's parts vector, rejecting a
// declared length the underlying buffer cannot contain.
func partsCount(geom *flat.Geometry) (int, error) {
	n := geom.PartsLength()

	bufLen := len(geom.Table().Bytes)
	if n < 0 || n > bufLen/partsElemBytes {
		return 0, fmt.Errorf("parts vector declares %d values, more than the %d byte buffer can hold", n, bufLen)
	}

	return n, nil
}

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

	return readAll(f)
}

// readAll decodes a whole FlatGeobuf stream. It is separate from ReadWithCRS so
// that the decoder can be driven from a byte slice, which is what FuzzReadFGB
// does.
//
// The source has to be seekable: knowing how many bytes are left is what makes
// the per-feature bound in readFeatures possible, and the reader library
// already prefers a seekable stream for skipping the spatial index.
func readAll(src io.ReadSeeker) (ReadResult, error) {
	size, err := src.Seek(0, io.SeekEnd)
	if err != nil {
		return ReadResult{}, fmt.Errorf("fgb: measure input: %w", err)
	}

	_, err = src.Seek(0, io.SeekStart)
	if err != nil {
		return ReadResult{}, fmt.Errorf("fgb: rewind input: %w", err)
	}

	r := flatgeobuf.NewFileReader(src)

	hdr, err := r.Header()
	if err != nil {
		return ReadResult{}, fmt.Errorf("fgb: read header: %w", err)
	}

	fields, err := readHeaderFields(hdr)
	if err != nil {
		return ReadResult{}, fmt.Errorf("fgb: %w", err)
	}

	dataStart, err := seekToDataSection(r, src)
	if err != nil {
		return ReadResult{}, fmt.Errorf("fgb: locate data section: %w", err)
	}

	features, err := readFeatures(src, size-dataStart, hdr, fields)
	if err != nil {
		return ReadResult{}, err
	}

	return ReadResult{
		Collection: modelgeojson.FeatureCollection{
			Type:     modelgeojson.TypeFeatureCollection,
			Features: features,
		},
		EPSGCode: fields.epsgCode,
	}, nil
}

// seekToDataSection leaves src positioned at the first feature and reports that
// offset.
//
// Skipping the spatial index means reading the header's index_node_size and
// features_count and recomputing the packed Hilbert R-tree's on-disk size,
// which is the library's job and not worth duplicating. FileReader.Data does
// it on its first call; asking it for zero features runs that positioning and
// reads nothing.
func seekToDataSection(r *flatgeobuf.FileReader, src io.Seeker) (int64, error) {
	_, err := r.Data(nil)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, fmt.Errorf("step over spatial index: %w", err)
	}

	offset, err := src.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, fmt.Errorf("read stream position: %w", err)
	}

	return offset, nil
}

// readFeatures walks the length-prefixed feature buffers of the data section,
// of which avail bytes remain, and converts each one.
func readFeatures(src io.Reader, avail int64, hdr *flat.Header, fields headerFields) ([]modelgeojson.GeoJSONFeature, error) {
	if avail < 0 {
		return nil, fmt.Errorf("fgb: data section starts %d bytes past the end of the file", -avail)
	}

	var (
		features []modelgeojson.GeoJSONFeature
		prefix   [featureLengthPrefixBytes]byte
		index    int
	)

	for avail >= featureLengthPrefixBytes {
		_, err := io.ReadFull(src, prefix[:])
		if err != nil {
			return nil, fmt.Errorf("fgb: feature %d: read length prefix: %w", index, err)
		}

		avail -= featureLengthPrefixBytes

		featureLen := int64(binary.LittleEndian.Uint32(prefix[:]))

		// The declared length is bounded against the bytes still to come
		// before it sizes the buffer, not after.
		if featureLen < minFeatureBytes || featureLen > avail {
			return nil, fmt.Errorf(
				"fgb: feature %d: declared length %d is outside the %d bytes left in the data section",
				index, featureLen, avail,
			)
		}

		buf := make([]byte, featureLengthPrefixBytes+featureLen)
		copy(buf, prefix[:])

		_, err = io.ReadFull(src, buf[featureLengthPrefixBytes:])
		if err != nil {
			return nil, fmt.Errorf("fgb: feature %d: read %d bytes: %w", index, featureLen, err)
		}

		avail -= featureLen

		flatFeature := flat.GetSizePrefixedRootAsFeature(buf, 0)

		feat, convertErr := decodeFeature(flatFeature, hdr, fields.geometryType, index)
		if convertErr != nil {
			return nil, fmt.Errorf("fgb: feature %d: %w", index, convertErr)
		}

		if feat != nil {
			features = append(features, *feat)
		}

		index++
	}

	if avail != 0 {
		return nil, fmt.Errorf("fgb: %d trailing bytes after the last feature", avail)
	}

	// The header's declared count is not allowed to size anything, but it is
	// still an integrity claim: a file that declares a count and then holds a
	// different number of features is truncated or corrupt.
	if fields.featuresCount > 0 && fields.featuresCount != uint64(index) {
		return nil, fmt.Errorf(
			"fgb: header declares %d features but the data section holds %d",
			fields.featuresCount, index,
		)
	}

	return features, nil
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

// CorruptHeaderError reports a FlatGeobuf header whose FlatBuffers encoding
// made the reader library fault instead of returning an error.
//
// github.com/gogama/flatgeobuf hands raw file bytes to
// github.com/google/flatbuffers without running a verifier over them, and that
// flatbuffers release ships no verifier to run: the Go runtime package at the
// version this module pins contains no Verify function and no Verifier type,
// and the generated flat package carries no per-table verification helpers
// either. A table whose vtable offset points outside the buffer therefore
// panics with an out-of-range index inside the library, before any of this
// package's own bounds checks are reached. Converting that into an error is
// the only defence available short of forking the reader.
//
// The panic value is preserved rather than discarded, so a library fault stays
// reportable as a library fault.
type CorruptHeaderError struct {
	// Site is the fully qualified name of the function that faulted.
	Site string
	// Value is the recovered panic value.
	Value any
}

func (e *CorruptHeaderError) Error() string {
	return fmt.Sprintf("header is not a decodable FlatBuffer: %v (in %s)", e.Value, e.Site)
}

// CorruptFeatureError reports a FlatGeobuf feature whose FlatBuffers encoding
// made the reader library fault instead of returning an error. See
// CorruptHeaderError for why this class of input faults rather than erroring.
type CorruptFeatureError struct {
	// Index is the position of the offending feature in the data section.
	Index int
	// Site is the fully qualified name of the function that faulted.
	Site string
	// Value is the recovered panic value.
	Value any
}

func (e *CorruptFeatureError) Error() string {
	return fmt.Sprintf("feature %d is not a decodable FlatBuffer: %v (in %s)", e.Index, e.Value, e.Site)
}

// headerFields holds the header values this package reads. They are pulled out
// in one guarded step so that no later code has to touch the header table.
type headerFields struct {
	geometryType  flat.GeometryType
	epsgCode      int
	featuresCount uint64
}

// readHeaderFields reads the header fields this package needs, turning a fault
// inside the FlatBuffers reader into a CorruptHeaderError.
func readHeaderFields(hdr *flat.Header) (fields headerFields, err error) {
	defer func() {
		rec := recover()
		if rec == nil {
			return
		}

		fields = headerFields{}
		err = &CorruptHeaderError{Site: libraryFaultSite(rec), Value: rec}
	}()

	return headerFields{
		geometryType:  hdr.GeometryType(),
		epsgCode:      extractHeaderCRS(hdr),
		featuresCount: hdr.FeaturesCount(),
	}, nil
}

// decodeFeature converts one feature, turning a fault inside the FlatBuffers
// reader into a CorruptFeatureError.
//
// The guard is deliberately per feature rather than around the whole read: it
// must not be able to absorb a fault raised anywhere else in the pipeline.
func decodeFeature(feat *flat.Feature, hdr *flat.Header, headerGeomType flat.GeometryType, index int) (result *modelgeojson.GeoJSONFeature, err error) {
	defer func() {
		rec := recover()
		if rec == nil {
			return
		}

		result = nil
		err = &CorruptFeatureError{Index: index, Site: libraryFaultSite(rec), Value: rec}
	}()

	return convertFeature(feat, hdr, headerGeomType, index)
}

// libraryFaultSite names the function that raised the panic being recovered,
// and re-panics if that function is not one of the reader libraries.
//
// This is what stops the guards from masking a bug in this repository's own
// code: only a fault raised inside github.com/gogama/flatgeobuf or
// github.com/google/flatbuffers is eligible to become an error. Anything else,
// including a stack this function cannot read, is re-panicked untouched.
func libraryFaultSite(rec any) string {
	site := panicSite()
	if site == "" || strings.HasPrefix(site, aconiqPackagePrefix) {
		panic(rec)
	}

	return site
}

// panicSite returns the fully qualified name of the function that raised the
// panic currently being recovered, or "" if it cannot be determined.
//
// Deferred functions run before their frames are unwound, so the stack visible
// here still contains the panicking frame. It sits below runtime.gopanic,
// behind however many runtime helpers raised it (runtime.goPanicIndex for a
// bad index, for example), so the first non-runtime frame past gopanic is the
// site. Scanning for gopanic rather than skipping a fixed number of frames
// keeps this independent of how deeply it is called. CallersFrames expands
// inlined calls, so a library accessor inlined into this package is still
// attributed to the library.
func panicSite() string {
	var pcs [64]uintptr

	n := runtime.Callers(1, pcs[:])
	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pcs[:n])
	seenGopanic := false

	for {
		frame, more := frames.Next()

		switch {
		case frame.Function == "runtime.gopanic":
			seenGopanic = true
		case seenGopanic && !strings.HasPrefix(frame.Function, "runtime."):
			return frame.Function
		}

		if !more {
			return ""
		}
	}
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
		return modelgeojson.GeometryTypePoint, decodePoint(geom), nil
	case flat.GeometryTypeLineString:
		return decodeWhole(modelgeojson.GeometryTypeLineString, geom, decodeLineString)
	case flat.GeometryTypePolygon:
		return decodeWhole(modelgeojson.GeometryTypePolygon, geom, decodePolygon)
	case flat.GeometryTypeMultiPoint:
		return decodeWhole(modelgeojson.GeometryTypeMultiPoint, geom, decodeMultiPoint)
	case flat.GeometryTypeMultiLineString:
		return decodeWhole(modelgeojson.GeometryTypeMultiLineString, geom, decodeMultiLineString)
	case flat.GeometryTypeMultiPolygon:
		return decodeWhole(modelgeojson.GeometryTypeMultiPolygon, geom, decodeMultiPolygon)
	default:
		return "", nil, fmt.Errorf("unsupported geometry type: %s", gt)
	}
}

// decodeWhole adapts a coordinate decoder to the (type, coords, error) shape
// geometryToGeoJSON returns.
func decodeWhole(geomType string, geom *flat.Geometry, decode func(*flat.Geometry) ([]any, error)) (string, any, error) {
	coords, err := decode(geom)
	if err != nil {
		return "", nil, err
	}

	return geomType, coords, nil
}

func decodeLineString(geom *flat.Geometry) ([]any, error) {
	total, err := xyPairCount(geom)
	if err != nil {
		return nil, err
	}

	return decodeCoordSequence(geom, 0, total)
}

func decodePoint(geom *flat.Geometry) []any {
	if geom.XyLength() < 2 {
		return nil
	}

	return []any{geom.Xy(0), geom.Xy(1)}
}

// decodeCoordSequence extracts a slice of [x, y] coordinate pairs from xy array.
// start and end are in coordinate-pair counts (not xy-index).
func decodeCoordSequence(geom *flat.Geometry, start, end int) ([]any, error) {
	total, err := xyPairCount(geom)
	if err != nil {
		return nil, err
	}

	// start and end derive from the file's ends vector, so the range has to be
	// validated against the coordinates actually present before it sizes an
	// allocation or indexes into the xy vector.
	if start < 0 || end < start || end > total {
		return nil, fmt.Errorf("coordinate range [%d,%d) outside the %d coordinate pairs available", start, end, total)
	}

	pts := make([]any, 0, end-start)

	for i := start; i < end; i++ {
		x := geom.Xy(i * 2)
		y := geom.Xy(i*2 + 1)
		pts = append(pts, []any{x, y})
	}

	return pts, nil
}

func decodePolygon(geom *flat.Geometry) ([]any, error) {
	numEnds, err := endsCount(geom)
	if err != nil {
		return nil, err
	}

	totalPairs, err := xyPairCount(geom)
	if err != nil {
		return nil, err
	}

	if numEnds == 0 {
		// Single ring: all coordinates form one ring.
		ring, ringErr := decodeCoordSequence(geom, 0, totalPairs)
		if ringErr != nil {
			return nil, ringErr
		}

		return []any{ring}, nil
	}

	rings := make([]any, 0, numEnds)
	start := 0

	for i := range numEnds {
		end := int(geom.Ends(i))

		ring, ringErr := decodeCoordSequence(geom, start, end)
		if ringErr != nil {
			return nil, fmt.Errorf("ring %d: %w", i, ringErr)
		}

		rings = append(rings, ring)
		start = end
	}

	return rings, nil
}

func decodeMultiPoint(geom *flat.Geometry) ([]any, error) {
	n, err := xyPairCount(geom)
	if err != nil {
		return nil, err
	}

	pts := make([]any, 0, n)

	for i := range n {
		pts = append(pts, []any{geom.Xy(i * 2), geom.Xy(i*2 + 1)})
	}

	return pts, nil
}

func decodeMultiLineString(geom *flat.Geometry) ([]any, error) {
	numParts, err := partsCount(geom)
	if err != nil {
		return nil, err
	}

	if numParts > 0 {
		return decodePartsCoords(geom)
	}

	// Use ends array to split coordinate sequences into linestrings.
	numEnds, err := endsCount(geom)
	if err != nil {
		return nil, err
	}

	if numEnds == 0 {
		line, lineErr := decodeLineString(geom)
		if lineErr != nil {
			return nil, lineErr
		}

		return []any{line}, nil
	}

	lines := make([]any, 0, numEnds)
	start := 0

	for i := range numEnds {
		end := int(geom.Ends(i))

		line, lineErr := decodeCoordSequence(geom, start, end)
		if lineErr != nil {
			return nil, fmt.Errorf("linestring %d: %w", i, lineErr)
		}

		lines = append(lines, line)
		start = end
	}

	return lines, nil
}

func decodeMultiPolygon(geom *flat.Geometry) ([]any, error) {
	n, err := partsCount(geom)
	if err != nil {
		return nil, err
	}

	if n == 0 {
		// Fallback: treat as single polygon.
		poly, polyErr := decodePolygon(geom)
		if polyErr != nil {
			return nil, polyErr
		}

		return []any{poly}, nil
	}

	polys := make([]any, 0, n)
	part := new(flat.Geometry)

	for i := range n {
		if !geom.Parts(part, i) {
			continue
		}

		poly, polyErr := decodePolygon(part)
		if polyErr != nil {
			return nil, fmt.Errorf("polygon %d: %w", i, polyErr)
		}

		polys = append(polys, poly)
	}

	return polys, nil
}

func decodePartsCoords(geom *flat.Geometry) ([]any, error) {
	n, err := partsCount(geom)
	if err != nil {
		return nil, err
	}

	parts := make([]any, 0, n)
	part := new(flat.Geometry)

	for i := range n {
		if !geom.Parts(part, i) {
			continue
		}

		coords, coordErr := decodeLineString(part)
		if coordErr != nil {
			return nil, fmt.Errorf("part %d: %w", i, coordErr)
		}

		parts = append(parts, coords)
	}

	return parts, nil
}

func readProperties(feat *flat.Feature, hdr *flat.Header) (map[string]any, error) {
	propBytes := feat.PropertiesBytes()
	if len(propBytes) == 0 {
		return make(map[string]any), nil
	}

	// The reader is kept alongside the PropReader so the length prefix of a
	// variable-length value can be inspected before PropReader allocates from
	// it. See checkVariableLengthValue.
	src := bytes.NewReader(propBytes)
	pr := flatgeobuf.NewPropReader(src)
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

		val, readErr := readOneProperty(pr, src, col.Type())
		if readErr != nil {
			return nil, fmt.Errorf("read property %q: %w", string(col.Name()), readErr)
		}

		props[string(col.Name())] = normalizeValue(val)
	}

	return props, nil
}

// variableLengthColumnTypes are the column types whose value is encoded as a
// little-endian uint32 byte count followed by that many bytes.
var variableLengthColumnTypes = map[flat.ColumnType]struct{}{
	flat.ColumnTypeString:   {},
	flat.ColumnTypeDateTime: {},
	flat.ColumnTypeJson:     {},
	flat.ColumnTypeBinary:   {},
}

// readOneProperty reads a single property value, bounding it first if its
// length comes from the file.
func readOneProperty(pr *flatgeobuf.PropReader, src *bytes.Reader, colType flat.ColumnType) (any, error) {
	if _, variable := variableLengthColumnTypes[colType]; variable {
		err := checkVariableLengthValue(src)
		if err != nil {
			return nil, err
		}
	}

	return readPropertyValue(pr, colType)
}

// checkVariableLengthValue rejects a property value longer than the property
// block that is supposed to contain it, leaving src positioned as it found it.
//
// PropReader.ReadBinary, which also backs ReadString, does
//
//	n, err := r.ReadUInt()
//	...
//	b := make([]byte, int(n))
//
// with no check of n against the bytes available, so four bytes inside a
// feature can ask for a 4 GiB buffer. src is a bytes.Reader over the whole
// property block, so the check is exact: read the prefix, compare, rewind.
func checkVariableLengthValue(src *bytes.Reader) error {
	var prefix [4]byte

	_, err := io.ReadFull(src, prefix[:])
	if err != nil {
		return fmt.Errorf("read value length: %w", err)
	}

	declared := int64(binary.LittleEndian.Uint32(prefix[:]))

	remaining := int64(src.Len())

	_, err = src.Seek(-int64(len(prefix)), io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("rewind to value length: %w", err)
	}

	if declared > remaining {
		return fmt.Errorf("value declares %d bytes, more than the %d bytes left in the property block", declared, remaining)
	}

	return nil
}

// propertyReaders maps each FlatGeobuf column type to the reader function and
// human-readable name used to build error context in readPropertyValue.
var propertyReaders = map[flat.ColumnType]struct {
	name string
	read func(*flatgeobuf.PropReader) (any, error)
}{
	flat.ColumnTypeByte:     {"byte", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadByte() }},
	flat.ColumnTypeUByte:    {"ubyte", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadUByte() }},
	flat.ColumnTypeBool:     {"bool", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadBool() }},
	flat.ColumnTypeShort:    {"short", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadShort() }},
	flat.ColumnTypeUShort:   {"ushort", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadUShort() }},
	flat.ColumnTypeInt:      {"int", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadInt() }},
	flat.ColumnTypeUInt:     {"uint", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadUInt() }},
	flat.ColumnTypeLong:     {"long", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadLong() }},
	flat.ColumnTypeULong:    {"ulong", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadULong() }},
	flat.ColumnTypeFloat:    {"float", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadFloat() }},
	flat.ColumnTypeDouble:   {"double", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadDouble() }},
	flat.ColumnTypeString:   {"string", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadString() }},
	flat.ColumnTypeDateTime: {"datetime", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadString() }},
	flat.ColumnTypeJson:     {"json", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadBinary() }},
	flat.ColumnTypeBinary:   {"binary", func(pr *flatgeobuf.PropReader) (any, error) { return pr.ReadBinary() }},
}

func readPropertyValue(pr *flatgeobuf.PropReader, colType flat.ColumnType) (any, error) {
	reader, ok := propertyReaders[colType]
	if !ok {
		return nil, fmt.Errorf("unsupported column type %d", colType)
	}

	v, err := reader.read(pr)
	if err != nil {
		return v, fmt.Errorf("read %s property: %w", reader.name, err)
	}

	return v, nil
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
