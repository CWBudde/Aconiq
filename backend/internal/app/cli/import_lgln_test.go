package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/lglnimport"
)

// The LGLN import tests swap newLGLNClient, so they do not run in parallel:
// Go starts a package's parallel tests only after its sequential ones end.

const hannoverLGLNFlag = "52.372,9.735,52.378,9.745"

// lglnTileBuilding is a 10 m × 10 m LoD2 block centred on a WGS84 point.
type lglnTileBuilding struct {
	id       string
	lon, lat float64
}

// serveLGLN points newLGLNClient at one TLS server that answers every STAC
// search with a single tile holding the given buildings.
func serveLGLN(t *testing.T, buildings ...lglnTileBuilding) {
	t.Helper()

	tile := lglnCityGMLTile(t, buildings...)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".gml") {
			_, _ = w.Write([]byte(tile))

			return
		}

		w.Header().Set("Content-Type", "application/geo+json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "FeatureCollection",
			"features": []map[string]any{{
				"id":         "LoD2_32_550_5803_1_ni",
				"type":       "Feature",
				"properties": map[string]any{"letzte_aenderung": "2024-06-12"},
				"assets": map[string]any{
					"lod2-gml": map[string]any{"href": "https://" + r.Host + "/LoD2_32_550_5803_1_ni.gml"},
				},
			}},
		})
	}))
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	previous := newLGLNClient
	newLGLNClient = func() *lglnimport.Client {
		return lglnimport.NewClient(
			lglnimport.WithSTACURL(server.URL),
			lglnimport.WithHTTPClient(server.Client()),
			lglnimport.WithAllowedHosts(u.Host),
		)
	}

	t.Cleanup(func() { newLGLNClient = previous })
}

// lglnCityGMLTile renders the buildings as a CityGML 2.0 tile in ETRS89 /
// UTM 32N with the AdV srsName LGLN tiles carry.
func lglnCityGMLTile(t *testing.T, buildings ...lglnTileBuilding) string {
	t.Helper()

	from, err := geo.ParseCRS("EPSG:4326")
	if err != nil {
		t.Fatal(err)
	}

	to, err := geo.ParseCRS("EPSG:25832")
	if err != nil {
		t.Fatal(err)
	}

	toUTM, err := geo.BuildTransformPipeline(to, from)
	if err != nil {
		t.Fatal(err)
	}

	var b strings.Builder

	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0"
  xmlns:bldg="http://www.opengis.net/citygml/building/2.0"
  xmlns:gml="http://www.opengis.net/gml">
  <gml:boundedBy><gml:Envelope srsName="urn:adv:crs:ETRS89_UTM32*DE_DHHN2016_NH" srsDimension="3">
    <gml:lowerCorner>0 0 0</gml:lowerCorner><gml:upperCorner>1 1 1</gml:upperCorner>
  </gml:Envelope></gml:boundedBy>
`)

	for _, bld := range buildings {
		c, err := toUTM.ApplyPoint(geo.Point2D{X: bld.lon, Y: bld.lat})
		if err != nil {
			t.Fatal(err)
		}

		x0, y0, x1, y1 := c.X-5, c.Y-5, c.X+5, c.Y+5
		fmt.Fprintf(&b, `  <core:cityObjectMember><bldg:Building gml:id=%q>
    <bldg:measuredHeight uom="m">12</bldg:measuredHeight>
    <bldg:boundedBy><bldg:GroundSurface><bldg:lod2MultiSurface><gml:MultiSurface><gml:surfaceMember>
      <gml:Polygon><gml:exterior><gml:LinearRing><gml:posList srsDimension="3">%f %f 50 %f %f 50 %f %f 50 %f %f 50 %f %f 50</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon>
    </gml:surfaceMember></gml:MultiSurface></bldg:lod2MultiSurface></bldg:GroundSurface></bldg:boundedBy>
  </bldg:Building></core:cityObjectMember>
`, bld.id, x0, y0, x1, y0, x1, y1, x0, y1, x0, y0)
	}

	b.WriteString("</core:CityModel>\n")

	return b.String()
}

// lonLatSquare is a closed ~10 m square ring around a WGS84 point.
func lonLatSquare(lon, lat float64) [][][2]float64 {
	const d = 0.00007

	return [][][2]float64{{
		{lon - d, lat - d}, {lon + d, lat - d}, {lon + d, lat + d}, {lon - d, lat + d}, {lon - d, lat - d},
	}}
}

// writeOSMLikeModel writes a WGS84 GeoJSON model shaped like an OSM import:
// a road, two OSM buildings (one inside the Hannover box, one outside) and a
// drawn building inside the box.
func writeOSMLikeModel(t *testing.T, dir string) string {
	t.Helper()

	feature := func(props map[string]any, geomType string, coords any) map[string]any {
		return map[string]any{
			"type":       "Feature",
			"properties": props,
			"geometry":   map[string]any{"type": geomType, "coordinates": coords},
		}
	}

	fc := map[string]any{
		"type": "FeatureCollection",
		"features": []any{
			feature(map[string]any{"id": "osm-way-10", "kind": "source", "source_type": "line", "osm_id": "10"},
				"LineString", [][2]float64{{9.736, 52.373}, {9.744, 52.377}}),
			feature(map[string]any{"id": "osm-way-1", "kind": "building", "height_m": 9.0, "osm_id": "1"},
				"Polygon", lonLatSquare(9.740, 52.375)),
			feature(map[string]any{"id": "osm-way-2", "kind": "building", "height_m": 9.0, "osm_id": "2"},
				"Polygon", lonLatSquare(9.760, 52.375)),
			feature(map[string]any{"id": "drawn-1", "kind": "building", "height_m": 6.0},
				"Polygon", lonLatSquare(9.742, 52.376)),
		},
	}

	payload, err := json.Marshal(fc)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "osm.geojson")

	err = os.WriteFile(path, payload, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	return path
}

func readModelIDs(t *testing.T, projectDir string) []string {
	t.Helper()

	payload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "model", "model.normalized.geojson"))
	if err != nil {
		t.Fatalf("read normalized model: %v", err)
	}

	var fc modelgeojson.FeatureCollection

	err = json.Unmarshal(payload, &fc)
	if err != nil {
		t.Fatalf("decode normalized model: %v", err)
	}

	ids := make([]string, 0, len(fc.Features))
	for _, f := range fc.Features {
		id, _ := f.Properties["id"].(string)
		ids = append(ids, id)
	}

	return ids
}

func TestImportLGLNIntoEmptyProject(t *testing.T) {
	serveLGLN(t, lglnTileBuilding{id: "DENI_a", lon: 9.741, lat: 52.376})

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "LGLN", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--from-lgln", hannoverLGLNFlag)

	if got := readModelIDs(t, projectDir); !slices.Equal(got, []string{"DENI_a"}) {
		t.Fatalf("model ids = %v, want [DENI_a]", got)
	}

	_, err := os.Stat(filepath.Join(projectDir, ".noise", "cache", "lgln"))
	if err != nil {
		t.Errorf("tile cache not written under .noise/cache/lgln: %v", err)
	}
}

func TestImportLGLNHonoursCacheDir(t *testing.T) {
	serveLGLN(t, lglnTileBuilding{id: "DENI_a", lon: 9.741, lat: 52.376})

	projectDir := t.TempDir()
	cacheDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "LGLN", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "--cache-dir", cacheDir, "import", "--from-lgln", hannoverLGLNFlag)

	_, err := os.Stat(filepath.Join(cacheDir, "lgln"))
	if err != nil {
		t.Errorf("tile cache not written under --cache-dir: %v", err)
	}

	_, err = os.Stat(filepath.Join(projectDir, ".noise", "cache", "lgln"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("tile cache written under the project despite --cache-dir: %v", err)
	}
}

func TestLGLNLoadErrorKinds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want domainerrors.Kind
	}{
		{"too many tiles", &lglnimport.TooManyTilesError{}, domainerrors.KindUserInput},
		{"outside coverage", lglnimport.ErrOutsideCoverage, domainerrors.KindUserInput},
		{"invalid bbox", lglnimport.ErrInvalidBBox, domainerrors.KindUserInput},
		{"service unavailable", lglnimport.ErrUnavailable, domainerrors.KindInternal},
		{"invalid tile", lglnimport.ErrInvalidTile, domainerrors.KindInternal},
		{"timeout", context.DeadlineExceeded, domainerrors.KindInternal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var appErr *domainerrors.AppError
			if !errors.As(lglnLoadError(tc.err), &appErr) {
				t.Fatalf("lglnLoadError(%v) is not an AppError", tc.err)
			}

			if appErr.Kind != tc.want {
				t.Errorf("kind = %s, want %s", appErr.Kind, tc.want)
			}
		})
	}
}

func TestImportLGLNMergesIntoOSMModel(t *testing.T) {
	serveLGLN(
		t,
		lglnTileBuilding{id: "DENI_a", lon: 9.741, lat: 52.376},
		lglnTileBuilding{id: "DENI_out", lon: 9.760, lat: 52.376}, // outside the box: Load drops it
	)

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "LGLN", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", writeOSMLikeModel(t, projectDir), "--input-crs", "EPSG:4326")
	mustRunCLI(t, "--project", projectDir, "import", "--from-lgln", hannoverLGLNFlag)

	// The road, the OSM building outside the box and the drawn building stay;
	// the OSM building inside the box makes way for the LGLN one.
	want := []string{"osm-way-10", "osm-way-2", "drawn-1", "DENI_a"}
	if got := readModelIDs(t, projectDir); !slices.Equal(got, want) {
		t.Fatalf("model ids after LGLN import = %v, want %v", got, want)
	}

	// A second load of the same box changes nothing.
	mustRunCLI(t, "--project", projectDir, "import", "--from-lgln", hannoverLGLNFlag)

	if got := readModelIDs(t, projectDir); !slices.Equal(got, want) {
		t.Fatalf("model ids after repeat LGLN import = %v, want %v", got, want)
	}
}

func TestImportLGLNRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"with --from-osm", []string{"--from-lgln", hannoverLGLNFlag, "--from-osm", hannoverLGLNFlag}, "cannot use --from-osm together with --from-lgln"},
		{"with --input", []string{"--from-lgln", hannoverLGLNFlag, "--input", "x.geojson"}, "cannot use --input together with --from-lgln"},
		{"malformed box", []string{"--from-lgln", "52.3,9.7,52.4"}, "invalid --from-lgln bbox"},
		{"south above north", []string{"--from-lgln", "52.378,9.735,52.372,9.745"}, "invalid --from-lgln bbox"},
		{"outside Lower Saxony", []string{"--from-lgln", "48.1,11.5,48.2,11.6"}, "Lower Saxony only"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			projectDir := t.TempDir()
			mustRunCLI(t, "--project", projectDir, "init", "--name", "LGLN", "--crs", "EPSG:25832")

			err := runCLI(append([]string{"--project", projectDir, "import"}, tc.args...)...)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestMergeLGLNBuildingsCounts(t *testing.T) {
	t.Parallel()

	square := func(lon, lat float64) any {
		ring := make([]any, 0, 5)
		for _, p := range lonLatSquare(lon, lat)[0] {
			ring = append(ring, []any{p[0], p[1]})
		}

		return []any{ring}
	}

	building := func(id string, props map[string]any, lon, lat float64) modelgeojson.Feature {
		return modelgeojson.Feature{
			ID: id, Kind: modelgeojson.FeatureKindBuilding, Properties: props,
			GeometryType: "Polygon", Coordinates: square(lon, lat),
		}
	}

	lgln := map[string]any{"import_format": lglnimport.ImportFormat}

	existing := []modelgeojson.Feature{
		building("osm-way-1", map[string]any{}, 9.740, 52.375),            // OSM by id, inside: replaced
		building("renamed", map[string]any{"osm_id": "7"}, 9.738, 52.374), // OSM by tag, inside: replaced
		building("osm-way-2", map[string]any{}, 9.760, 52.375),            // OSM, outside: stays
		building("DENI_old", lgln, 9.741, 52.376),                         // earlier LGLN load: stays
	}
	incoming := []modelgeojson.Feature{
		building("DENI_old", lgln, 9.741, 52.376),
		building("DENI_new", lgln, 9.739, 52.375),
	}

	identity, err := wgs84PipelineFrom("EPSG:4326")
	if err != nil {
		t.Fatal(err)
	}

	bb := lglnimport.BBox{West: 9.735, South: 52.372, East: 9.745, North: 52.378}

	merged, stats, err := mergeLGLNBuildings(existing, incoming, bb, identity)
	if err != nil {
		t.Fatal(err)
	}

	ids := make([]string, 0, len(merged))
	for _, f := range merged {
		ids = append(ids, f.ID)
	}

	if want := []string{"osm-way-2", "DENI_old", "DENI_new"}; !slices.Equal(ids, want) {
		t.Errorf("merged ids = %v, want %v", ids, want)
	}

	if stats != (lglnMergeStats{added: 1, replaced: 2, duplicates: 1}) {
		t.Errorf("stats = %+v", stats)
	}
}
