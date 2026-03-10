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
