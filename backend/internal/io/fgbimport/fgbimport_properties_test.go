package fgbimport

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/gogama/flatgeobuf/flatgeobuf"
	"github.com/gogama/flatgeobuf/flatgeobuf/flat"
)

// propBlock encodes one property value for the column at index, using the
// writer that matches the column type. encodeProps in fgbimport_test.go covers
// only the handful of types its fixtures use.
func propBlock(t *testing.T, index uint16, colType flat.ColumnType, value any) []byte {
	t.Helper()

	var buf bytes.Buffer

	pw := flatgeobuf.NewPropWriter(&buf)

	_, err := pw.WriteUShort(index)
	if err != nil {
		t.Fatalf("write column index: %v", err)
	}

	switch colType {
	case flat.ColumnTypeByte:
		_, err = pw.WriteByte(value.(int8))
	case flat.ColumnTypeUByte:
		_, err = pw.WriteUByte(value.(uint8))
	case flat.ColumnTypeBool:
		_, err = pw.WriteBool(value.(bool))
	case flat.ColumnTypeShort:
		_, err = pw.WriteShort(value.(int16))
	case flat.ColumnTypeUShort:
		_, err = pw.WriteUShort(value.(uint16))
	case flat.ColumnTypeInt:
		_, err = pw.WriteInt(value.(int32))
	case flat.ColumnTypeUInt:
		_, err = pw.WriteUInt(value.(uint32))
	case flat.ColumnTypeLong:
		_, err = pw.WriteLong(value.(int64))
	case flat.ColumnTypeULong:
		_, err = pw.WriteULong(value.(uint64))
	case flat.ColumnTypeFloat:
		_, err = pw.WriteFloat(value.(float32))
	case flat.ColumnTypeDouble:
		_, err = pw.WriteDouble(value.(float64))
	case flat.ColumnTypeString, flat.ColumnTypeDateTime:
		_, err = pw.WriteString(value.(string))
	case flat.ColumnTypeJson, flat.ColumnTypeBinary:
		_, err = pw.WriteBinary(value.([]byte))
	default:
		t.Fatalf("propBlock does not know column type %d", colType)
	}

	if err != nil {
		t.Fatalf("write property value: %v", err)
	}

	return buf.Bytes()
}

// readOneColumn runs a single-column property block through readProperties.
func readOneColumn(t *testing.T, colType flat.ColumnType, props []byte) (map[string]any, error) {
	t.Helper()

	columns := []testColumn{{name: "value", colType: colType}}
	feat := buildFeature(flat.GeometryTypePoint, []float64{1, 2}, nil, props)
	hdr := buildHeader(flat.GeometryTypePoint, columns, 1)

	return readProperties(&feat, hdr)
}

// Every integer column type is promoted to float64 so the value survives a JSON
// round-trip into the normalized model. A type left unpromoted reaches the
// GeoJSON encoder as an int64 and comes back as something the schema check does
// not recognise — and a wrong-width reader turns a negative value into a huge
// positive one.
func TestReadPropertiesPromotesEveryNumericColumnToFloat64(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		colType flat.ColumnType
		value   any
		want    float64
	}{
		{name: "byte", colType: flat.ColumnTypeByte, value: int8(-128), want: -128},
		{name: "byte positive", colType: flat.ColumnTypeByte, value: int8(127), want: 127},
		{name: "ubyte", colType: flat.ColumnTypeUByte, value: uint8(255), want: 255},
		{name: "short", colType: flat.ColumnTypeShort, value: int16(-32768), want: -32768},
		{name: "ushort", colType: flat.ColumnTypeUShort, value: uint16(65535), want: 65535},
		{name: "int", colType: flat.ColumnTypeInt, value: int32(-2147483648), want: -2147483648},
		{name: "uint", colType: flat.ColumnTypeUInt, value: uint32(4294967295), want: 4294967295},
		{name: "long", colType: flat.ColumnTypeLong, value: int64(-1234567890123), want: -1234567890123},
		{name: "ulong", colType: flat.ColumnTypeULong, value: uint64(1234567890123), want: 1234567890123},
		{name: "float", colType: flat.ColumnTypeFloat, value: float32(12.5), want: 12.5},
		{name: "double", colType: flat.ColumnTypeDouble, value: 12.25, want: 12.25},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			props, err := readOneColumn(t, testCase.colType, propBlock(t, 0, testCase.colType, testCase.value))
			if err != nil {
				t.Fatalf("read properties: %v", err)
			}

			got, ok := props["value"].(float64)
			if !ok {
				t.Fatalf("value is %T (%v), want float64", props["value"], props["value"])
			}

			if got != testCase.want {
				t.Fatalf("value = %v, want %v", got, testCase.want)
			}
		})
	}
}

// Booleans and strings are the two types that must NOT be promoted: a bool
// turned into 1 would silently become a height, and a string turned into a
// number would lose the kind tag the model is keyed on.
func TestReadPropertiesKeepsBoolsAndStringsAsThemselves(t *testing.T) {
	t.Parallel()

	props, err := readOneColumn(t, flat.ColumnTypeBool, propBlock(t, 0, flat.ColumnTypeBool, true))
	if err != nil {
		t.Fatalf("read bool: %v", err)
	}

	if props["value"] != true {
		t.Fatalf("bool value = %#v, want true", props["value"])
	}

	props, err = readOneColumn(t, flat.ColumnTypeString, propBlock(t, 0, flat.ColumnTypeString, "Wohngebiet"))
	if err != nil {
		t.Fatalf("read string: %v", err)
	}

	if props["value"] != "Wohngebiet" {
		t.Fatalf("string value = %#v", props["value"])
	}
}

// Binary, JSON and DateTime all arrive as bytes and are rendered as text, so
// the GeoJSON they end up in stays encodable. A []byte left alone would be
// base64-encoded by encoding/json and no longer readable.
func TestReadPropertiesRendersByteColumnsAsText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		colType flat.ColumnType
		value   any
		want    string
	}{
		{name: "json", colType: flat.ColumnTypeJson, value: []byte(`{"a":1}`), want: `{"a":1}`},
		{name: "binary", colType: flat.ColumnTypeBinary, value: []byte("raw"), want: "raw"},
		{name: "datetime", colType: flat.ColumnTypeDateTime, value: "2024-01-02T03:04:05Z", want: "2024-01-02T03:04:05Z"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			props, err := readOneColumn(t, testCase.colType, propBlock(t, 0, testCase.colType, testCase.value))
			if err != nil {
				t.Fatalf("read properties: %v", err)
			}

			if props["value"] != testCase.want {
				t.Fatalf("value = %#v, want %q", props["value"], testCase.want)
			}
		})
	}
}

// A column index past the schema is either a corrupt file or a file whose
// header and data disagree. Either way it must be refused: reading on would
// attribute one column's bytes to another column's name.
func TestReadPropertiesRefusesAColumnIndexOutsideTheSchema(t *testing.T) {
	t.Parallel()

	// One column declared, index 5 referenced.
	props := propBlock(t, 5, flat.ColumnTypeDouble, 1.5)

	_, err := readOneColumn(t, flat.ColumnTypeDouble, props)
	if err == nil {
		t.Fatal("expected an error for a column index past the schema")
	}

	if !strings.Contains(err.Error(), "exceeds schema") {
		t.Fatalf("error %q does not report the out-of-range column", err)
	}
}

// A property block that ends mid-value is truncation, not an end of block, and
// has to be reported rather than leaving the last property silently missing.
func TestReadPropertiesRefusesATruncatedValue(t *testing.T) {
	t.Parallel()

	full := propBlock(t, 0, flat.ColumnTypeDouble, 12.5)

	// Keep the column index and half the double.
	_, err := readOneColumn(t, flat.ColumnTypeDouble, full[:len(full)-4])
	if err == nil {
		t.Fatal("expected an error for a value cut in half")
	}

	if !strings.Contains(err.Error(), `read property "value"`) {
		t.Fatalf("error %q does not name the property", err)
	}
}

// PropReader.ReadBinary sizes make([]byte, n) straight from a four-byte length
// prefix with no check against the bytes available, so four bytes inside a
// feature could ask for a 4 GiB buffer. The length is bounded against the
// property block first; this is the regression test for that bound.
func TestReadPropertiesRefusesAValueLongerThanItsBlock(t *testing.T) {
	t.Parallel()

	for _, colType := range []flat.ColumnType{
		flat.ColumnTypeString,
		flat.ColumnTypeBinary,
		flat.ColumnTypeJson,
		flat.ColumnTypeDateTime,
	} {
		props := propBlock(t, 0, flat.ColumnTypeString, "short")

		// The length prefix follows the two-byte column index.
		lying := bytes.Clone(props)
		binary.LittleEndian.PutUint32(lying[2:], 0xFFFFFFF0)

		_, err := readOneColumn(t, colType, lying)
		if err == nil {
			t.Fatalf("%s: expected an error for a value longer than its block", colType)
		}

		if !strings.Contains(err.Error(), "more than the") {
			t.Fatalf("%s: error %q does not report the oversized value", colType, err)
		}
	}
}

// A length that exactly fills the remaining block is legal and must still read:
// the bound is "longer than the block", not "as long as".
func TestReadPropertiesAcceptsAValueThatFillsItsBlock(t *testing.T) {
	t.Parallel()

	props, err := readOneColumn(t, flat.ColumnTypeString, propBlock(t, 0, flat.ColumnTypeString, "exactly"))
	if err != nil {
		t.Fatalf("read properties: %v", err)
	}

	if props["value"] != "exactly" {
		t.Fatalf("value = %#v", props["value"])
	}
}

// A zero-length string is a value, not an absence.
func TestReadPropertiesAcceptsAnEmptyStringValue(t *testing.T) {
	t.Parallel()

	props, err := readOneColumn(t, flat.ColumnTypeString, propBlock(t, 0, flat.ColumnTypeString, ""))
	if err != nil {
		t.Fatalf("read properties: %v", err)
	}

	value, present := props["value"]
	if !present {
		t.Fatal("an empty string value did not appear in the properties")
	}

	if value != "" {
		t.Fatalf("value = %#v, want the empty string", value)
	}
}

// A feature with no property block at all yields an empty map, not nil, so a
// caller can index it without a check.
func TestReadPropertiesOnAFeatureWithoutProperties(t *testing.T) {
	t.Parallel()

	props, err := readOneColumn(t, flat.ColumnTypeString, nil)
	if err != nil {
		t.Fatalf("read properties: %v", err)
	}

	if props == nil {
		t.Fatal("a feature without properties yielded a nil map")
	}

	if len(props) != 0 {
		t.Fatalf("expected no properties, got %#v", props)
	}
}

// A column type outside the FlatGeobuf enum has no reader, and the value's
// width is unknown — so continuing would misalign every property after it.
func TestReadPropertiesRefusesAnUnknownColumnType(t *testing.T) {
	t.Parallel()

	// A well-formed index followed by eight bytes, under a column type the
	// format does not define.
	block := make([]byte, 0, 10)
	block = append(block, propBlock(t, 0, flat.ColumnTypeDouble, 1.5)[:2]...)
	block = append(block, make([]byte, 8)...)

	_, err := readOneColumn(t, flat.ColumnType(99), block)
	if err == nil {
		t.Fatal("expected an error for an undefined column type")
	}

	if !strings.Contains(err.Error(), "unsupported column type") {
		t.Fatalf("error %q does not report the unsupported type", err)
	}
}

// Several properties in one block must all arrive, keyed by their schema names.
func TestReadPropertiesReadsEveryColumnInOneBlock(t *testing.T) {
	t.Parallel()

	columns := []testColumn{
		{name: "kind", colType: flat.ColumnTypeString},
		{name: "height_m", colType: flat.ColumnTypeDouble},
		{name: "lanes", colType: flat.ColumnTypeShort},
	}

	block := make([]byte, 0, 32)

	block = append(block, propBlock(t, 0, flat.ColumnTypeString, "building")...)
	block = append(block, propBlock(t, 1, flat.ColumnTypeDouble, 12.5)...)
	block = append(block, propBlock(t, 2, flat.ColumnTypeShort, int16(2))...)

	feat := buildFeature(flat.GeometryTypePolygon, []float64{0, 0, 1, 0, 1, 1, 0, 0}, []uint32{4}, block)
	hdr := buildHeader(flat.GeometryTypePolygon, columns, 1)

	props, err := readProperties(&feat, hdr)
	if err != nil {
		t.Fatalf("read properties: %v", err)
	}

	if props["kind"] != "building" || props["height_m"] != 12.5 || props["lanes"] != float64(2) {
		t.Fatalf("properties = %#v", props)
	}
}

// --- Feature IDs ---

// The feature ID is what a later `aconiq compare` matches against, so the
// preferred key order and the rendering of each type have to be pinned. A
// float64 ID must not gain an exponent or a trailing ".0": both would stop
// matching the same ID written by another tool.
func TestExtractIDPrefersTheFirstKnownKeyAndFormatsIt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		props map[string]any
		index int
		want  string
	}{
		{name: "no id falls back to the index", props: map[string]any{"kind": "building"}, index: 7, want: "7"},
		{name: "fid wins over id", props: map[string]any{"fid": "a", "id": "b", "FID": "c", "ID": "d"}, want: "a"},
		{name: "id wins over FID", props: map[string]any{"id": "b", "FID": "c", "ID": "d"}, want: "b"},
		{name: "FID wins over ID", props: map[string]any{"FID": "c", "ID": "d"}, want: "c"},
		{name: "ID is used last", props: map[string]any{"ID": "d"}, want: "d"},
		{name: "numeric id", props: map[string]any{"fid": float64(42)}, want: "42"},
		{name: "large numeric id keeps its digits", props: map[string]any{"fid": float64(1234567890123)}, want: "1234567890123"},
		{name: "fractional numeric id", props: map[string]any{"fid": 1.5}, want: "1.5"},
		{name: "negative numeric id", props: map[string]any{"fid": float64(-3)}, want: "-3"},
		{name: "bool id", props: map[string]any{"fid": true}, want: "true"},
		{name: "nil id becomes empty", props: map[string]any{"fid": nil}, want: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := extractID(testCase.props, testCase.index)
			if got != testCase.want {
				t.Fatalf("extractID = %q, want %q", got, testCase.want)
			}
		})
	}
}

// A non-finite ID would be nonsense, but it must still render as text rather
// than take the formatter down.
func TestExtractIDHandlesNonFiniteNumbers(t *testing.T) {
	t.Parallel()

	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		got := extractID(map[string]any{"fid": value}, 0)
		if got == "" {
			t.Fatalf("extractID for %v produced an empty ID", value)
		}
	}
}

// The IDs a whole file produces have to be distinct even when the file declares
// none: the index fallback is what keeps two unlabelled features apart.
func TestReadAssignsDistinctIDsWhenTheFileDeclaresNone(t *testing.T) {
	t.Parallel()

	columns := []testColumn{{name: "kind", colType: flat.ColumnTypeString}}
	props := encodeProps(columns, []any{"building"})

	features := make([]flat.Feature, 0, 5)
	for i := range 5 {
		features = append(features, buildFeature(flat.GeometryTypePoint, []float64{float64(i), 47.3}, nil, props))
	}

	stream := buildTestFGBBytes(t, flat.GeometryTypePoint, columns, features)

	res, err := readAll(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}

	seen := make(map[any]struct{}, len(res.Collection.Features))

	for i, feat := range res.Collection.Features {
		if _, exists := seen[feat.ID]; exists {
			t.Fatalf("feature %d reused the ID %v", i, feat.ID)
		}

		seen[feat.ID] = struct{}{}
	}

	if len(seen) != 5 {
		t.Fatalf("got %d distinct IDs from 5 features", len(seen))
	}
}
