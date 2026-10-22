package export

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// readShape is a test-only Shapefile reader, written from the ESRI
// specification rather than from the writer, so the two can disagree.
type readShape struct {
	shapeType ShapeType
	bbox      [4]float64
	records   [][][][2]float64 // record → part → vertex
}

func readShapefile(t *testing.T, base string) readShape {
	t.Helper()

	shp := mustRead(t, base+".shp")
	shx := mustRead(t, base+".shx")

	if got := int(binary.BigEndian.Uint32(shp[0:4])); got != 9994 {
		t.Fatalf("file code %d, want 9994", got)
	}

	if got := int(binary.BigEndian.Uint32(shp[24:28])) * 2; got != len(shp) {
		t.Fatalf(".shp header declares %d bytes, file has %d", got, len(shp))
	}

	if got := int(binary.BigEndian.Uint32(shx[24:28])) * 2; got != len(shx) {
		t.Fatalf(".shx header declares %d bytes, file has %d", got, len(shx))
	}

	if got := binary.LittleEndian.Uint32(shp[28:32]); got != 1000 {
		t.Fatalf("version %d, want 1000", got)
	}

	if !bytes.Equal(shp[32:100], shx[32:100]) {
		t.Fatal(".shp and .shx headers disagree on shape type or bbox")
	}

	out := readShape{shapeType: ShapeType(binary.LittleEndian.Uint32(shp[32:36]))}
	for i := range 4 {
		out.bbox[i] = leFloat(shp[36+8*i:])
	}

	pos := 100

	for n := 1; pos < len(shp); n++ {
		if got := int(binary.BigEndian.Uint32(shp[pos:])); got != n {
			t.Fatalf("record %d numbered %d", n, got)
		}

		contentLen := int(binary.BigEndian.Uint32(shp[pos+4:])) * 2

		idx := 100 + 8*(n-1)
		if got := int(binary.BigEndian.Uint32(shx[idx:])) * 2; got != pos {
			t.Fatalf(".shx offset of record %d is %d, record sits at %d", n, got, pos)
		}

		if got := int(binary.BigEndian.Uint32(shx[idx+4:])) * 2; got != contentLen {
			t.Fatalf(".shx length of record %d is %d, record is %d", n, got, contentLen)
		}

		out.records = append(out.records, decodeShapeContent(t, shp[pos+8:pos+8+contentLen]))
		pos += 8 + contentLen
	}

	if want := (len(shx) - 100) / 8; len(out.records) != want {
		t.Fatalf(".shp has %d records, .shx indexes %d", len(out.records), want)
	}

	return out
}

func decodeShapeContent(t *testing.T, c []byte) [][][2]float64 {
	t.Helper()

	switch ShapeType(binary.LittleEndian.Uint32(c)) {
	case ShapePoint:
		return [][][2]float64{{{leFloat(c[4:]), leFloat(c[12:])}}}
	case ShapePolyLine, ShapePolygon:
		numParts := int(binary.LittleEndian.Uint32(c[36:]))
		numPoints := int(binary.LittleEndian.Uint32(c[40:]))

		starts := make([]int, numParts+1)
		for i := range numParts {
			starts[i] = int(binary.LittleEndian.Uint32(c[44+4*i:]))
		}

		starts[numParts] = numPoints
		pointsAt := 44 + 4*numParts

		parts := make([][][2]float64, numParts)
		for p := range numParts {
			for v := starts[p]; v < starts[p+1]; v++ {
				parts[p] = append(parts[p], [2]float64{leFloat(c[pointsAt+16*v:]), leFloat(c[pointsAt+16*v+8:])})
			}
		}

		return parts
	default:
		t.Fatalf("unexpected record shape type %d", binary.LittleEndian.Uint32(c))

		return nil
	}
}

type readDBF struct {
	date   [3]byte
	fields []ShapeField
	widths []int
	rows   [][]string
}

func readDBFFile(t *testing.T, path string) readDBF {
	t.Helper()

	data := mustRead(t, path)

	if data[0] != 0x03 {
		t.Fatalf("dbf version byte 0x%02x, want 0x03", data[0])
	}

	numRecords := int(binary.LittleEndian.Uint32(data[4:]))
	headerLen := int(binary.LittleEndian.Uint16(data[8:]))
	recordLen := int(binary.LittleEndian.Uint16(data[10:]))

	out := readDBF{date: [3]byte{data[1], data[2], data[3]}}

	for pos := 32; data[pos] != 0x0D; pos += 32 {
		out.fields = append(out.fields, ShapeField{
			Name:     strings.TrimRight(string(data[pos:pos+11]), "\x00"),
			Kind:     FieldKind(data[pos+11]),
			Decimals: int(data[pos+17]),
		})
		out.widths = append(out.widths, int(data[pos+16]))
	}

	if want := 32 + 32*len(out.fields) + 1; headerLen != want {
		t.Fatalf("header length %d, want %d", headerLen, want)
	}

	sum := 1
	for _, w := range out.widths {
		sum += w
	}

	if recordLen != sum {
		t.Fatalf("record length %d, fields sum to %d", recordLen, sum)
	}

	if want := headerLen + numRecords*recordLen + 1; len(data) != want {
		t.Fatalf("dbf is %d bytes, want %d", len(data), want)
	}

	if data[len(data)-1] != 0x1A {
		t.Fatal("dbf does not end with 0x1A")
	}

	for r := range numRecords {
		rec := data[headerLen+r*recordLen : headerLen+(r+1)*recordLen]
		if rec[0] != ' ' {
			t.Fatalf("record %d is flagged deleted", r)
		}

		row := make([]string, 0, len(out.widths))
		pos := 1

		for _, w := range out.widths {
			row = append(row, strings.TrimSpace(string(rec[pos:pos+w])))
			pos += w
		}

		out.rows = append(out.rows, row)
	}

	return out
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func leFloat(b []byte) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(b)) }

func testDate() time.Time { return time.Date(2026, 10, 22, 7, 0, 0, 0, time.UTC) }

func TestWriteShapefileRoundTripsPolygonsWithHoles(t *testing.T) {
	t.Parallel()

	// GeoJSON winding: outer counter-clockwise, hole clockwise.
	outer := [][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}}
	hole := [][2]float64{{2, 2}, {2, 4}, {4, 4}, {4, 2}, {2, 2}}
	second := [][2]float64{{20, 0}, {30, 0}, {30, 5}} // unclosed

	layer := ShapeLayer{
		Type:   ShapePolygon,
		Fields: []ShapeField{{Name: "ID", Kind: FieldChar}, {Name: "HEIGHT_M", Kind: FieldNumeric, Decimals: 3}},
		Records: []ShapeRecord{
			{Parts: OrientPolygonRings([][][][2]float64{{outer, hole}, {second}}), Values: []any{"b1", 12.5}},
		},
		PRJ:  "PROJCS[\"x\"]",
		Date: testDate(),
	}

	base := filepath.Join(t.TempDir(), "buildings")

	paths, err := WriteShapefile(base, layer)
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 5 {
		t.Fatalf("wrote %d files, want .shp .shx .dbf .cpg .prj", len(paths))
	}

	got := readShapefile(t, base)
	if got.shapeType != ShapePolygon || len(got.records) != 1 {
		t.Fatalf("got type %d with %d records", got.shapeType, len(got.records))
	}

	if got.bbox != [4]float64{0, 0, 30, 10} {
		t.Fatalf("file bbox %v", got.bbox)
	}

	parts := got.records[0]
	if len(parts) != 3 {
		t.Fatalf("got %d parts, want 3", len(parts))
	}

	for i, wantClockwise := range []bool{true, false, true} {
		if clockwise := signedArea(parts[i]) < 0; clockwise != wantClockwise {
			t.Errorf("part %d clockwise=%v, want %v", i, clockwise, wantClockwise)
		}

		if parts[i][0] != parts[i][len(parts[i])-1] {
			t.Errorf("part %d is not closed", i)
		}
	}

	dbf := readDBFFile(t, base+".dbf")
	if dbf.date != [3]byte{126, 10, 22} {
		t.Errorf("dbf date %v, want 2026-10-22", dbf.date)
	}

	if dbf.fields[1] != (ShapeField{Name: "HEIGHT_M", Kind: FieldNumeric, Decimals: 3}) {
		t.Errorf("field %+v", dbf.fields[1])
	}

	if strings.Join(dbf.rows[0], "|") != "b1|12.500" {
		t.Errorf("row %v", dbf.rows[0])
	}

	if cpg := string(mustRead(t, base+".cpg")); cpg != "UTF-8" {
		t.Errorf(".cpg %q", cpg)
	}
}

func TestWriteShapefileWritesPointsLinesAndAllFieldKinds(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	points := ShapeLayer{
		Type: ShapePoint,
		Fields: []ShapeField{
			{Name: "ID", Kind: FieldChar},
			{Name: "LW_DB", Kind: FieldNumeric, Decimals: 2},
			{Name: "ACTIVE", Kind: FieldLogical},
		},
		Records: []ShapeRecord{
			{Parts: [][][2]float64{{{500000.25, 5800000.5}}}, Values: []any{"Größe", 98.456, true}},
			{Parts: [][][2]float64{{{500010, 5800010}}}, Values: []any{nil, nil, nil}},
		},
		Date: testDate(),
	}

	if _, err := WriteShapefile(filepath.Join(dir, "p"), points); err != nil {
		t.Fatal(err)
	}

	got := readShapefile(t, filepath.Join(dir, "p"))
	if got.records[0][0][0] != [2]float64{500000.25, 5800000.5} {
		t.Errorf("point %v", got.records[0][0][0])
	}

	if _, err := os.Stat(filepath.Join(dir, "p.prj")); !os.IsNotExist(err) {
		t.Error("a layer without PRJ wrote a .prj")
	}

	dbf := readDBFFile(t, filepath.Join(dir, "p.dbf"))

	// Rounded at the declared decimals, and UTF-8 bytes counted as bytes.
	if got := strings.Join(dbf.rows[0], "|"); got != "Größe|98.46|T" {
		t.Errorf("row 0 %q", got)
	}

	if got := strings.Join(dbf.rows[1], "|"); got != "||?" {
		t.Errorf("row 1 %q", got)
	}

	if dbf.widths[0] != len("Größe") {
		t.Errorf("char width %d, want the byte length %d", dbf.widths[0], len("Größe"))
	}

	lines := ShapeLayer{
		Type:   ShapePolyLine,
		Fields: []ShapeField{{Name: "ID", Kind: FieldChar}},
		Records: []ShapeRecord{
			{Parts: [][][2]float64{{{0, 0}, {5, 5}}, {{10, 0}, {10, 3}, {12, 3}}}, Values: []any{"w1"}},
		},
		Date: testDate(),
	}

	if _, err := WriteShapefile(filepath.Join(dir, "l"), lines); err != nil {
		t.Fatal(err)
	}

	gotLines := readShapefile(t, filepath.Join(dir, "l"))
	if len(gotLines.records[0]) != 2 || len(gotLines.records[0][1]) != 3 {
		t.Errorf("multipart line %v", gotLines.records[0])
	}
}

func TestWriteShapefileIsByteIdentical(t *testing.T) {
	t.Parallel()

	layer := ShapeLayer{
		Type:    ShapePoint,
		Fields:  []ShapeField{{Name: "ID", Kind: FieldChar}},
		Records: []ShapeRecord{{Parts: [][][2]float64{{{1, 2}}}, Values: []any{"a"}}},
		Date:    testDate(),
	}

	a, b := filepath.Join(t.TempDir(), "x"), filepath.Join(t.TempDir(), "x")

	for _, base := range []string{a, b} {
		if _, err := WriteShapefile(base, layer); err != nil {
			t.Fatal(err)
		}
	}

	for _, ext := range []string{".shp", ".shx", ".dbf"} {
		if !bytes.Equal(mustRead(t, a+ext), mustRead(t, b+ext)) {
			t.Errorf("%s differs between two writes of one layer", ext)
		}
	}
}

func TestWriteShapefileRefusesWhatDBaseCannotHold(t *testing.T) {
	t.Parallel()

	cases := map[string]ShapeLayer{
		"long name": {Type: ShapePoint, Fields: []ShapeField{{Name: "ELEVENCHARS", Kind: FieldChar}}},
		"duplicate": {Type: ShapePoint, Fields: []ShapeField{{Name: "A", Kind: FieldChar}, {Name: "a", Kind: FieldChar}}},
		"long text": {
			Type: ShapePoint, Fields: []ShapeField{{Name: "T", Kind: FieldChar}},
			Records: []ShapeRecord{{Parts: [][][2]float64{{{0, 0}}}, Values: []any{strings.Repeat("x", 255)}}},
		},
		"wide number": {
			Type: ShapePoint, Fields: []ShapeField{{Name: "N", Kind: FieldNumeric, Decimals: 6}},
			Records: []ShapeRecord{{Parts: [][][2]float64{{{0, 0}}}, Values: []any{1e15}}},
		},
		"short ring": {
			Type: ShapePolygon, Fields: []ShapeField{{Name: "ID", Kind: FieldChar}},
			Records: []ShapeRecord{{Parts: [][][2]float64{{{0, 0}, {1, 0}, {0, 0}}}, Values: []any{"r"}}},
		},
		"non-finite": {
			Type: ShapePoint, Fields: []ShapeField{{Name: "ID", Kind: FieldChar}},
			Records: []ShapeRecord{{Parts: [][][2]float64{{{math.NaN(), 0}}}, Values: []any{"n"}}},
		},
	}

	for name, layer := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := WriteShapefile(filepath.Join(t.TempDir(), "x"), layer); err == nil {
				t.Fatal("expected a refusal")
			}
		})
	}
}

func TestAllocateFieldNameIsShortUniqueAndStable(t *testing.T) {
	t.Parallel()

	used := map[string]bool{"ID": true}

	keys := []string{"rls19_traffic_day", "rls19_traffic_night", "rls19_traffic_evening", "id", "1st", "ä-ü"}

	got := make([]string, 0, len(keys))
	for _, key := range keys {
		got = append(got, allocateFieldName(key, used))
	}

	want := []string{"RLS19_TRAF", "RLS19_TR_1", "RLS19_TR_2", "ID_1", "F_1ST", "F____"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func FuzzAllocateFieldName(f *testing.F) {
	for _, seed := range []string{"height_m", "ü", "", "aaaaaaaaaaaaaaa", "9"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, key string) {
		used := map[string]bool{}

		for i := range 12 {
			name := allocateFieldName(key, used)
			if len(name) == 0 || len(name) > MaxDBFFieldName || !isASCII(name) || name[0] < 'A' || name[0] > 'Z' {
				t.Fatalf("allocation %d of %q is %q", i, key, name)
			}
		}

		if len(used) != 12 {
			t.Fatalf("12 allocations of %q produced %d distinct names", key, len(used))
		}
	})
}

func TestESRIWKTForEPSG(t *testing.T) {
	t.Parallel()

	wkt, ok := ESRIWKTForEPSG(25832)
	if !ok || !strings.HasPrefix(wkt, `PROJCS["ETRS_1989_UTM_Zone_32N",GEOGCS["GCS_ETRS_1989"`) ||
		!strings.Contains(wkt, `PARAMETER["Central_Meridian",9.0]`) || !strings.Contains(wkt, `PARAMETER["Scale_Factor",0.9996]`) {
		t.Errorf("25832: %s", wkt)
	}

	wkt, ok = ESRIWKTForEPSG(31467)
	if !ok || !strings.Contains(wkt, `PARAMETER["False_Easting",3500000.0]`) || !strings.Contains(wkt, `"Bessel_1841"`) {
		t.Errorf("31467: %s", wkt)
	}

	for _, code := range []int{4326, 4258, 3857, 0} {
		if _, ok := ESRIWKTForEPSG(code); ok {
			t.Errorf("EPSG:%d must not be offered as a SoundPLAN plane", code)
		}
	}

	// Every projected code the geo package can transform into, save Web
	// Mercator, gets a .prj.
	for _, code := range []int{25831, 25833, 25834, 31466, 31468, 31469, 32632, 32633} {
		if _, ok := ESRIWKTForEPSG(code); !ok {
			t.Errorf("EPSG:%s has no WKT", strconv.Itoa(code))
		}
	}
}
