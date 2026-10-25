package modelgeojson

import (
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// leveledBuilding is b1 of simpleFacadeFootprint with a storey count and,
// when source is not empty, where its height came from.
func leveledBuilding(height float64, levels any, source string) Feature {
	feature := buildingFeature("b1", simpleFacadeFootprint())
	feature.HeightM = &height
	feature.Properties = map[string]any{PropertyBuildingLevels: levels}

	if source != "" {
		feature.Properties[PropertyHeightSource] = source
	}

	return feature
}

// windowsAtSills is a model of building on edge 0, with one window per sill,
// each 1.2 m tall and side by side so every one fits the 20 m edge.
func windowsAtSills(t *testing.T, building Feature, sills ...float64) Model {
	t.Helper()

	model := facadeModel(building)

	for i, sill := range sills {
		spec := bedroomSpec(fmt.Sprintf("w%d", i))
		spec.Frame = geo.FacadeWindow{UM: float64(1 + 2*(i%9)), SillZM: sill, WidthM: 1, HeightM: 1.2}
		model.Features = append(model.Features, mustWindowFeature(t, model, spec))
	}

	return model
}

func TestBuildingLevelsReadsAPositiveInteger(t *testing.T) {
	t.Parallel()

	cases := map[any]int{
		"4": 4, " 5 ": 5, 3.0: 3, "2.5": 0, 2.5: 0, "0": 0, -1.0: 0, "four": 0, true: 0, nil: 0,
	}

	for raw, want := range cases {
		if got := BuildingLevels(raw); got != want {
			t.Errorf("BuildingLevels(%#v) = %d, want %d", raw, got, want)
		}
	}
}

// A height not derived from the storey count is compared with it: a storey
// height outside [2.2, 4.5] m is flagged, the bounds themselves are not, and
// a height the count made, or a default, never is.
func TestLevelsAgainstAMeasuredHeight(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		height float64
		levels any
		source string
		flag   bool
	}{
		{"plausible, tagged", 12, "4", "", false},
		{"at the lower bound", 8.8, "4", "measured", false},
		{"below it", 8.7, 4.0, "measured", true},
		{"at the upper bound", 13.5, "3", "measured", false},
		{"ridge over three storeys", 15, "3", "", true},
		{"derived from the count", 30, "3", geo.SlabHeightLevels, false},
		{"a default", 30, "3", geo.SlabHeightAssumed, false},
		{"no count", 30, nil, "measured", false},
		{"a fractional count", 30, "2.5", "measured", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			report := Validate(facadeModel(leveledBuilding(tc.height, tc.levels, tc.source)))
			got := issuesWithCode(report.Warnings, CodeBuildingLevelsDisagree)

			if tc.flag != (len(got) == 1) || len(report.Warnings) != len(got) || !report.Valid {
				t.Fatalf("warnings = %#v, want flagged=%v", report.Warnings, tc.flag)
			}

			if tc.flag && (got[0].FeatureID != "b1" || got[0].Facade != nil || !strings.Contains(got[0].Message, "building:levels")) {
				t.Errorf("finding = %#v, want one on building b1 as a whole", got[0])
			}
		})
	}
}

// A facade whose windows stand in more rows than the building has storeys is
// flagged; sills within 1 m of each other are one row, a gap above 1 m starts
// the next, and a rejected window counts for nothing.
func TestLevelsAgainstWindowRows(t *testing.T) {
	t.Parallel()

	// height_source levels: only the windows can disagree with the count.
	building := func(levels string) Feature { return leveledBuilding(9, levels, geo.SlabHeightLevels) }

	rejected := windowsAtSills(t, building("2"), 0.9, 3.9, 6.9)
	top := maps.Clone(rejected.Features[3].Properties)
	top[PropertyReviewStatus] = geo.ReviewStatusRejected
	rejected.Features[3].Properties = top

	cases := []struct {
		name  string
		model Model
		flag  bool
	}{
		{"three rows, three storeys", windowsAtSills(t, building("3"), 0.9, 0.95, 3.9, 6.9), false},
		{"three rows, two storeys", windowsAtSills(t, building("2"), 0.9, 3.9, 6.9), true},
		{"a gap of exactly 1 m is one row", windowsAtSills(t, building("1"), 0.9, 1.9), false},
		{"a gap above 1 m is two", windowsAtSills(t, building("1"), 0.9, 1.91), true},
		{"a rejected window is not a row", rejected, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			report := Validate(tc.model)
			got := issuesWithCode(report.Warnings, CodeBuildingLevelsDisagree)

			if tc.flag != (len(got) == 1) || len(report.Warnings) != len(got) {
				t.Fatalf("warnings = %#v, want flagged=%v", report.Warnings, tc.flag)
			}

			if tc.flag && !strings.Contains(got[0].Message, "(part 0, ring 0, edge 0)") {
				t.Errorf("message %q does not name the facade", got[0].Message)
			}
		})
	}
}

// A facade another building stands against from end to end, where facade
// mode places nothing, is flagged when it carries a window; one with a free
// stretch is not, nor is one whose only window is rejected.
func TestFacadeWithWindowsAndNoCandidate(t *testing.T) {
	t.Parallel()

	b1 := buildingFeature("b1", simpleFacadeFootprint())
	// Edge 0 runs along y0 from x0 to x0 + 20; a neighbour south of it
	// covers all of it, or only its western half.
	whole := buildingFeature("n1", [][][]geo.Point2D{{facadeRect(facadeX0-1, facadeY0-8, facadeX0+21, facadeY0)}})
	half := buildingFeature("n1", [][][]geo.Point2D{{facadeRect(facadeX0, facadeY0-8, facadeX0+10, facadeY0)}})

	withWindow := func(neighbour Feature, status string) Model {
		model := facadeModel(b1, neighbour)
		spec := bedroomSpec("w1")
		spec.ReviewStatus = status
		model.Features = append(model.Features, mustWindowFeature(t, model, spec))

		return model
	}

	cases := map[string]struct {
		model Model
		flag  bool
	}{
		"against all of it":   {withWindow(whole, ""), true},
		"a free stretch":      {withWindow(half, ""), false},
		"no window":           {facadeModel(b1, whole), false},
		"its window rejected": {withWindow(whole, geo.ReviewStatusRejected), false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			report := Validate(tc.model)
			got := issuesWithCode(report.Warnings, CodeFacadeWindowsNoCandidate)

			if tc.flag != (len(got) == 1) || len(report.Warnings) != len(got) {
				t.Fatalf("warnings = %#v, want flagged=%v", report.Warnings, tc.flag)
			}

			if tc.flag && (got[0].FeatureID != "b1" || got[0].Facade == nil || *got[0].Facade != (IssueFacade{})) {
				t.Errorf("finding = %#v, want one on facade (0, 0, 0) of b1", got[0])
			}
		})
	}
}

// A window outside its facade's extent is already reported (P18 D3's first
// check): past the far end as window.frame.outside, before the near end or
// below the ground as window.invalid.
func TestWindowOutsideItsFacadeExtent(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		member string
		value  float64
		code   string
		errors bool
	}{
		"past the far end":  {"u_m", 19.5, CodeWindowOutsideFacade, false},
		"before the start":  {"u_m", -0.5, CodeWindowInvalid, true},
		"below the ground":  {"sill_z_m", -0.5, CodeWindowInvalid, true},
		"within the facade": {"u_m", 18, "", false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			model, window := oneWindow(t)
			window.Properties[PropertyWindow] = frameWith(tc.member, tc.value)
			model.Features[1] = window

			report := Validate(model)

			issues := report.Warnings
			if tc.errors {
				issues = report.Errors
			}

			if tc.code == "" {
				assertCleanReport(t, report)

				return
			}

			if got := issuesWithCode(issues, tc.code); len(got) != 1 || got[0].FeatureID != "w1" {
				t.Fatalf("report = %#v, want one %s on w1", report, tc.code)
			}
		})
	}
}
