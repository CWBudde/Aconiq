package geo

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// windowedHouse is house b1 (10 m × 8 m, three storeys of 3 m) with windows.
// Its canonical exterior starts at (0, 0): edge 0 runs east along y = 0 and
// faces south, edge 1 runs north along x = 10.
func windowedHouse(windows ...FacadeBuildingWindow) FacadeBuilding {
	b := house("b1", rect(0, 0, 10, 8))
	b.Windows = windows

	return b
}

// bedroom is a window on edge 0 with its left edge at u = 2: 1.2 m wide, 1.5 m
// tall, its sill at 3.9 m. Its centre is at u = 2.6 and z = 4.65, its head at
// 5.4 m.
func bedroom(id string) FacadeBuildingWindow {
	return FacadeBuildingWindow{
		ID:           id,
		Edge:         0,
		Frame:        FacadeWindow{UM: 2, SillZM: 3.9, WidthM: 1.2, HeightM: 1.5},
		ReviewStatus: ReviewStatusConfirmed,
	}
}

func near(a, b float64) bool { return math.Abs(a-b) <= 1e-9 }

func TestFacadeWindowReceiversPerRule(t *testing.T) {
	t.Parallel()

	cases := []struct {
		rule   string
		y, z   float64
		basis  string
		storey int
	}{
		// RLS-19: 5 cm out, at the slab Schall 03 derives from the window
		// head, recorded as borrowed.
		{PlacementRLS19Facade, -0.05, 5.6, HeightBasisSchall03WindowHead, 1},
		// Schall 03: on the facade, 0.2 m above the window head.
		{PlacementSchall03Facade, 0, 5.6, HeightBasisWindowHead, 1},
		// TA Lärm: 0.5 m out, at the window centre.
		{PlacementTALaermWindow, -0.5, 4.65, HeightBasisWindowCentre, 1},
	}

	for _, tc := range cases {
		receivers := mustGenerate(t, facadeSet(tc.rule, 3, "b1"), windowedHouse(bedroom("win-a")))

		atWindow := onEdge(receivers, 0)
		if len(atWindow) != 1 {
			t.Fatalf("%s: %d receivers on the windowed edge, want 1: %v", tc.rule, len(atWindow), ids(atWindow))
		}

		r := atWindow[0]
		if !near(r.Point.X, 2.6) || !near(r.Point.Y, tc.y) || !near(r.HeightM, tc.z) {
			t.Errorf("%s: point (%v, %v) at %v m, want (2.6, %v) at %v m", tc.rule, r.Point.X, r.Point.Y, r.HeightM, tc.y, tc.z)
		}

		if r.HeightBasis != tc.basis || r.Storey != tc.storey || r.SlabHeightSource != "" {
			t.Errorf("%s: basis %q, storey %d, slab source %q; want %q, %d and none",
				tc.rule, r.HeightBasis, r.Storey, r.SlabHeightSource, tc.basis, tc.storey)
		}

		if r.WindowID != "win-a" || r.Window == nil || *r.Window != bedroom("").Frame {
			t.Errorf("%s: window %q %+v, want win-a and its frame", tc.rule, r.WindowID, r.Window)
		}

		if r.ID != "set/b1/p0r0e0/w/win-a" || r.ReviewStatus != ReviewStatusConfirmed || !near(r.StationUM, 2.6) {
			t.Errorf("%s: id %q, review %q, u %v", tc.rule, r.ID, r.ReviewStatus, r.StationUM)
		}

		// The facades without a window keep the station grid, except under
		// TA Lärm, whose Immissionsort only a window defines.
		others := len(receivers) - 1
		if tc.rule == PlacementTALaermWindow && others != 0 {
			t.Errorf("%s: %d receivers away from the window, want none", tc.rule, others)
		}

		if tc.rule != PlacementTALaermWindow && others == 0 {
			t.Errorf("%s: no station receivers on the facades without windows", tc.rule)
		}
	}
}

// The windows of one facade come in order along it, whatever order the model
// holds them in, and the station index counts them.
func TestFacadeWindowReceiversOrderAlongTheFacade(t *testing.T) {
	t.Parallel()

	right := bedroom("a-right")
	right.Frame.UM = 7

	receivers := onEdge(mustGenerate(t, facadeSet(PlacementTALaermWindow, 3, "b1"), windowedHouse(right, bedroom("z-left"))), 0)

	got := ids(receivers)
	want := []string{"set/b1/p0r0e0/w/z-left", "set/b1/p0r0e0/w/a-right"}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ids = %v, want %v", got, want)
	}

	if receivers[0].Station != 0 || receivers[1].Station != 1 {
		t.Errorf("stations = %d, %d; want 0, 1", receivers[0].Station, receivers[1].Station)
	}
}

// A window in the top storey stands below its ceiling slab, and one above
// the last slab counts as the top storey: the storey is a label, not a height.
func TestFacadeWindowStorey(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		sill   float64
		storey int
	}{{0.9, 0}, {6.9, 2}, {9.5, 2}} {
		w := bedroom("w")
		w.Frame.SillZM = tc.sill

		r := onEdge(mustGenerate(t, facadeSet(PlacementTALaermWindow, 3, "b1"), windowedHouse(w)), 0)
		if len(r) != 1 || r[0].Storey != tc.storey {
			t.Errorf("sill %v: %+v, want storey %d", tc.sill, r, tc.storey)
		}
	}
}

func TestFacadeWindowReceiversRefuse(t *testing.T) {
	t.Parallel()

	with := func(edit func(*FacadeBuildingWindow)) FacadeBuildingWindow {
		w := bedroom("w")
		edit(&w)

		return w
	}

	cases := map[string][]FacadeBuildingWindow{
		"no id":           {with(func(w *FacadeBuildingWindow) { w.ID = " " })},
		"slash in id":     {with(func(w *FacadeBuildingWindow) { w.ID = "w/1" })},
		"twice":           {bedroom("w"), bedroom("w")},
		"zero width":      {with(func(w *FacadeBuildingWindow) { w.Frame.WidthM = 0 })},
		"negative height": {with(func(w *FacadeBuildingWindow) { w.Frame.HeightM = -1 })},
		"nan sill":        {with(func(w *FacadeBuildingWindow) { w.Frame.SillZM = math.NaN() })},
		"negative u":      {with(func(w *FacadeBuildingWindow) { w.Frame.UM = -0.1 })},
		"past the corner": {with(func(w *FacadeBuildingWindow) { w.Frame.UM = 9.5 })},
		"no such edge":    {with(func(w *FacadeBuildingWindow) { w.Edge = 4 })},
		"no such ring":    {with(func(w *FacadeBuildingWindow) { w.Ring = 1 })},
		"no such part":    {with(func(w *FacadeBuildingWindow) { w.Part = 1 })},
		"unknown review":  {with(func(w *FacadeBuildingWindow) { w.ReviewStatus = "maybe" })},
		"rejected":        {with(func(w *FacadeBuildingWindow) { w.ReviewStatus = ReviewStatusRejected })},
	}

	for name, windows := range cases {
		_, err := facadeSet(PlacementRLS19Facade, 3, "b1").Generate([]FacadeBuilding{windowedHouse(windows...)})
		if err == nil || !strings.Contains(err.Error(), "window") {
			t.Errorf("%s: err = %v, want the window refused", name, err)
		}
	}

	// A window that ends on the corner fits.
	corner := with(func(w *FacadeBuildingWindow) { w.Frame.UM = 8.8 })
	mustGenerate(t, facadeSet(PlacementRLS19Facade, 3, "b1"), windowedHouse(corner))
}

// A courtyard window is placed only when the set includes courtyards, as a
// courtyard station is.
func TestFacadeWindowOnACourtyard(t *testing.T) {
	t.Parallel()

	b := house("b1", rect(0, 0, 20, 20), reversed(rect(5, 5, 15, 15)))
	b.Windows = []FacadeBuildingWindow{{
		ID: "court", Ring: 1, Edge: 0,
		Frame:        FacadeWindow{UM: 4, SillZM: 0.9, WidthM: 1, HeightM: 1.4},
		ReviewStatus: ReviewStatusProposed,
	}}

	set := facadeSet(PlacementTALaermWindow, 3, "b1")
	if got := mustGenerate(t, set, b); len(got) != 0 {
		t.Errorf("without courtyards: %v, want none", ids(got))
	}

	set.IncludeCourtyard = true

	got := mustGenerate(t, set, b)
	if len(got) != 1 || got[0].Facade.Ring != 1 || got[0].ReviewStatus != ReviewStatusProposed {
		t.Fatalf("with courtyards: %+v, want the courtyard window", got)
	}
}

func TestFacadeWindowJSONIsTheModelShape(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(FacadeWindow{UM: 2, SillZM: 3.9, WidthM: 1.2, HeightM: 1.5})
	if err != nil {
		t.Fatal(err)
	}

	if got := string(data); got != `{"u_m":2,"sill_z_m":3.9,"width_m":1.2,"height_m":1.5}` {
		t.Errorf("JSON = %s", got)
	}
}
