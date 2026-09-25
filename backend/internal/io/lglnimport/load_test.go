package lglnimport

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/io/citygmlimport"
)

// utm32 projects a WGS84 point into EPSG:25832, the CRS LGLN tiles use, so a
// fixture building can be placed relative to the test's bounding box.
func utm32(t *testing.T, lon, lat float64) geo.Point2D {
	t.Helper()

	from, err := geo.ParseCRS("EPSG:4326")
	if err != nil {
		t.Fatal(err)
	}

	to, err := geo.ParseCRS("EPSG:25832")
	if err != nil {
		t.Fatal(err)
	}

	pipeline, err := geo.BuildTransformPipeline(to, from)
	if err != nil {
		t.Fatal(err)
	}

	p, err := pipeline.ApplyPoint(geo.Point2D{X: lon, Y: lat})
	if err != nil {
		t.Fatal(err)
	}

	return p
}

type fixtureBuilding struct {
	id       string
	lon, lat float64 // footprint centre
	height   float64
}

// cityGMLTile renders buildings as 10 m × 10 m blocks in a CityGML 2.0
// document with an AdV srsName, the way LGLN tiles name their CRS.
func cityGMLTile(t *testing.T, buildings ...fixtureBuilding) string {
	t.Helper()

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
		c := utm32(t, bld.lon, bld.lat)
		x0, y0, x1, y1 := c.X-5, c.Y-5, c.X+5, c.Y+5
		fmt.Fprintf(&b, `  <core:cityObjectMember><bldg:Building gml:id=%q>
    <bldg:measuredHeight uom="m">%g</bldg:measuredHeight>
    <bldg:boundedBy><bldg:GroundSurface><bldg:lod2MultiSurface><gml:MultiSurface><gml:surfaceMember>
      <gml:Polygon><gml:exterior><gml:LinearRing><gml:posList srsDimension="3">%f %f 50 %f %f 50 %f %f 50 %f %f 50 %f %f 50</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon>
    </gml:surfaceMember></gml:MultiSurface></bldg:lod2MultiSurface></bldg:GroundSurface></bldg:boundedBy>
  </bldg:Building></core:cityObjectMember>
`, bld.id, bld.height, x0, y0, x1, y0, x1, y1, x0, y1, x0, y0)
	}

	b.WriteString("</core:CityModel>\n")

	return b.String()
}

func TestLoadFiltersByCentroidDedupsAndReprojects(t *testing.T) {
	t.Parallel()

	inside := fixtureBuilding{id: "DENI_inside", lon: 9.740, lat: 52.375, height: 12}
	outside := fixtureBuilding{id: "DENI_outside", lon: 9.750, lat: 52.375, height: 8}

	tiles := map[string]string{
		"/a.gml": cityGMLTile(t, inside, outside),
		// The same building listed again by the neighbouring tile must come
		// back once.
		"/b.gml": cityGMLTile(t, inside),
		// A tile without buildings is empty, not broken.
		"/c.gml": cityGMLTile(t),
	}

	var f *fixture

	f = newFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		writePage(t, w, []map[string]any{
			item("LoD2_32_550_5803_1_ni", "2024-06-12", f.tiles.URL+"/a.gml"),
			item("LoD2_32_551_5803_1_ni", "2024-06-12", f.tiles.URL+"/b.gml"),
			item("LoD2_32_552_5803_1_ni", "2023-01-01", f.tiles.URL+"/c.gml"),
		}, "")
	}, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(tiles[r.URL.Path]))
	})

	result, err := f.client().Load(context.Background(), hannover, t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(result.Tiles) != 3 {
		t.Fatalf("tiles = %d, want 3", len(result.Tiles))
	}

	features := result.Collection.Features
	if len(features) != 1 {
		t.Fatalf("features = %d, want 1: %+v", len(features), features)
	}

	props := features[0].Properties
	if props["id"] != inside.id || props["kind"] != "building" || props["height_m"] != inside.height {
		t.Errorf("properties = %v", props)
	}

	if props["import_format"] != ImportFormat || props["lgln_tile"] != "LoD2_32_550_5803_1_ni" {
		t.Errorf("provenance properties = %v", props)
	}

	rings, ok := polygonRings(features[0].Geometry.Coordinates)
	if !ok {
		t.Fatalf("geometry not a polygon: %v", features[0].Geometry.Coordinates)
	}

	centroid, ok := geo.PolygonCentroid(rings)
	if !ok || !hannover.containsCentroid(centroid) {
		t.Errorf("centroid %v not in WGS84 box %v", centroid, hannover)
	}
}

func TestLoadCountsSkipsOfATileWithNothingUsable(t *testing.T) {
	t.Parallel()

	// One building, and its footprint crosses itself: the tile has nothing
	// usable, which is an empty result with a skip count, not an error.
	bowtie := `<?xml version="1.0" encoding="UTF-8"?>
<core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0"
  xmlns:bldg="http://www.opengis.net/citygml/building/2.0"
  xmlns:gml="http://www.opengis.net/gml">
  <core:cityObjectMember><bldg:Building gml:id="DENI_bowtie">
    <bldg:measuredHeight uom="m">9</bldg:measuredHeight>
    <bldg:boundedBy><bldg:GroundSurface><bldg:lod2MultiSurface><gml:MultiSurface><gml:surfaceMember>
      <gml:Polygon><gml:exterior><gml:LinearRing><gml:posList srsName="EPSG:25832" srsDimension="2">550000 5803000 550010 5803010 550010 5803000 550000 5803010 550000 5803000</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon>
    </gml:surfaceMember></gml:MultiSurface></bldg:lod2MultiSurface></bldg:GroundSurface></bldg:boundedBy>
  </bldg:Building></core:cityObjectMember>
</core:CityModel>
`

	var f *fixture

	f = newFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		writePage(t, w, []map[string]any{
			item("LoD2_32_550_5803_1_ni", "2024-06-12", f.tiles.URL+"/a.gml"),
		}, "")
	}, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(bowtie))
	})

	result, err := f.client().Load(context.Background(), hannover, t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(result.Collection.Features) != 0 {
		t.Errorf("features = %d, want 0", len(result.Collection.Features))
	}

	if result.Skipped[string(citygmlimport.SkipSelfIntersects)] != 1 {
		t.Errorf("skipped = %v, want the bowtie counted", result.Skipped)
	}
}

func TestLoadPropagatesSearchErrors(t *testing.T) {
	t.Parallel()

	f := newFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}, notCalled(t))

	_, err := f.client().Load(context.Background(), hannover, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), ErrUnavailable.Error()) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestLoadRejectsUnparseableTile(t *testing.T) {
	t.Parallel()

	var f *fixture

	f = newFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		writePage(t, w, []map[string]any{
			item("LoD2_32_550_5803_1_ni", "2024-06-12", f.tiles.URL+"/a.gml"),
		}, "")
	}, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<not-citygml"))
	})

	_, err := f.client().Load(context.Background(), hannover, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "parse tile LoD2_32_550_5803_1_ni") {
		t.Fatalf("err = %v, want a parse error naming the tile", err)
	}
}

func TestContainsCentroidIncludesEdges(t *testing.T) {
	t.Parallel()

	bb := BBox{West: 0, South: 0, East: 1, North: 1}

	cases := []struct {
		p    geo.Point2D
		want bool
	}{
		{geo.Point2D{X: 0, Y: 0}, true},
		{geo.Point2D{X: 0.5, Y: 0.5}, true},
		{geo.Point2D{X: 1, Y: 0.5}, true},
		{geo.Point2D{X: 0.5, Y: 1}, true},
		{geo.Point2D{X: 1.1, Y: 0.5}, false},
		{geo.Point2D{X: -0.1, Y: 0.5}, false},
	}

	for _, tc := range cases {
		if got := bb.containsCentroid(tc.p); got != tc.want {
			t.Errorf("containsCentroid(%v) = %v, want %v", tc.p, got, tc.want)
		}
	}
}
