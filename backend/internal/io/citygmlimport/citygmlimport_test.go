package citygmlimport

import (
	"errors"
	"testing"
)

func TestReadExtractsBuildingFootprintsAndHeight(t *testing.T) {
	t.Parallel()

	payload := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0"
                xmlns:bldg="http://www.opengis.net/citygml/building/2.0"
                xmlns:gml="http://www.opengis.net/gml">
  <core:cityObjectMember>
    <bldg:Building gml:id="b-1">
      <bldg:measuredHeight>12</bldg:measuredHeight>
      <bldg:lod1Solid>
        <gml:Solid>
          <gml:exterior>
            <gml:CompositeSurface>
              <gml:surfaceMember>
                <gml:Polygon gml:id="ground">
                  <gml:exterior>
                    <gml:LinearRing>
                      <gml:posList>
                        0 0 100 10 0 100 10 10 100 0 10 100 0 0 100
                      </gml:posList>
                    </gml:LinearRing>
                  </gml:exterior>
                </gml:Polygon>
              </gml:surfaceMember>
              <gml:surfaceMember>
                <gml:Polygon gml:id="roof">
                  <gml:exterior>
                    <gml:LinearRing>
                      <gml:posList>
                        0 0 112 10 0 112 10 10 112 0 10 112 0 0 112
                      </gml:posList>
                    </gml:LinearRing>
                  </gml:exterior>
                </gml:Polygon>
              </gml:surfaceMember>
            </gml:CompositeSurface>
          </gml:exterior>
        </gml:Solid>
      </bldg:lod1Solid>
    </bldg:Building>
  </core:cityObjectMember>
</core:CityModel>`)

	fc, err := Read(payload)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(fc.Features))
	}

	feature := fc.Features[0]
	if feature.Properties["kind"] != "building" {
		t.Fatalf("expected building kind, got %#v", feature.Properties["kind"])
	}

	if feature.Properties["height_m"] != 12.0 {
		t.Fatalf("expected measured height 12, got %#v", feature.Properties["height_m"])
	}

	if feature.Geometry.Type != "Polygon" {
		t.Fatalf("expected polygon geometry, got %q", feature.Geometry.Type)
	}
}

func TestReadComputesHeightFromZExtentWhenMissingMeasuredHeight(t *testing.T) {
	t.Parallel()

	payload := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<CityModel xmlns:bldg="http://www.opengis.net/citygml/building/2.0" xmlns:gml="http://www.opengis.net/gml">
  <cityObjectMember>
    <bldg:Building gml:id="b-2">
      <bldg:boundedBy>
        <bldg:GroundSurface>
          <gml:Polygon>
            <gml:exterior>
              <gml:LinearRing>
                <gml:posList>0 0 5 4 0 5 4 4 5 0 4 5 0 0 5</gml:posList>
              </gml:LinearRing>
            </gml:exterior>
          </gml:Polygon>
        </bldg:GroundSurface>
      </bldg:boundedBy>
      <bldg:boundedBy>
        <bldg:RoofSurface>
          <gml:Polygon>
            <gml:exterior>
              <gml:LinearRing>
                <gml:posList>0 0 14 4 0 14 4 4 14 0 4 14 0 0 14</gml:posList>
              </gml:LinearRing>
            </gml:exterior>
          </gml:Polygon>
        </bldg:RoofSurface>
      </bldg:boundedBy>
    </bldg:Building>
  </cityObjectMember>
</CityModel>`)

	fc, err := Read(payload)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got := fc.Features[0].Properties["height_m"]; got != 9.0 {
		t.Fatalf("expected computed height 9, got %#v", got)
	}
}

func TestReadRejectsFilesWithoutSupportedBuildings(t *testing.T) {
	t.Parallel()

	payload := []byte(`<?xml version="1.0" encoding="UTF-8"?><CityModel></CityModel>`)

	_, err := Read(payload)
	if err == nil {
		t.Fatal("expected error for empty CityGML content")
	}
}

func TestReadPreservesCityGMLAttributes(t *testing.T) {
	t.Parallel()

	payload := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0"
                xmlns:bldg="http://www.opengis.net/citygml/building/2.0"
                xmlns:gml="http://www.opengis.net/gml">
  <core:cityObjectMember>
    <bldg:Building gml:id="attr-test">
      <bldg:class>1000</bldg:class>
      <bldg:function>residential</bldg:function>
      <bldg:usage>office</bldg:usage>
      <bldg:measuredHeight>8</bldg:measuredHeight>
      <bldg:lod1Solid>
        <gml:Solid>
          <gml:exterior>
            <gml:CompositeSurface>
              <gml:surfaceMember>
                <gml:Polygon>
                  <gml:exterior>
                    <gml:LinearRing>
                      <gml:posList>0 0 0 5 0 0 5 5 0 0 5 0 0 0 0</gml:posList>
                    </gml:LinearRing>
                  </gml:exterior>
                </gml:Polygon>
              </gml:surfaceMember>
              <gml:surfaceMember>
                <gml:Polygon>
                  <gml:exterior>
                    <gml:LinearRing>
                      <gml:posList>0 0 8 5 0 8 5 5 8 0 5 8 0 0 8</gml:posList>
                    </gml:LinearRing>
                  </gml:exterior>
                </gml:Polygon>
              </gml:surfaceMember>
            </gml:CompositeSurface>
          </gml:exterior>
        </gml:Solid>
      </bldg:lod1Solid>
    </bldg:Building>
  </core:cityObjectMember>
</core:CityModel>`)

	result, err := ReadWithCRS(payload)
	if err != nil {
		t.Fatalf("ReadWithCRS: %v", err)
	}

	props := result.Collection.Features[0].Properties

	if got := props["citygml_class"]; got != "1000" {
		t.Errorf("citygml_class: want %q, got %#v", "1000", got)
	}

	if got := props["citygml_function"]; got != "residential" {
		t.Errorf("citygml_function: want %q, got %#v", "residential", got)
	}

	if got := props["citygml_usage"]; got != "office" {
		t.Errorf("citygml_usage: want %q, got %#v", "office", got)
	}

	if got := props["citygml_lod"]; got != "1" {
		t.Errorf("citygml_lod: want %q, got %#v", "1", got)
	}
}

func TestReadOmitsEmptyAttributes(t *testing.T) {
	t.Parallel()

	payload := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0"
                xmlns:bldg="http://www.opengis.net/citygml/building/2.0"
                xmlns:gml="http://www.opengis.net/gml">
  <core:cityObjectMember>
    <bldg:Building gml:id="no-attrs">
      <bldg:measuredHeight>5</bldg:measuredHeight>
      <bldg:lod1Solid>
        <gml:Solid>
          <gml:exterior>
            <gml:CompositeSurface>
              <gml:surfaceMember>
                <gml:Polygon>
                  <gml:exterior>
                    <gml:LinearRing>
                      <gml:posList>0 0 0 3 0 0 3 3 0 0 3 0 0 0 0</gml:posList>
                    </gml:LinearRing>
                  </gml:exterior>
                </gml:Polygon>
              </gml:surfaceMember>
              <gml:surfaceMember>
                <gml:Polygon>
                  <gml:exterior>
                    <gml:LinearRing>
                      <gml:posList>0 0 5 3 0 5 3 3 5 0 3 5 0 0 5</gml:posList>
                    </gml:LinearRing>
                  </gml:exterior>
                </gml:Polygon>
              </gml:surfaceMember>
            </gml:CompositeSurface>
          </gml:exterior>
        </gml:Solid>
      </bldg:lod1Solid>
    </bldg:Building>
  </core:cityObjectMember>
</core:CityModel>`)

	result, err := ReadWithCRS(payload)
	if err != nil {
		t.Fatalf("ReadWithCRS: %v", err)
	}

	props := result.Collection.Features[0].Properties

	for _, key := range []string{"citygml_class", "citygml_function", "citygml_usage"} {
		if _, exists := props[key]; exists {
			t.Errorf("expected %s to be absent, but it is present", key)
		}
	}
}

func TestImportReportCountsAndSkipReasons(t *testing.T) {
	t.Parallel()

	// Two valid buildings + one without height (will be skipped).
	payload := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0"
                xmlns:bldg="http://www.opengis.net/citygml/building/2.0"
                xmlns:gml="http://www.opengis.net/gml">
  <core:cityObjectMember>
    <bldg:Building gml:id="good-1">
      <bldg:measuredHeight>10</bldg:measuredHeight>
      <bldg:lod1Solid>
        <gml:Solid>
          <gml:exterior>
            <gml:CompositeSurface>
              <gml:surfaceMember>
                <gml:Polygon>
                  <gml:exterior>
                    <gml:LinearRing>
                      <gml:posList>0 0 0 5 0 0 5 5 0 0 5 0 0 0 0</gml:posList>
                    </gml:LinearRing>
                  </gml:exterior>
                </gml:Polygon>
              </gml:surfaceMember>
              <gml:surfaceMember>
                <gml:Polygon>
                  <gml:exterior>
                    <gml:LinearRing>
                      <gml:posList>0 0 10 5 0 10 5 5 10 0 5 10 0 0 10</gml:posList>
                    </gml:LinearRing>
                  </gml:exterior>
                </gml:Polygon>
              </gml:surfaceMember>
            </gml:CompositeSurface>
          </gml:exterior>
        </gml:Solid>
      </bldg:lod1Solid>
    </bldg:Building>
  </core:cityObjectMember>
  <core:cityObjectMember>
    <bldg:Building gml:id="good-2">
      <bldg:measuredHeight>7</bldg:measuredHeight>
      <bldg:lod1Solid>
        <gml:Solid>
          <gml:exterior>
            <gml:CompositeSurface>
              <gml:surfaceMember>
                <gml:Polygon>
                  <gml:exterior>
                    <gml:LinearRing>
                      <gml:posList>10 10 0 15 10 0 15 15 0 10 15 0 10 10 0</gml:posList>
                    </gml:LinearRing>
                  </gml:exterior>
                </gml:Polygon>
              </gml:surfaceMember>
              <gml:surfaceMember>
                <gml:Polygon>
                  <gml:exterior>
                    <gml:LinearRing>
                      <gml:posList>10 10 7 15 10 7 15 15 7 10 15 7 10 10 7</gml:posList>
                    </gml:LinearRing>
                  </gml:exterior>
                </gml:Polygon>
              </gml:surfaceMember>
            </gml:CompositeSurface>
          </gml:exterior>
        </gml:Solid>
      </bldg:lod1Solid>
    </bldg:Building>
  </core:cityObjectMember>
  <core:cityObjectMember>
    <bldg:Building gml:id="no-height">
    </bldg:Building>
  </core:cityObjectMember>
</core:CityModel>`)

	result, err := ReadWithCRS(payload)
	if err != nil {
		t.Fatalf("ReadWithCRS: %v", err)
	}

	r := result.Report
	if r.Total != 3 {
		t.Errorf("total: want 3, got %d", r.Total)
	}

	if r.Imported != 2 {
		t.Errorf("imported: want 2, got %d", r.Imported)
	}

	if r.Skipped != 1 {
		t.Errorf("skipped: want 1, got %d", r.Skipped)
	}

	if len(r.Details) != 1 {
		t.Fatalf("details: want 1 entry, got %d", len(r.Details))
	}

	if r.Details[0].ID != "no-height" {
		t.Errorf("skipped ID: want %q, got %q", "no-height", r.Details[0].ID)
	}

	if r.Details[0].Reason != SkipNoHeight {
		t.Errorf("skip reason: want %q, got %q", SkipNoHeight, r.Details[0].Reason)
	}
}

func TestImportReportPopulatedWhenAllSkipped(t *testing.T) {
	t.Parallel()

	// Single building without any geometry or height.
	payload := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0"
                xmlns:bldg="http://www.opengis.net/citygml/building/2.0"
                xmlns:gml="http://www.opengis.net/gml">
  <core:cityObjectMember>
    <bldg:Building gml:id="bad-building">
    </bldg:Building>
  </core:cityObjectMember>
</core:CityModel>`)

	result, err := ReadWithCRS(payload)
	if err == nil {
		t.Fatal("expected error when all buildings are skipped")
	}

	r := result.Report
	if r.Total != 1 {
		t.Errorf("total: want 1, got %d", r.Total)
	}

	if r.Skipped != 1 {
		t.Errorf("skipped: want 1, got %d", r.Skipped)
	}

	if r.Imported != 0 {
		t.Errorf("imported: want 0, got %d", r.Imported)
	}

	if len(r.Details) != 1 {
		t.Fatalf("details: want 1 entry, got %d", len(r.Details))
	}
}

// LGLN's LoD2 tiles are CityGML 1.0 and split most buildings into parts, each
// with its own height. Every part must become a feature, the container
// building must not be reported as skipped, and the courtyard of a plain
// building must survive as a hole.
func TestReadCityGML10BuildingParts(t *testing.T) {
	t.Parallel()

	ground := func(ring string) string {
		return `<bldg:boundedBy><bldg:GroundSurface><bldg:lod2MultiSurface><gml:MultiSurface><gml:surfaceMember>
          <gml:Polygon>` + ring + `</gml:Polygon>
        </gml:surfaceMember></gml:MultiSurface></bldg:lod2MultiSurface></bldg:GroundSurface></bldg:boundedBy>`
	}
	exterior := func(x0, y0, x1, y1 string) string {
		return `<gml:exterior><gml:LinearRing><gml:posList srsDimension="3">` +
			x0 + ` ` + y0 + ` 50 ` + x1 + ` ` + y0 + ` 50 ` + x1 + ` ` + y1 + ` 50 ` + x0 + ` ` + y1 + ` 50 ` + x0 + ` ` + y0 + ` 50` +
			`</gml:posList></gml:LinearRing></gml:exterior>`
	}

	payload := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<core:CityModel xmlns:core="http://www.opengis.net/citygml/1.0"
                xmlns:bldg="http://www.opengis.net/citygml/building/1.0"
                xmlns:gml="http://www.opengis.net/gml">
  <gml:boundedBy><gml:Envelope srsName="urn:adv:crs:ETRS89_UTM32*DE_DHHN2016_NH" srsDimension="3">
    <gml:lowerCorner>550000 5803000 40</gml:lowerCorner><gml:upperCorner>551000 5804000 90</gml:upperCorner>
  </gml:Envelope></gml:boundedBy>
  <core:cityObjectMember>
    <bldg:Building gml:id="DENI_parent">
      <bldg:function>31001_1000</bldg:function>
      <bldg:consistsOfBuildingPart><bldg:BuildingPart gml:id="DENI_part_a">
        <bldg:measuredHeight uom="urn:adv:uom:m">18.5</bldg:measuredHeight>
        ` + ground(exterior("550100", "5803100", "550110", "5803110")) + `
      </bldg:BuildingPart></bldg:consistsOfBuildingPart>
      <bldg:consistsOfBuildingPart><bldg:BuildingPart gml:id="DENI_part_b">
        <bldg:measuredHeight uom="urn:adv:uom:m">7.25</bldg:measuredHeight>
        ` + ground(exterior("550110", "5803100", "550120", "5803110")) + `
      </bldg:BuildingPart></bldg:consistsOfBuildingPart>
    </bldg:Building>
  </core:cityObjectMember>
  <core:cityObjectMember>
    <bldg:Building gml:id="DENI_courtyard">
      <bldg:measuredHeight uom="urn:adv:uom:m">12</bldg:measuredHeight>
      ` + ground(exterior("550200", "5803200", "550240", "5803240")+
		`<gml:interior><gml:LinearRing><gml:posList srsDimension="3">550210 5803210 50 550210 5803230 50 550230 5803230 50 550230 5803210 50 550210 5803210 50</gml:posList></gml:LinearRing></gml:interior>`) + `
    </bldg:Building>
  </core:cityObjectMember>
</core:CityModel>`)

	result, err := ReadWithCRS(payload)
	if err != nil {
		t.Fatalf("ReadWithCRS: %v", err)
	}

	if result.EPSGCode != 25832 {
		t.Errorf("EPSG = %d, want 25832", result.EPSGCode)
	}

	if result.Report.Total != 3 || result.Report.Imported != 3 || result.Report.Skipped != 0 {
		t.Errorf("report = %+v, want 3 candidates, all imported", result.Report)
	}

	want := []struct {
		id     string
		height float64
		parent any
		rings  int
	}{
		{"DENI_part_a", 18.5, "DENI_parent", 1},
		{"DENI_part_b", 7.25, "DENI_parent", 1},
		{"DENI_courtyard", 12, nil, 2},
	}

	if len(result.Collection.Features) != len(want) {
		t.Fatalf("features = %d, want %d", len(result.Collection.Features), len(want))
	}

	for i, w := range want {
		props := result.Collection.Features[i].Properties
		if props["id"] != w.id || props["height_m"] != w.height || props["citygml_parent_id"] != w.parent {
			t.Errorf("feature %d properties = %v, want id %s height %g parent %v", i, props, w.id, w.height, w.parent)
		}

		rings, _ := result.Collection.Features[i].Geometry.Coordinates.([]any)
		if len(rings) != w.rings {
			t.Errorf("feature %d has %d rings, want %d", i, len(rings), w.rings)
		}
	}
}

// A footprint that doubles back over itself would make validation refuse the
// whole import; the importer skips that one building and reports why.
func TestReadSkipsSelfIntersectingFootprint(t *testing.T) {
	t.Parallel()

	building := func(id, posList string) string {
		return `<core:cityObjectMember><bldg:Building gml:id="` + id + `">
      <bldg:measuredHeight>9</bldg:measuredHeight>
      <bldg:boundedBy><bldg:GroundSurface><bldg:lod2MultiSurface><gml:MultiSurface><gml:surfaceMember>
        <gml:Polygon><gml:exterior><gml:LinearRing><gml:posList srsDimension="2">` + posList + `</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon>
      </gml:surfaceMember></gml:MultiSurface></bldg:lod2MultiSurface></bldg:GroundSurface></bldg:boundedBy>
    </bldg:Building></core:cityObjectMember>`
	}

	payload := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0"
                xmlns:bldg="http://www.opengis.net/citygml/building/2.0"
                xmlns:gml="http://www.opengis.net/gml">
  ` + building("bowtie", "0 0 10 10 10 0 0 10 0 0") + `
  ` + building("square", "20 0 30 0 30 10 20 10 20 0") + `
</core:CityModel>`)

	result, err := ReadWithCRS(payload)
	if err != nil {
		t.Fatalf("ReadWithCRS: %v", err)
	}

	if len(result.Collection.Features) != 1 || result.Collection.Features[0].Properties["id"] != "square" {
		t.Fatalf("features = %+v, want only the square", result.Collection.Features)
	}

	if len(result.Report.Details) != 1 || result.Report.Details[0].Reason != SkipSelfIntersects {
		t.Errorf("skipped = %+v, want bowtie for %q", result.Report.Details, SkipSelfIntersects)
	}
}

func TestReadSkipsSelfIntersectingHole(t *testing.T) {
	t.Parallel()

	payload := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0"
                xmlns:bldg="http://www.opengis.net/citygml/building/2.0"
                xmlns:gml="http://www.opengis.net/gml">
  <core:cityObjectMember><bldg:Building gml:id="crossed-courtyard">
    <bldg:measuredHeight>9</bldg:measuredHeight>
    <bldg:boundedBy><bldg:GroundSurface><bldg:lod2MultiSurface><gml:MultiSurface><gml:surfaceMember>
      <gml:Polygon>
        <gml:exterior><gml:LinearRing><gml:posList srsDimension="2">0 0 30 0 30 30 0 30 0 0</gml:posList></gml:LinearRing></gml:exterior>
        <gml:interior><gml:LinearRing><gml:posList srsDimension="2">10 10 20 20 20 10 10 20 10 10</gml:posList></gml:LinearRing></gml:interior>
      </gml:Polygon>
    </gml:surfaceMember></gml:MultiSurface></bldg:lod2MultiSurface></bldg:GroundSurface></bldg:boundedBy>
  </bldg:Building></core:cityObjectMember>
</core:CityModel>`)

	result, err := ReadWithCRS(payload)
	if !errors.Is(err, ErrNoBuildings) {
		t.Fatalf("err = %v, want ErrNoBuildings", err)
	}

	if len(result.Report.Details) != 1 || result.Report.Details[0].Reason != SkipSelfIntersects {
		t.Errorf("skipped = %+v, want the building for %q", result.Report.Details, SkipSelfIntersects)
	}
}
