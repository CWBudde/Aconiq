package export

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/io/soundplanimport"
	"github.com/aconiq/backend/internal/qa/golden"
)

// soundPLANFixture is a model with one feature of every kind the export
// places, and one of each thing it has to report rather than write.
func soundPLANFixture(t *testing.T) []ModelFeature {
	t.Helper()

	longText := strings.Repeat("x", MaxDBFCharBytes+1)

	raw := `{"type":"FeatureCollection","features":[
	{"type":"Feature","properties":{"id":"b1","kind":"building","height_m":12.5,"height_source":"osm"},
	 "geometry":{"type":"Polygon","coordinates":[[[500000,5800000],[500010,5800000],[500010,5800010],[500000,5800010],[500000,5800000]]]}},
	{"type":"Feature","properties":{"id":"b2","kind":"building","height_m":8},
	 "geometry":{"type":"MultiPolygon","coordinates":[[[[500020,5800000],[500030,5800000],[500030,5800005],[500020,5800000]]],[[[500040,5800000],[500050,5800000],[500050,5800005],[500040,5800000]]]]}},
	{"type":"Feature","properties":{"id":"w1","kind":"barrier","height_m":3},
	 "geometry":{"type":"LineString","coordinates":[[500000,5800020],[500030,5800020]]}},
	{"type":"Feature","properties":{"id":"r1","kind":"receiver","height_m":4,"bimschv16_area_category":"Mischgebiet"},
	 "geometry":{"type":"Point","coordinates":[500100,5800100]}},
	{"type":"Feature","properties":{"id":"r2","kind":"receiver","height_m":4},
	 "geometry":{"type":"Point","coordinates":[500110,5800100,62.5]}},
	{"type":"Feature","properties":{"id":"s1","kind":"source","source_type":"point","iso9613_sound_power_level_db":100,
	  "iso9613_source_height_m":1.5,"note":"` + longText + `","mixed":"a","pi":3.14159265358979,
	  "rls19_directional_sources":[{"direction":"forward"}]},
	 "geometry":{"type":"Point","coordinates":[500200,5800200]}},
	{"type":"Feature","properties":{"id":"s2","kind":"source","source_type":"point","iso9613_sound_power_level_db":95.125,"mixed":5,"note":"short"},
	 "geometry":{"type":"MultiPoint","coordinates":[[500210,5800200],[500220,5800200]]}},
	{"type":"Feature","properties":{"id":"l1","kind":"source","source_type":"line","rls19_traffic_day_veh_h":300,"rls19_traffic_night_veh_h":40,"rls19_speed_kph":50},
	 "geometry":{"type":"LineString","coordinates":[[500000,5800300],[500100,5800300]]}},
	{"type":"Feature","properties":{"id":"a1","kind":"source","source_type":"area","iso9613_sound_power_level_db":90,"active":true},
	 "geometry":{"type":"Polygon","coordinates":[[[500300,5800300],[500310,5800300],[500310,5800310],[500300,5800300]]]}},
	{"type":"Feature","properties":{"id":"ca","kind":"calc-area"},
	 "geometry":{"type":"Polygon","coordinates":[[[499900,5799900],[500400,5799900],[500400,5800400],[499900,5800400],[499900,5799900]]]}},
	{"type":"Feature","properties":{"id":"gz","kind":"ground-zone","ground_factor":0.5},
	 "geometry":{"type":"Polygon","coordinates":[[[500000,5800050],[500050,5800050],[500050,5800060],[500000,5800050]]]}},
	{"type":"Feature","properties":{"id":"x1","kind":"tree"},
	 "geometry":{"type":"Point","coordinates":[500000,5800000]}}
	]}`

	var fc struct {
		Features []struct {
			Properties map[string]any `json:"properties"`
			Geometry   struct {
				Type        string `json:"type"`
				Coordinates any    `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}

	if err := json.Unmarshal([]byte(raw), &fc); err != nil {
		t.Fatal(err)
	}

	out := make([]ModelFeature, 0, len(fc.Features))

	for _, f := range fc.Features {
		id, _ := f.Properties["id"].(string)
		kind, _ := f.Properties["kind"].(string)
		sourceType, _ := f.Properties["source_type"].(string)

		out = append(out, ModelFeature{
			ID: id, Kind: kind, SourceType: sourceType,
			GeometryType: f.Geometry.Type, Coordinates: f.Geometry.Coordinates,
			Properties: f.Properties,
		})
	}

	return out
}

func exportFixture(t *testing.T) (string, SoundPLANReport) {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "soundplan")

	report, paths, err := ExportSoundPLANShapefiles(dir, soundPLANFixture(t), SoundPLANOptions{
		CRS:  "EPSG:25832",
		Date: testDate(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Five files per layer: .shp .shx .dbf .cpg .prj.
	if len(paths) != 5*len(report.Layers) {
		t.Fatalf("wrote %d files for %d layers", len(paths), len(report.Layers))
	}

	return dir, report
}

// The mapping report is the contract a SoundPLAN user fills the assignment
// table from; the golden pins every column name, unit, rounding and loss.
func TestExportSoundPLANMappingReport(t *testing.T) {
	t.Parallel()

	_, report := exportFixture(t)

	golden.AssertJSONSnapshot(t, filepath.Join("testdata", "soundplan-mapping.golden.json"), report)
}

func TestExportSoundPLANLayersCarryTheModel(t *testing.T) {
	t.Parallel()

	dir, report := exportFixture(t)

	layers := make([]string, 0, len(report.Layers))
	for _, l := range report.Layers {
		layers = append(layers, l.Layer)
	}

	want := []string{"buildings", "barriers", "receivers", "sources_point", "sources_line", "sources_area", "calc_area", "ground_zones"}
	if !slices.Equal(layers, want) {
		t.Fatalf("layers %v, want %v", layers, want)
	}

	// A MultiPoint source is one record per part, each with the full Lw —
	// what ISO 9613-2 extraction computes — and the part's own id.
	points := readDBFFile(t, filepath.Join(dir, "sources_point.dbf"))

	names := make([]string, 0, len(points.fields))
	for _, f := range points.fields {
		names = append(names, f.Name)
	}

	col := func(name string) int { return slices.Index(names, name) }

	if col("LW_DB") < 0 || col("HS_M") < 0 || col("PART") < 0 {
		t.Fatalf("sources_point fields %v", names)
	}

	n := len(points.rows)

	ids, lws, parts := make([]string, 0, n), make([]string, 0, n), make([]string, 0, n)
	for _, row := range points.rows {
		ids = append(ids, row[col("ID")])
		lws = append(lws, row[col("LW_DB")])
		parts = append(parts, row[col("PART")])
	}

	// 95.125 is exact in binary, so the 2-decimal boundary meets a true tie:
	// strconv rounds it to even, 95.12. The tie rule is pinned here so a
	// change of formatter cannot move it unnoticed.
	if strings.Join(ids, ",") != "s1,s2-01,s2-02" || strings.Join(lws, ",") != "100.00,95.12,95.12" ||
		strings.Join(parts, ",") != ",1,2" {
		t.Errorf("ids %v, lws %v, parts %v", ids, lws, parts)
	}

	// The over-long note is left empty for s1 and written for s2.
	if note := col("NOTE"); note < 0 || points.rows[0][note] != "" || points.rows[1][note] != "short" {
		t.Errorf("NOTE column %d in %v", note, points.rows)
	}

	// Buildings keep their geometry, outer rings turned clockwise.
	buildings := readShapefile(t, filepath.Join(dir, "buildings"))
	if len(buildings.records) != 2 || len(buildings.records[1]) != 2 {
		t.Fatalf("buildings records %v", buildings.records)
	}

	for _, record := range buildings.records {
		for _, ring := range record {
			if signedArea(ring) >= 0 {
				t.Errorf("an outer building ring is not clockwise: %v", ring)
			}
		}
	}

	if !strings.Contains(string(mustRead(t, filepath.Join(dir, "buildings.prj"))), "ETRS_1989_UTM_Zone_32N") {
		t.Error("buildings.prj does not name EPSG:25832")
	}
}

func TestExportSoundPLANReportsWhatItCannotCarry(t *testing.T) {
	t.Parallel()

	_, report := exportFixture(t)

	type key struct{ id, property string }

	got := map[key]bool{}
	for _, e := range report.Lossy {
		got[key{e.FeatureID, e.Property}] = true
	}

	for _, want := range []key{
		{"x1", "kind"},                      // no layer for the kind
		{"r2", "geometry"},                  // z coordinate dropped
		{"s1", "note"},                      // over 254 bytes
		{"s1", "rls19_directional_sources"}, // structured value
		{"s1", "mixed"},                     // text here, a number on s2
		{"s2", "mixed"},
		{"s1", "pi"}, // more digits than a passthrough keeps
	} {
		if !got[want] {
			t.Errorf("lossy report misses %v; it has %v", want, report.Lossy)
		}
	}

	if len(report.Lossy) != 7 {
		t.Errorf("lossy report has %d entries, want exactly the 7 above: %v", len(report.Lossy), report.Lossy)
	}
}

// An ID longer than a character field holds costs its feature the ID cell,
// reported, rather than failing the export — and so does a MultiPoint part
// whose -NN suffix pushes an ID that fit past the limit.
func TestExportSoundPLANReportsAnOverlongID(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("r", MaxDBFCharBytes+1)
	edge := strings.Repeat("s", MaxDBFCharBytes-1)

	features := []ModelFeature{
		{
			ID: long, Kind: "receiver", GeometryType: geoJSONPoint,
			Coordinates: []any{500000.0, 5800000.0},
			Properties:  map[string]any{"id": long, "kind": "receiver", "height_m": 4.0},
		},
		{
			ID: edge, Kind: kindSource, SourceType: "point", GeometryType: geoJSONMultiPoint,
			Coordinates: []any{[]any{500010.0, 5800000.0}, []any{500020.0, 5800000.0}},
			Properties:  map[string]any{"id": edge, "kind": kindSource, "source_type": "point", "iso9613_sound_power_level_db": 90.0},
		},
	}

	dir := t.TempDir()

	report, _, err := ExportSoundPLANShapefiles(dir, features, SoundPLANOptions{CRS: "EPSG:25832", Date: testDate()})
	if err != nil {
		t.Fatalf("an overlong ID failed the export: %v", err)
	}

	idLosses := map[string]int{}

	for _, e := range report.Lossy {
		if e.Property == "id" {
			idLosses[e.FeatureID]++
		}
	}

	if idLosses[long] != 1 || idLosses[edge] != 1 || len(idLosses) != 2 {
		t.Errorf("ID losses %v, want one for each feature: %v", idLosses, report.Lossy)
	}

	for _, layer := range []string{"receivers", "sources_point"} {
		for _, row := range readDBFFile(t, filepath.Join(dir, layer+".dbf")).rows {
			if row[0] != "" {
				t.Errorf("%s writes ID %q, want it left empty", layer, row[0])
			}
		}
	}
}

func TestExportSoundPLANRefusesPlanesSoundPLANWouldMismeasure(t *testing.T) {
	t.Parallel()

	for _, crs := range []string{"EPSG:4326", "EPSG:3857", "", "garbage"} {
		_, _, err := ExportSoundPLANShapefiles(t.TempDir(), soundPLANFixture(t), SoundPLANOptions{CRS: crs, Date: testDate()})
		if err == nil {
			t.Errorf("CRS %q was accepted", crs)
		}
	}
}

// terrain.txt is written in the layout SoundPLAN's own Höhen.txt uses, so the
// importer this project already has is the reader that checks it.
func TestWriteSoundPLANTerrainRoundTripsThroughTheImporter(t *testing.T) {
	t.Parallel()

	points := [][3]float64{{500000.5, 5800000.25, 55.1234}, {500001, 5800000, -3.5}, {500002, 5799999.9996, 0}}
	path := filepath.Join(t.TempDir(), SoundPLANTerrainFile)

	report, err := WriteSoundPLANTerrain(path, func(emit func(x, y, z float64) error) error {
		for _, p := range points {
			if err := emit(p[0], p[1], p[2]); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if report.Points != len(points) || report.File != SoundPLANTerrainFile {
		t.Fatalf("report %+v", report)
	}

	if first := strings.SplitN(string(mustRead(t, path)), "\n", 2)[0]; first != "500000,500;5800000,250;55,123" {
		t.Errorf("first line %q", first)
	}

	parsed, err := soundplanimport.ParseHoehenTxtFile(path)
	if err != nil {
		t.Fatal(err)
	}

	for i, p := range parsed {
		got := [3]float64{p.X, p.Y, p.Z}
		for axis := range 3 {
			if math.Abs(got[axis]-points[i][axis]) > 0.0005 {
				t.Errorf("point %d axis %d: %v, wrote %v", i, axis, got[axis], points[i][axis])
			}
		}
	}

	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("the temporary file was left behind")
	}
}

func TestWriteSoundPLANTerrainRefusesAnEmptyTerrain(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), SoundPLANTerrainFile)

	_, err := WriteSoundPLANTerrain(path, func(func(x, y, z float64) error) error { return nil })
	if err == nil {
		t.Fatal("an empty terrain was written")
	}

	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("a refused terrain left a file")
	}
}
