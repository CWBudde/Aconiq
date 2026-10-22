package citygmlimport

import (
	"strings"
	"testing"
)

// A LoD2 building's bldg:address becomes addr:street and addr:housenumber, the
// two properties the OSM importer writes, so `aconiq run --facade-building
// "<street> <number>"` finds a building whichever source it came from. LGLN
// tiles carry the address on the Building, also when its volumes are
// BuildingParts, so each part inherits its building's address.
func TestReadKeepsTheBuildingAddress(t *testing.T) {
	t.Parallel()

	ground := func(x0, y0, x1, y1 string) string {
		return `<bldg:measuredHeight uom="urn:adv:uom:m">9</bldg:measuredHeight>
      <bldg:boundedBy><bldg:GroundSurface><bldg:lod2MultiSurface><gml:MultiSurface><gml:surfaceMember>
        <gml:Polygon><gml:exterior><gml:LinearRing><gml:posList srsDimension="3">` +
			x0 + ` ` + y0 + ` 50 ` + x1 + ` ` + y0 + ` 50 ` + x1 + ` ` + y1 + ` 50 ` + x0 + ` ` + y1 + ` 50 ` + x0 + ` ` + y0 + ` 50` +
			`</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon>
      </gml:surfaceMember></gml:MultiSurface></bldg:lod2MultiSurface></bldg:GroundSurface></bldg:boundedBy>`
	}
	address := func(thoroughfares ...string) string {
		var out strings.Builder
		for _, th := range thoroughfares {
			out.WriteString(`<bldg:address><core:Address><core:xalAddress><xal:AddressDetails><xal:Country>
        <xal:CountryName>Germany</xal:CountryName>
        <xal:Locality Type="Town"><xal:LocalityName>Musterstadt</xal:LocalityName>` + th + `</xal:Locality>
      </xal:Country></xal:AddressDetails></core:xalAddress></core:Address></bldg:address>`)
		}

		return out.String()
	}
	street := func(name, number string) string {
		return `<xal:Thoroughfare Type="Street"><xal:ThoroughfareNumber>` + number +
			`</xal:ThoroughfareNumber><xal:ThoroughfareName Code="01600">` + name + `</xal:ThoroughfareName></xal:Thoroughfare>`
	}

	payload := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0"
                xmlns:bldg="http://www.opengis.net/citygml/building/2.0"
                xmlns:gml="http://www.opengis.net/gml"
                xmlns:xal="urn:oasis:names:tc:ciq:xsdschema:xAL:2.0">
  <core:cityObjectMember>
    <bldg:Building gml:id="parts">
      <bldg:consistsOfBuildingPart><bldg:BuildingPart gml:id="parts_a">` + ground("0", "0", "10", "10") + `</bldg:BuildingPart></bldg:consistsOfBuildingPart>
      <bldg:consistsOfBuildingPart><bldg:BuildingPart gml:id="parts_b">` + ground("10", "0", "20", "10") + `</bldg:BuildingPart></bldg:consistsOfBuildingPart>
      ` + address(street("Musterstraße", "3")) + `
    </bldg:Building>
  </core:cityObjectMember>
  <core:cityObjectMember>
    <bldg:Building gml:id="own">` + ground("30", "0", "40", "10") + address(street("Musterstraße", "5A")) + `</bldg:Building>
  </core:cityObjectMember>
  <core:cityObjectMember>
    <bldg:Building gml:id="locality_only">` + ground("50", "0", "60", "10") + address() + address("") + `</bldg:Building>
  </core:cityObjectMember>
  <core:cityObjectMember>
    <bldg:Building gml:id="two_numbers">` + ground("70", "0", "80", "10") +
		address(street("Musterstraße", "7"), street("Musterstraße", "9")) + `</bldg:Building>
  </core:cityObjectMember>
  <core:cityObjectMember>
    <bldg:Building gml:id="same_street_spelled_twice">` + ground("130", "0", "140", "10") +
		address(street("Musterstraße", "13"), street(" musterSTRAßE ", "15")) + `</bldg:Building>
  </core:cityObjectMember>
  <core:cityObjectMember>
    <bldg:Building gml:id="corner">` + ground("90", "0", "100", "10") +
		address(street("Musterstraße", "11"), street("Beispielweg", "2")) + `</bldg:Building>
  </core:cityObjectMember>
  <core:cityObjectMember>
    <bldg:Building gml:id="none">` + ground("110", "0", "120", "10") + `</bldg:Building>
  </core:cityObjectMember>
</core:CityModel>`)

	result, err := ReadWithCRS(payload)
	if err != nil {
		t.Fatalf("ReadWithCRS: %v", err)
	}

	// A corner building's two addresses cannot be spelled in two flat
	// properties without pairing each number with its street, so it carries
	// none and is selected by its id.
	want := map[string][2]any{
		"parts_a":       {"Musterstraße", "3"},
		"parts_b":       {"Musterstraße", "3"},
		"own":           {"Musterstraße", "5A"},
		"locality_only": {nil, nil},
		"two_numbers":   {"Musterstraße", "7;9"},
		// One street, however its two entries spell it: the selector folds
		// case and spaces, so the grouping must too, and keeps the first
		// spelling.
		"same_street_spelled_twice": {"Musterstraße", "13;15"},
		"corner":                    {nil, nil},
		"none":                      {nil, nil},
	}

	if len(result.Collection.Features) != len(want) {
		t.Fatalf("features = %d, want %d", len(result.Collection.Features), len(want))
	}

	for _, feature := range result.Collection.Features {
		id, _ := feature.Properties["id"].(string)

		w, ok := want[id]
		if !ok {
			t.Errorf("unexpected feature %q", id)

			continue
		}

		if got := [2]any{feature.Properties["addr:street"], feature.Properties["addr:housenumber"]}; got != w {
			t.Errorf("%s: address = %v, want %v", id, got, w)
		}
	}
}
