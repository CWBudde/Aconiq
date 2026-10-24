package citygmlimport

import (
	"strings"
	"testing"
)

// lod3Polygon is a gml:Polygon whose exterior is the given posList.
func lod3Polygon(posList string, attrs string) string {
	return `<gml:Polygon><gml:exterior><gml:LinearRing><gml:posList` + attrs + `>` + posList +
		`</gml:posList></gml:LinearRing></gml:exterior></gml:Polygon>`
}

func lod3Surface(kind, posList, openings string) string {
	return `<bldg:boundedBy><bldg:` + kind + `><bldg:lod3MultiSurface><gml:MultiSurface><gml:surfaceMember>` +
		lod3Polygon(posList, "") + `</gml:surfaceMember></gml:MultiSurface></bldg:lod3MultiSurface>` +
		openings + `</bldg:` + kind + `></bldg:boundedBy>`
}

func lod3Opening(kind, id, posList, attrs string) string {
	return `<bldg:opening><bldg:` + kind + ` gml:id="` + id + `"><bldg:lod3MultiSurface><gml:MultiSurface><gml:surfaceMember>` +
		lod3Polygon(posList, attrs) + `</gml:surfaceMember></gml:MultiSurface></bldg:lod3MultiSurface></bldg:` + kind + `></bldg:opening>`
}

// lod3House is a synthetic 10 m × 8 m house on ground at z = 50, its south
// wall at y = 0 carrying a window recessed 0.2 m and a door, its roof a
// skylight, and a second building whose window states no z.
func lod3House() string {
	south := lod3Opening("Window", "win-south", "2 0.2 53.9 3.2 0.2 53.9 3.2 0.2 55.4 2 0.2 55.4 2 0.2 53.9", "") +
		lod3Opening("Door", "door-1", "5 0 50 6 0 50 6 0 52.1 5 0 52.1 5 0 50", "")

	return `<?xml version="1.0" encoding="UTF-8"?>
<core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0" xmlns:bldg="http://www.opengis.net/citygml/building/2.0" xmlns:gml="http://www.opengis.net/gml">
  <core:cityObjectMember>
    <bldg:Building gml:id="house">
      <bldg:measuredHeight uom="urn:adv:uom:m">9</bldg:measuredHeight>` +
		lod3Surface("GroundSurface", "0 0 50 0 8 50 10 8 50 10 0 50 0 0 50", "") +
		lod3Surface("WallSurface", "0 0 50 10 0 50 10 0 59 0 0 59 0 0 50", south) +
		lod3Surface("RoofSurface", "0 0 59 10 0 59 10 8 59 0 8 59 0 0 59",
			lod3Opening("Window", "skylight", "4 4 59 5 4 59 5 5 59 4 5 59 4 4 59", "")) + `
    </bldg:Building>
  </core:cityObjectMember>
  <core:cityObjectMember>
    <bldg:Building gml:id="shed">
      <bldg:measuredHeight uom="urn:adv:uom:m">3</bldg:measuredHeight>` +
		lod3Surface("GroundSurface", "20 0 48 20 4 48 24 4 48 24 0 48 20 0 48", "") +
		lod3Surface("WallSurface", "20 0 48 24 0 48 24 0 51 20 0 51 20 0 48",
			lod3Opening("Window", "flat", "21 0 22 0 22 1 21 1 21 0", ` srsDimension="2"`)) + `
    </bldg:Building>
  </core:cityObjectMember>
</core:CityModel>`
}

func TestReadOpeningsFromLoD3(t *testing.T) {
	t.Parallel()

	result, err := ReadWithCRS([]byte(lod3House()))
	if err != nil {
		t.Fatalf("ReadWithCRS: %v", err)
	}

	if result.Report.Imported != 2 {
		t.Fatalf("buildings = %+v, want the house and the shed", result.Report)
	}

	if len(result.Openings) != 1 {
		t.Fatalf("openings = %+v, want the south window alone", result.Openings)
	}

	win := result.Openings[0]
	if win.ID != "win-south" || win.BuildingID != "house" || win.BaseZ != 50 || len(win.Vertices) != 5 ||
		win.Vertices[1] != [3]float64{3.2, 0.2, 53.9} {
		t.Errorf("opening = %+v, want win-south of the house, its outline in 3D, its base at the ground surface", win)
	}

	report := result.Report.Openings
	if report == nil || report.Windows != 3 || report.Doors != 1 || report.Imported != 0 || len(report.Skipped) != 2 {
		t.Fatalf("report = %+v, want 3 windows, 1 door, 2 skipped and the placing left to the caller", report)
	}

	reasons := map[string]string{}
	for _, s := range report.Skipped {
		reasons[s.ID] = s.Building + ": " + s.Reason
	}

	if !strings.Contains(reasons["skylight"], "house: ") || !strings.Contains(reasons["skylight"], "RoofSurface") {
		t.Errorf("skylight = %q, want it refused for lying in the roof", reasons["skylight"])
	}

	if !strings.Contains(reasons["flat"], "shed: ") || !strings.Contains(reasons["flat"], "2D") {
		t.Errorf("flat = %q, want it refused for stating no z", reasons["flat"])
	}

	if len(report.SkippedDoors) != 1 {
		t.Fatalf("skipped doors = %+v, want door-1 named", report.SkippedDoors)
	}

	if door := report.SkippedDoors[0]; door.ID != "door-1" || door.Building != "house" || !strings.Contains(door.Reason, "door") {
		t.Errorf("door = %+v, want door-1 of the house, with the reason it is not imported", door)
	}
}

// A document without a single opening reports none: LoD2, as LGLN
// publishes it, is the common case.
func TestReadWithoutOpenings(t *testing.T) {
	t.Parallel()

	result, err := ReadWithCRS([]byte(strings.ReplaceAll(strings.ReplaceAll(lod3House(), "Window", "Pane"), "Door", "Gate")))
	if err != nil {
		t.Fatalf("ReadWithCRS: %v", err)
	}

	if result.Openings != nil || result.Report.Openings != nil {
		t.Errorf("openings = %+v, report = %+v, want none", result.Openings, result.Report.Openings)
	}
}
