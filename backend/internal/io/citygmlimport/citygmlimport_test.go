package citygmlimport

import "testing"

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
