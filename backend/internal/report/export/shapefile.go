package export

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aconiq/backend/internal/atomicfile"
)

// ShapeType is an ESRI Shapefile shape type. Only the 2D types SoundPLAN's
// Geo-Database import reads are written: heights travel as attributes.
type ShapeType int32

// The shape types this writer emits.
const (
	ShapePoint    ShapeType = 1
	ShapePolyLine ShapeType = 3
	ShapePolygon  ShapeType = 5
)

// FieldKind is a dBASE field type.
type FieldKind byte

// The dBASE field types this writer emits.
const (
	FieldChar    FieldKind = 'C'
	FieldNumeric FieldKind = 'N'
	FieldLogical FieldKind = 'L'
)

// The dBASE III limits a field must fit in. A 'C' field holds 254 bytes at most
// and an 'N' field 20 characters; a name is 10 ASCII characters plus the NUL.
const (
	MaxDBFCharBytes    = 254
	maxDBFNumericWidth = 20
	MaxDBFFieldName    = 10
)

// ShapeField is one attribute column.
//
// Decimals applies to FieldNumeric only, and is the rounding boundary: a value
// is written with exactly that many fractional digits, and that is the one
// place this export rounds.
type ShapeField struct {
	Name     string
	Kind     FieldKind
	Decimals int
}

// ShapeRecord is one feature. Parts holds the vertices: one part with one
// vertex for a Point, one part per line or ring otherwise. Values align with
// the layer's fields and hold a string, a float64, a bool or nil (no value).
type ShapeRecord struct {
	Parts  [][][2]float64
	Values []any
}

// ShapeLayer is one Shapefile: a single shape type, a fixed column set.
type ShapeLayer struct {
	Type    ShapeType
	Fields  []ShapeField
	Records []ShapeRecord
	// PRJ is the ESRI WKT of the coordinate system; empty writes no .prj.
	PRJ string
	// Date is stamped into the .dbf header as its last-update date. It is a
	// parameter rather than time.Now so that one input exports byte-identical
	// files.
	Date time.Time
}

// WriteShapefile writes basePath + .shp, .shx, .dbf, .cpg and, when the layer
// carries one, .prj, and returns the paths it wrote in that order.
func WriteShapefile(basePath string, layer ShapeLayer) ([]string, error) {
	err := validateShapeLayer(layer)
	if err != nil {
		return nil, fmt.Errorf("shapefile %s: %w", filepath.Base(basePath), err)
	}

	shp, shx := encodeShapes(layer)

	dbf, err := encodeDBF(layer)
	if err != nil {
		return nil, fmt.Errorf("shapefile %s: %w", filepath.Base(basePath), err)
	}

	type sidecar struct {
		ext  string
		data []byte
	}

	files := []sidecar{
		{".shp", shp},
		{".shx", shx},
		{".dbf", dbf},
		// The .dbf header's language-driver byte has no code for UTF-8; the
		// .cpg sidecar is how GIS readers learn it.
		{".cpg", []byte("UTF-8")},
	}

	if layer.PRJ != "" {
		files = append(files, sidecar{".prj", []byte(layer.PRJ)})
	}

	paths := make([]string, 0, len(files))

	for _, f := range files {
		path := basePath + f.ext

		err := atomicfile.WriteFile(path, f.data)
		if err != nil {
			return nil, fmt.Errorf("write %s: %w", path, err)
		}

		paths = append(paths, path)
	}

	return paths, nil
}

func validateShapeLayer(layer ShapeLayer) error {
	switch layer.Type {
	case ShapePoint, ShapePolyLine, ShapePolygon:
	default:
		return fmt.Errorf("unsupported shape type %d", layer.Type)
	}

	seen := make(map[string]struct{}, len(layer.Fields))

	for _, f := range layer.Fields {
		if f.Name == "" || len(f.Name) > MaxDBFFieldName || !isASCII(f.Name) {
			return fmt.Errorf("field name %q is not 1-%d ASCII characters", f.Name, MaxDBFFieldName)
		}

		upper := strings.ToUpper(f.Name)
		if _, dup := seen[upper]; dup {
			return fmt.Errorf("field name %q appears twice", f.Name)
		}

		seen[upper] = struct{}{}
	}

	for i, rec := range layer.Records {
		if len(rec.Values) != len(layer.Fields) {
			return fmt.Errorf("record %d carries %d values for %d fields", i, len(rec.Values), len(layer.Fields))
		}

		err := validateParts(layer.Type, rec.Parts)
		if err != nil {
			return fmt.Errorf("record %d: %w", i, err)
		}
	}

	return nil
}

func validateParts(shapeType ShapeType, parts [][][2]float64) error {
	if len(parts) == 0 {
		return errors.New("has no geometry")
	}

	if shapeType == ShapePoint && (len(parts) != 1 || len(parts[0]) != 1) {
		return errors.New("a point record carries exactly one vertex")
	}

	minVertices := map[ShapeType]int{ShapePoint: 1, ShapePolyLine: 2, ShapePolygon: 4}[shapeType]

	for _, part := range parts {
		if len(part) < minVertices {
			return fmt.Errorf("a part has %d vertices, fewer than the %d this shape type needs", len(part), minVertices)
		}

		for _, v := range part {
			if !isFinite(v[0]) || !isFinite(v[1]) {
				return fmt.Errorf("vertex (%v, %v) is not finite", v[0], v[1])
			}
		}
	}

	return nil
}

// shpHeaderBytes is the size of the .shp and .shx file headers.
const shpHeaderBytes = 100

// encodeShapes builds the .shp and its .shx index together, since the index is
// the offsets of the records the main file is made of.
func encodeShapes(layer ShapeLayer) ([]byte, []byte) {
	contents := make([][]byte, len(layer.Records))
	bbox := emptyBBox()

	for i, rec := range layer.Records {
		contents[i] = encodeShapeContent(layer.Type, rec.Parts)

		for _, part := range rec.Parts {
			for _, v := range part {
				bbox.extend(v)
			}
		}
	}

	if len(layer.Records) == 0 {
		bbox = shapeBBox{}
	}

	shpLen := shpHeaderBytes
	for _, c := range contents {
		shpLen += 8 + len(c)
	}

	shxLen := shpHeaderBytes + 8*len(contents)

	var shp, shx bytes.Buffer

	writeShapeHeader(&shp, shpLen, layer.Type, bbox)
	writeShapeHeader(&shx, shxLen, layer.Type, bbox)

	offset := shpHeaderBytes

	// Offsets and lengths are counted in 16-bit words and big-endian; the
	// record body is little-endian. That mix is the format's, not ours.
	for i, c := range contents {
		writeBE32(&shx, offset/2)
		writeBE32(&shx, len(c)/2)
		writeBE32(&shp, i+1)
		writeBE32(&shp, len(c)/2)
		shp.Write(c)

		offset += 8 + len(c)
	}

	return shp.Bytes(), shx.Bytes()
}

func encodeShapeContent(shapeType ShapeType, parts [][][2]float64) []byte {
	var buf bytes.Buffer

	writeLE32(&buf, int(shapeType))

	if shapeType == ShapePoint {
		writeLEFloat(&buf, parts[0][0][0])
		writeLEFloat(&buf, parts[0][0][1])

		return buf.Bytes()
	}

	bbox := emptyBBox()
	numPoints := 0

	for _, part := range parts {
		numPoints += len(part)

		for _, v := range part {
			bbox.extend(v)
		}
	}

	for _, v := range []float64{bbox.minX, bbox.minY, bbox.maxX, bbox.maxY} {
		writeLEFloat(&buf, v)
	}

	writeLE32(&buf, len(parts))
	writeLE32(&buf, numPoints)

	start := 0
	for _, part := range parts {
		writeLE32(&buf, start)
		start += len(part)
	}

	for _, part := range parts {
		for _, v := range part {
			writeLEFloat(&buf, v[0])
			writeLEFloat(&buf, v[1])
		}
	}

	return buf.Bytes()
}

func writeShapeHeader(buf *bytes.Buffer, fileBytes int, shapeType ShapeType, bbox shapeBBox) {
	const (
		fileCode = 9994
		version  = 1000
	)

	writeBE32(buf, fileCode)

	for range 5 {
		writeBE32(buf, 0)
	}

	writeBE32(buf, fileBytes/2)
	writeLE32(buf, version)
	writeLE32(buf, int(shapeType))

	// Xmin, Ymin, Xmax, Ymax, then the Z and M ranges a 2D file leaves zero.
	for _, v := range []float64{bbox.minX, bbox.minY, bbox.maxX, bbox.maxY, 0, 0, 0, 0} {
		writeLEFloat(buf, v)
	}
}

type shapeBBox struct{ minX, minY, maxX, maxY float64 }

func emptyBBox() shapeBBox {
	return shapeBBox{minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
}

func (b *shapeBBox) extend(v [2]float64) {
	b.minX = math.Min(b.minX, v[0])
	b.minY = math.Min(b.minY, v[1])
	b.maxX = math.Max(b.maxX, v[0])
	b.maxY = math.Max(b.maxY, v[1])
}

// dbfHeaderBytes and dbfDescriptorBytes are the fixed dBASE III header and the
// size of one field descriptor in it.
const (
	dbfHeaderBytes     = 32
	dbfDescriptorBytes = 32
)

// encodeDBF writes a dBASE III table. Every cell is formatted first, because a
// field's width is the widest value it has to hold.
func encodeDBF(layer ShapeLayer) ([]byte, error) {
	cells, widths, err := formatDBFCells(layer)
	if err != nil {
		return nil, err
	}

	recordLen := 1
	for _, w := range widths {
		recordLen += w
	}

	headerLen := dbfHeaderBytes + dbfDescriptorBytes*len(layer.Fields) + 1
	if headerLen > math.MaxUint16 || recordLen > math.MaxUint16 {
		return nil, fmt.Errorf("%d fields do not fit a dBASE header", len(layer.Fields))
	}

	var buf bytes.Buffer

	date := layer.Date.UTC()

	buf.WriteByte(0x03)                           // dBASE III, no memo
	buf.WriteByte(byte(date.Year() - 1900))       //nolint:gosec // dBASE stores the year as an offset from 1900
	buf.WriteByte(byte(date.Month()))             //nolint:gosec // 1..12
	buf.WriteByte(byte(date.Day()))               //nolint:gosec // 1..31
	writeLEUint(&buf, uint32(len(layer.Records))) //nolint:gosec // a record count
	_ = binary.Write(&buf, binary.LittleEndian, uint16(headerLen))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(recordLen))
	buf.Write(make([]byte, 20))

	for c, f := range layer.Fields {
		name := make([]byte, MaxDBFFieldName+1)
		copy(name, strings.ToUpper(f.Name))
		buf.Write(name)
		buf.WriteByte(byte(f.Kind))
		buf.Write(make([]byte, 4))
		buf.WriteByte(byte(widths[c])) //nolint:gosec // at most MaxDBFCharBytes, enforced by formatDBFCells

		decimals := 0
		if f.Kind == FieldNumeric {
			decimals = f.Decimals
		}

		buf.WriteByte(byte(decimals))
		buf.Write(make([]byte, 14))
	}

	buf.WriteByte(0x0D) // header terminator

	for _, row := range cells {
		buf.WriteByte(' ') // not deleted

		for c, f := range layer.Fields {
			pad := strings.Repeat(" ", widths[c]-len(row[c]))

			if f.Kind == FieldNumeric {
				buf.WriteString(pad + row[c])
			} else {
				buf.WriteString(row[c] + pad)
			}
		}
	}

	buf.WriteByte(0x1A) // end of file

	return buf.Bytes(), nil
}

func formatDBFCells(layer ShapeLayer) ([][]string, []int, error) {
	cells := make([][]string, len(layer.Records))
	widths := make([]int, len(layer.Fields))

	for c, f := range layer.Fields {
		widths[c] = 1
		if f.Kind == FieldNumeric {
			// Wide enough for the declared decimals even when every value
			// is blank.
			widths[c] = f.Decimals + 2
		}
	}

	for r, rec := range layer.Records {
		cells[r] = make([]string, len(layer.Fields))

		for c, f := range layer.Fields {
			text, err := formatDBFValue(f, rec.Values[c])
			if err != nil {
				return nil, nil, fmt.Errorf("record %d field %s: %w", r, f.Name, err)
			}

			cells[r][c] = text
			widths[c] = max(widths[c], len(text))
		}
	}

	for c, f := range layer.Fields {
		if f.Kind == FieldNumeric && widths[c] > maxDBFNumericWidth {
			return nil, nil, fmt.Errorf("field %s needs %d characters, more than a numeric field holds (%d)",
				f.Name, widths[c], maxDBFNumericWidth)
		}
	}

	return cells, widths, nil
}

func formatDBFValue(f ShapeField, value any) (string, error) {
	if value == nil {
		if f.Kind == FieldLogical {
			return "?", nil
		}

		return "", nil
	}

	switch f.Kind {
	case FieldChar:
		s, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("value %v is not a string", value)
		}

		if !utf8.ValidString(s) {
			return "", errors.New("value is not valid UTF-8")
		}

		if len(s) > MaxDBFCharBytes {
			return "", fmt.Errorf("value is %d bytes, more than a character field holds (%d)", len(s), MaxDBFCharBytes)
		}

		return s, nil
	case FieldNumeric:
		v, ok := value.(float64)
		if !ok {
			return "", fmt.Errorf("value %v is not a number", value)
		}

		if !isFinite(v) {
			return "", fmt.Errorf("value %v is not finite", v)
		}

		return strconv.FormatFloat(v, 'f', f.Decimals, 64), nil
	case FieldLogical:
		b, ok := value.(bool)
		if !ok {
			return "", fmt.Errorf("value %v is not a boolean", value)
		}

		if b {
			return "T", nil
		}

		return "F", nil
	default:
		return "", fmt.Errorf("unsupported field kind %q", f.Kind)
	}
}

// OrientPolygonRings returns the rings of polygons as Shapefile parts: each
// polygon's outer ring clockwise and its holes counter-clockwise, every ring
// closed.
//
// GeoJSON (RFC 7946 §3.1.6) prescribes the opposite winding, so the rings of a
// model polygon are reversed rather than copied. A reader that trusts the
// winding — which the Shapefile specification entitles it to — would otherwise
// read every building as a hole.
func OrientPolygonRings(polygons [][][][2]float64) [][][2]float64 {
	parts := make([][][2]float64, 0)

	for _, polygon := range polygons {
		for i, ring := range polygon {
			ring = closeRing(ring)
			clockwise := signedArea(ring) < 0

			if (i == 0) != clockwise {
				ring = reversedRing(ring)
			}

			parts = append(parts, ring)
		}
	}

	return parts
}

func closeRing(ring [][2]float64) [][2]float64 {
	if len(ring) == 0 || ring[0] == ring[len(ring)-1] {
		return ring
	}

	closed := make([][2]float64, len(ring), len(ring)+1)
	copy(closed, ring)

	return append(closed, ring[0])
}

// signedArea is the shoelace sum: positive for a counter-clockwise ring.
func signedArea(ring [][2]float64) float64 {
	var sum float64

	for i := 0; i+1 < len(ring); i++ {
		sum += ring[i][0]*ring[i+1][1] - ring[i+1][0]*ring[i][1]
	}

	return sum / 2
}

func reversedRing(ring [][2]float64) [][2]float64 {
	out := make([][2]float64, len(ring))
	for i, v := range ring {
		out[len(ring)-1-i] = v
	}

	return out
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}

	return true
}

// The Shapefile's integers are int32. Every value passed here is a count, an
// offset or a length bounded by the file the writer is building, so the
// narrowing cannot overflow for any file the format can represent.
func writeBE32(buf *bytes.Buffer, v int) {
	_ = binary.Write(buf, binary.BigEndian, int32(v)) //nolint:gosec // bounded, see above
}

func writeLE32(buf *bytes.Buffer, v int) {
	_ = binary.Write(buf, binary.LittleEndian, int32(v)) //nolint:gosec // bounded, see above
}

func writeLEUint(buf *bytes.Buffer, v uint32) {
	_ = binary.Write(buf, binary.LittleEndian, v)
}

func writeLEFloat(buf *bytes.Buffer, v float64) {
	_ = binary.Write(buf, binary.LittleEndian, v)
}
