package soundplanimport

import (
	"encoding/binary"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

// Fixture-free tests for the RRLK*.GM grid-map reader.
//
// A .GM image is authored here from the three things gridmap.go declares about
// it: the header offsets parseGridMapGeometry reads, the "<name>|<unit>"
// descriptors parseGridMapLayers looks for among the null-terminated printable
// runs, and the 13-byte cell record (three float32s and a flag). No byte comes
// from the licensed project.

// gridMapCellRecordBytes builds one 13-byte cell record.
func gridMapCellRecordBytes(groundM float32, dayDB float32, nightDB float32, flag byte) []byte {
	rec := make([]byte, 13)
	copy(rec[0:4], leF32(groundM))
	copy(rec[4:8], leF32(dayDB))
	copy(rec[8:12], leF32(nightDB))
	rec[12] = flag

	return rec
}

// gridMapRowSeparator is the all-zero record that ends a row: its flag byte is
// zero, which is what splitGridMapNonZeroSpansFromPayload breaks a span on.
func gridMapRowSeparator() []byte {
	return make([]byte, 13)
}

// gridMapMarkerCell is the (-1, 0, 0) cell each row opens with.
func gridMapMarkerCell() []byte {
	return gridMapCellRecordBytes(-1, 0, 0, 1)
}

// gridMapHeader builds the fixed part of a .GM file: the geometry header at
// the offsets parseGridMapGeometry reads, followed by the layer descriptors.
//
// The result is padded to a multiple of the 13-byte cell width so that the
// cell stream that follows it starts on a record boundary, which is what
// detectGridMapCellStreamStart looks for.
func gridMapHeader(rowCount uint16, originX float64, originY float64, spacingX float32, spacingY float32, layers ...string) []byte {
	const headerFields = 57

	fields := make([]byte, headerFields)
	binary.BigEndian.PutUint16(fields[4:6], rowCount)
	copy(fields[9:17], leF64(originX))
	copy(fields[17:25], leF64(originY))
	copy(fields[49:53], leF32(spacingX))
	copy(fields[53:57], leF32(spacingY))

	// The layer block opens with a terminator of its own: the last spacing byte
	// is often printable, and without the break it would be read as the first
	// character of the first descriptor's name.
	block := make([]byte, 0, 64)
	block = append(block, 0x00)

	for _, layer := range layers {
		block = append(block, layer...)
		block = append(block, 0x00)
	}

	header := concat(fields, block)

	return concat(header, make([]byte, (13-len(header)%13)%13))
}

// syntheticGridMap builds a two-row grid map: each row is a marker cell
// followed by two value cells, and the rows are separated by a zero record.
func syntheticGridMap() []byte {
	return concat(
		gridMapHeader(2, 1000, 2000, 5, 5, "Boden|m", "Tag|dB(A)", "Nacht|dB(A)"),
		gridMapMarkerCell(),
		gridMapCellRecordBytes(200, 55, 45, 1),
		gridMapCellRecordBytes(210, 56, 46, 1),
		gridMapRowSeparator(),
		gridMapMarkerCell(),
		gridMapCellRecordBytes(220, 57, 47, 1),
		gridMapCellRecordBytes(230, 58, 48, 1),
		gridMapRowSeparator(),
	)
}

// TestParseGridMapMetadata_Synthetic pins the whole metadata read: the layer
// descriptors, the raster origin and spacing, and the per-band statistics.
//
// It also pins something that is easy to miss and is the shipped behaviour:
// the row marker cell is counted as an active cell and is fed to the
// statistics, which is why the ground band's minimum is the marker's -1 rather
// than the lowest terrain elevation in the raster.
func TestParseGridMapMetadata_Synthetic(t *testing.T) {
	t.Parallel()

	path := writeFixture(t, t.TempDir(), "RRLK0022.GM", syntheticGridMap())

	meta, err := ParseGridMapMetadata(path, 6)
	if err != nil {
		t.Fatalf("ParseGridMapMetadata: %v", err)
	}

	if meta.GMFile != "RRLK0022.GM" {
		t.Errorf("GMFile = %q", meta.GMFile)
	}

	if meta.FileSizeBytes <= 0 {
		t.Errorf("FileSizeBytes = %d, want the size on disk", meta.FileSizeBytes)
	}

	if len(meta.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", meta.Warnings)
	}

	if meta.OriginX != 1000 || meta.OriginY != 2000 || meta.SpacingX != 5 || meta.SpacingY != 5 {
		t.Errorf("geometry = %v/%v @ %v/%v, want 1000/2000 @ 5/5",
			meta.OriginX, meta.OriginY, meta.SpacingX, meta.SpacingY)
	}

	if meta.DeclaredRowCount != 2 {
		t.Errorf("DeclaredRowCount = %d, want 2", meta.DeclaredRowCount)
	}

	wantLayers := []GridMapLayer{{Name: "Boden", Unit: "m"}, {Name: "Tag", Unit: "dB(A)"}, {Name: "Nacht", Unit: "dB(A)"}}
	if len(meta.Layers) != len(wantLayers) {
		t.Fatalf("Layers = %+v, want %+v", meta.Layers, wantLayers)
	}

	for i, want := range wantLayers {
		if meta.Layers[i] != want {
			t.Errorf("Layers[%d] = %+v, want %+v", i, meta.Layers[i], want)
		}
	}

	if !meta.DecodedValues || meta.RowCount != 2 || meta.ActiveCellCount != 6 {
		t.Errorf("decode = %v, rows = %d, active cells = %d; want true/2/6",
			meta.DecodedValues, meta.RowCount, meta.ActiveCellCount)
	}

	if len(meta.RowCellCounts) != 2 || meta.RowCellCounts[0] != 3 || meta.RowCellCounts[1] != 3 {
		t.Errorf("RowCellCounts = %v, want [3 3]", meta.RowCellCounts)
	}

	// Sorted by layer name: Boden, Nacht, Tag.
	wantStats := []GridMapValueStats{
		{Name: "Boden", Unit: "m", Min: -1, Max: 230, Mean: 143, Count: 6},
		{Name: "Nacht", Unit: "dB(A)", Min: 0, Max: 48, Mean: 31, Count: 6},
		{Name: "Tag", Unit: "dB(A)", Min: 0, Max: 58, Mean: 226.0 / 6.0, Count: 6},
	}

	if len(meta.ValueStats) != len(wantStats) {
		t.Fatalf("ValueStats = %+v, want %+v", meta.ValueStats, wantStats)
	}

	for i, want := range wantStats {
		got := meta.ValueStats[i]
		if got.Name != want.Name || got.Unit != want.Unit || got.Count != want.Count {
			t.Errorf("ValueStats[%d] = %+v, want %+v", i, got, want)
		}

		if got.Min != want.Min || got.Max != want.Max || math.Abs(got.Mean-want.Mean) > 1e-9 {
			t.Errorf("ValueStats[%d] numbers = %+v, want %+v", i, got, want)
		}
	}
}

// TestParseGridMapMetadata_NoLayersIsRefused pins the one hard refusal: a file
// with no layer descriptor is not a grid map this parser can describe, and
// answering with an unlabelled raster would put unnamed dB values in a report.
func TestParseGridMapMetadata_NoLayersIsRefused(t *testing.T) {
	t.Parallel()

	image := concat(
		gridMapHeader(2, 1000, 2000, 5, 5),
		gridMapMarkerCell(),
		gridMapCellRecordBytes(200, 55, 45, 1),
		gridMapRowSeparator(),
	)

	_, err := ParseGridMapMetadata(writeFixture(t, t.TempDir(), "RRLK0022.GM", image), 2)
	if err == nil {
		t.Fatal("got nil error for a file with no layer descriptors")
	}

	if !strings.Contains(err.Error(), "no raster layer descriptors found") {
		t.Errorf("error = %q", err)
	}
}

// TestParseGridMapMetadata_GeometryIsWarnedAboutNotGuessed pins that an
// implausible header degrades to a warning with the layers still reported,
// rather than to an origin the parser made up.
func TestParseGridMapMetadata_GeometryIsWarnedAboutNotGuessed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header []byte
	}{
		{name: "row count zero", header: gridMapHeader(0, 1000, 2000, 5, 5, "Tag|dB(A)")},
		{name: "row count past the ceiling", header: gridMapHeader(20000, 1000, 2000, 5, 5, "Tag|dB(A)")},
		{name: "origin below the projected-metre floor", header: gridMapHeader(2, 999, 2000, 5, 5, "Tag|dB(A)")},
		{name: "northing below the projected-metre floor", header: gridMapHeader(2, 1000, 999, 5, 5, "Tag|dB(A)")},
		{name: "zero spacing", header: gridMapHeader(2, 1000, 2000, 0, 5, "Tag|dB(A)")},
		{name: "negative spacing", header: gridMapHeader(2, 1000, 2000, 5, -5, "Tag|dB(A)")},
		{name: "not-a-number origin", header: gridMapHeader(2, math.NaN(), 2000, 5, 5, "Tag|dB(A)")},
		{name: "infinite spacing", header: gridMapHeader(2, 1000, 2000, float32(math.Inf(1)), 5, "Tag|dB(A)")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			image := concat(tc.header, gridMapMarkerCell(), gridMapCellRecordBytes(200, 55, 45, 1), gridMapRowSeparator())

			meta, err := ParseGridMapMetadata(writeFixture(t, t.TempDir(), "RRLK0022.GM", image), 2)
			if err != nil {
				t.Fatalf("ParseGridMapMetadata: %v", err)
			}

			if meta.OriginX != 0 || meta.OriginY != 0 || meta.SpacingX != 0 || meta.DeclaredRowCount != 0 {
				t.Errorf("geometry was accepted: %+v", meta)
			}

			if len(meta.Warnings) == 0 || !strings.Contains(meta.Warnings[0], "no plausible origin/spacing header") {
				t.Errorf("warnings = %v, want one naming the rejected header", meta.Warnings)
			}

			if len(meta.Layers) != 1 {
				t.Errorf("Layers = %+v, want the descriptor to survive a bad geometry header", meta.Layers)
			}
		})
	}
}

// TestParseGridMapMetadata_UndecodableRowsBecomeAWarning pins that a
// points_total the cell stream cannot account for leaves DecodedValues false
// and the metadata otherwise intact. Reporting cell statistics from a stream
// that does not add up to the run's own point count would be a fabricated
// raster.
func TestParseGridMapMetadata_UndecodableRowsBecomeAWarning(t *testing.T) {
	t.Parallel()

	path := writeFixture(t, t.TempDir(), "RRLK0022.GM", syntheticGridMap())

	meta, err := ParseGridMapMetadata(path, 99)
	if err != nil {
		t.Fatalf("ParseGridMapMetadata: %v", err)
	}

	if meta.DecodedValues || meta.RowCount != 0 || len(meta.ValueStats) != 0 {
		t.Errorf("meta = %+v, want no decoded values", meta)
	}

	if len(meta.Warnings) != 1 || !strings.Contains(meta.Warnings[0], "could not match 99 points_total") {
		t.Errorf("warnings = %v, want one naming the mismatch", meta.Warnings)
	}

	if meta.OriginX != 1000 || len(meta.Layers) != 3 {
		t.Errorf("meta = %+v, want the header metadata to survive a failed row decode", meta)
	}
}

// TestParseGridMapMetadata_MissingFile pins the stat/read error path.
func TestParseGridMapMetadata_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := ParseGridMapMetadata(filepath.Join(t.TempDir(), "RRLK0022.GM"), 1)
	if err == nil {
		t.Fatal("got nil error for a missing .GM file")
	}

	if !strings.Contains(err.Error(), "stat GM") {
		t.Errorf("error = %q, want it to name the stat step", err)
	}
}

// TestParseDecodedGridMap_Synthetic pins the row-wise decode, and in
// particular that the leading marker cell is stripped from every row and
// counted separately instead of becoming a raster value of -1 metres.
func TestParseDecodedGridMap_Synthetic(t *testing.T) {
	t.Parallel()

	path := writeFixture(t, t.TempDir(), "RRLK0022.GM", syntheticGridMap())

	decoded, err := ParseDecodedGridMap(path, 6)
	if err != nil {
		t.Fatalf("ParseDecodedGridMap: %v", err)
	}

	if decoded.MarkerCellCount != 2 || decoded.ValueCellCount != 4 {
		t.Errorf("marker/value cells = %d/%d, want 2/4", decoded.MarkerCellCount, decoded.ValueCellCount)
	}

	if len(decoded.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(decoded.Rows))
	}

	want := [][]GridMapCell{
		{{GroundM: 200, DayDB: 55, NightDB: 45}, {GroundM: 210, DayDB: 56, NightDB: 46}},
		{{GroundM: 220, DayDB: 57, NightDB: 47}, {GroundM: 230, DayDB: 58, NightDB: 48}},
	}

	for row := range want {
		if len(decoded.Rows[row]) != len(want[row]) {
			t.Fatalf("row %d has %d cells, want %d", row, len(decoded.Rows[row]), len(want[row]))
		}

		for col, cell := range want[row] {
			if decoded.Rows[row][col] != cell {
				t.Errorf("cell (%d,%d) = %+v, want %+v", row, col, decoded.Rows[row][col], cell)
			}
		}
	}
}

// TestParseDecodedGridMap_Refusals pins the two failures a caller has to be
// able to tell apart: the file is not there, and the file is there but its
// cell stream does not match the run's point count.
func TestParseDecodedGridMap_Refusals(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := ParseDecodedGridMap(filepath.Join(dir, "absent.GM"), 6)
	if err == nil || !strings.Contains(err.Error(), "read GM") {
		t.Errorf("missing file: error = %v, want a read failure", err)
	}

	path := writeFixture(t, dir, "RRLK0022.GM", syntheticGridMap())

	_, err = ParseDecodedGridMap(path, 0)
	if err == nil || !strings.Contains(err.Error(), "points_total must be > 0") {
		t.Errorf("zero points_total: error = %v, want a refusal", err)
	}
}

// TestDecodeGridMapCellRecord_Bounds pins the per-record plausibility filter,
// which is what keeps unrelated bytes out of the raster.
func TestDecodeGridMapCellRecord_Bounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		chunk []byte
		want  bool
	}{
		{name: "a value cell", chunk: gridMapCellRecordBytes(200, 55, 45, 1), want: true},
		{name: "the row marker", chunk: gridMapMarkerCell(), want: true},
		{name: "a zero flag", chunk: gridMapCellRecordBytes(200, 55, 45, 0), want: false},
		{name: "ground below the floor", chunk: gridMapCellRecordBytes(99, 55, 45, 1), want: false},
		{name: "ground above the ceiling", chunk: gridMapCellRecordBytes(401, 55, 45, 1), want: false},
		{name: "day level above the ceiling", chunk: gridMapCellRecordBytes(200, 151, 45, 1), want: false},
		{name: "negative night level", chunk: gridMapCellRecordBytes(200, 55, -1, 1), want: false},
		{name: "not a number", chunk: gridMapCellRecordBytes(float32(math.NaN()), 55, 45, 1), want: false},
		{name: "infinite", chunk: gridMapCellRecordBytes(float32(math.Inf(1)), 55, 45, 1), want: false},
		{name: "a marker with a non-zero level is not a marker", chunk: gridMapCellRecordBytes(-1, 1, 0, 1), want: false},
		{name: "too short", chunk: make([]byte, 12), want: false},
		{name: "too long", chunk: make([]byte, 14), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, ok := decodeGridMapCellRecord(tc.chunk)
			if ok != tc.want {
				t.Errorf("decodeGridMapCellRecord(%s) = %v, want %v", tc.name, ok, tc.want)
			}
		})
	}
}

// TestParseGridMapLayer_Filtering pins which "<name>|<unit>" strings count as
// layer descriptors. The filter is what stops arbitrary printable runs in the
// file from being reported as raster bands.
func TestParseGridMapLayer_Filtering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw      string
		wantName string
		wantUnit string
		wantOK   bool
	}{
		{raw: "Tag|dB(A)", wantName: "Tag", wantUnit: "dB(A)", wantOK: true},
		{raw: "  Sound Level (A)/x | dB  ", wantName: "Sound Level (A)/x", wantUnit: "dB", wantOK: true},
		{raw: "Boden-Hoehe|m", wantName: "Boden-Hoehe", wantUnit: "m", wantOK: true},
		{raw: "Boden_Hoehe_1|m", wantOK: false},
		{raw: "Tag", wantOK: false},
		{raw: "|dB(A)", wantOK: false},
		{raw: "Tag|", wantOK: false},
		{raw: "Tag|kg", wantOK: false},
		{raw: "Tag2|dB", wantOK: false},
		{raw: "123|dB", wantOK: false},
		{raw: "", wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()

			name, unit, ok := parseGridMapLayer(tc.raw)
			if ok != tc.wantOK {
				t.Fatalf("parseGridMapLayer(%q) ok = %v, want %v", tc.raw, ok, tc.wantOK)
			}

			if ok && (name != tc.wantName || unit != tc.wantUnit) {
				t.Errorf("parseGridMapLayer(%q) = %q/%q, want %q/%q", tc.raw, name, unit, tc.wantName, tc.wantUnit)
			}
		})
	}
}

// TestParseGridMapLayers_Deduplicates pins that a descriptor repeated in the
// file is reported once, in first-seen order.
func TestParseGridMapLayers_Deduplicates(t *testing.T) {
	t.Parallel()

	payload := concat(
		[]byte("Tag|dB(A)\x00"),
		[]byte("Boden|m\x00"),
		[]byte("Tag|dB(A)\x00"),
		[]byte("not a descriptor\x00"),
	)

	layers := parseGridMapLayers(payload)

	want := []GridMapLayer{{Name: "Tag", Unit: "dB(A)"}, {Name: "Boden", Unit: "m"}}
	if len(layers) != len(want) {
		t.Fatalf("layers = %+v, want %+v", layers, want)
	}

	for i, expected := range want {
		if layers[i] != expected {
			t.Errorf("layers[%d] = %+v, want %+v", i, layers[i], expected)
		}
	}
}

// TestLoadGridMapMetadata_Synthetic pins which runs the sweep picks up and
// what it carries onto each entry from the .res the run already parsed. The
// geometry file list and the run layout are the only two signals that tell two
// grid maps of the same site apart, so they have to survive even for a run
// whose .GM file is missing.
func TestLoadGridMapMetadata_Synthetic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gridDir := makeDir(t, dir, "RRLK0022")

	writeFixture(t, gridDir, "RRLK0022.GM", syntheticGridMap())

	runs := []*RunResult{
		nil,
		{RunType: "Single Point", ResultSubFolder: "RSPS0011"},
		{RunType: "Grid Map Sound", ResultSubFolder: "   "},
		{
			RunType:           "Grid Map Sound",
			ResultSubFolder:   "RRLK0022",
			RunCommands:       "GNM5:4",
			Statistics:        RunStatistics{PointsTotal: 6, PointsCalculated: 4},
			GeoFiles:          []GeoFileRef{{Name: `C:\proj\GeoObjs.geo`}, {Name: "GeoWand.geo"}},
			AssessmentPeriods: []ResAssessmentPeriod{{Name: "Tag"}, {Name: "  "}},
		},
		{
			RunType:         "Grid Map Sound",
			ResultSubFolder: "RRLK0033",
			RunCommands:     "GNM10:6",
			Statistics:      RunStatistics{PointsTotal: 4},
		},
	}

	items := LoadGridMapMetadata(dir, runs)

	if len(items) != 2 {
		t.Fatalf("got %d grid maps, want the two Grid Map Sound runs with a subfolder", len(items))
	}

	decoded := items[0]
	if decoded.ResultSubFolder != "RRLK0022" || decoded.GMFile != "RRLK0022.GM" {
		t.Errorf("first entry = %+v", decoded)
	}

	if decoded.PointsTotal != 6 || decoded.PointsCalculated != 4 {
		t.Errorf("point counts = %d/%d, want 6/4", decoded.PointsTotal, decoded.PointsCalculated)
	}

	wantGeometry := []string{"geoobjs.geo", "geowand.geo"}
	if len(decoded.GeometryFiles) != len(wantGeometry) {
		t.Fatalf("GeometryFiles = %v, want %v", decoded.GeometryFiles, wantGeometry)
	}

	for i, want := range wantGeometry {
		if decoded.GeometryFiles[i] != want {
			t.Errorf("GeometryFiles[%d] = %q, want %q", i, decoded.GeometryFiles[i], want)
		}
	}

	if decoded.RunLayout == nil || *decoded.RunLayout != (GridMapRunLayout{SpacingM: 5, HeightM: 4}) {
		t.Errorf("RunLayout = %+v, want 5 m spacing at 4 m", decoded.RunLayout)
	}

	if len(decoded.AssessmentPeriods) != 1 || decoded.AssessmentPeriods[0] != "Tag" {
		t.Errorf("AssessmentPeriods = %v, want [Tag]: a blank window name is not a period",
			decoded.AssessmentPeriods)
	}

	if !decoded.DecodedValues || decoded.ActiveCellCount != 6 {
		t.Errorf("entry = %+v, want the .GM payload decoded", decoded)
	}

	missing := items[1]
	if missing.DecodedValues || len(missing.Warnings) != 1 {
		t.Errorf("second entry = %+v, want one warning and no decoded values", missing)
	}

	if missing.RunLayout == nil || missing.RunLayout.SpacingM != 10 {
		t.Errorf("second entry RunLayout = %+v, want the layout to survive a missing .GM", missing.RunLayout)
	}

	if missing.GMFile != "RRLK0033.GM" {
		t.Errorf("second entry GMFile = %q, want the name the run implies", missing.GMFile)
	}
}
