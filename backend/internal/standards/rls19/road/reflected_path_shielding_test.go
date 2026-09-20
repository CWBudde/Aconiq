package road

import (
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// The scenes below put the reflector on the far side of the road from the
// receiver and the barrier between the road and that reflector. The direct
// path (road → receiver) never crosses the barrier; only the mirrored path
// does, because it runs from an image source behind the reflector. That is
// what makes these assertions about the reflected leg alone.
func reflectedOnlyScene() (RoadSource, geo.Point2D, Reflector, Barrier) {
	source := sampleSource()
	receiver := geo.Point2D{X: 0, Y: 40}

	reflector := Reflector{
		ID:       "back-wall",
		Geometry: []geo.Point2D{{X: -200, Y: -30}, {X: 200, Y: -30}},
		HeightM:  12,
	}

	barrier := Barrier{
		ID:       "screen",
		Geometry: []geo.Point2D{{X: -200, Y: -10}, {X: 200, Y: -10}},
		HeightM:  6,
	}

	return source, receiver, reflector, barrier
}

// TestReflectedPath_IsShieldedLikeAnyOtherPath is the assertion the missing
// D_z on mirrored paths would have failed. RLS-19 Nr. 3.5 states the source of
// a propagation calculation may itself be a Spiegelschallquelle, so Eq. 11
// applies to it unchanged.
func TestReflectedPath_IsShieldedLikeAnyOtherPath(t *testing.T) {
	t.Parallel()

	source, receiver, reflector, barrier := reflectedOnlyScene()

	cfg := DefaultPropagationConfig()
	cfg.Reflectors = []Reflector{reflector}

	unscreened, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("reflector without barrier: %v", err)
	}

	screened, err := ComputeReceiverLevels(receiver, []RoadSource{source}, []Barrier{barrier}, cfg)
	if err != nil {
		t.Fatalf("reflector with barrier: %v", err)
	}

	if screened.LrDay >= unscreened.LrDay {
		t.Errorf("a barrier across the mirrored path must lower the level: with=%.6f dB, without=%.6f dB",
			screened.LrDay, unscreened.LrDay)
	}
}

// TestReflectedPath_BarrierLeavesTheDirectPathAlone guards the geometry the
// test above depends on. If the barrier ever started shielding the direct path
// too, that test would keep passing while measuring the wrong thing.
func TestReflectedPath_BarrierLeavesTheDirectPathAlone(t *testing.T) {
	t.Parallel()

	source, receiver, _, barrier := reflectedOnlyScene()

	cfg := DefaultPropagationConfig()

	open, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("direct path without barrier: %v", err)
	}

	withBarrier, err := ComputeReceiverLevels(receiver, []RoadSource{source}, []Barrier{barrier}, cfg)
	if err != nil {
		t.Fatalf("direct path with barrier: %v", err)
	}

	if !almostEqual(withBarrier.LrDay, open.LrDay, 1e-12) {
		t.Errorf("the barrier must not touch the direct path: with=%.12f dB, without=%.12f dB",
			withBarrier.LrDay, open.LrDay)
	}
}

// TestReflectedPath_ReflectorDoesNotShieldItself pins the exclusion in
// dropExcludedBarriers. A building is barrier and reflector at once, and the
// mirrored ray crosses its own reflecting facade by construction — so without
// the exclusion the building would invent a diffraction edge exactly where the
// standard sees a reflection.
//
// The assertion is an exact equality against the same surface modelled as a
// pure reflector: in this geometry the building shields nothing on the direct
// path, so the two scenes must agree bit for bit.
func TestReflectedPath_ReflectorDoesNotShieldItself(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	receiver := geo.Point2D{X: 0, Y: 20}

	footprint := []geo.Point2D{
		{X: -40, Y: -15}, {X: 40, Y: -15}, {X: 40, Y: -10}, {X: -40, Y: -10},
	}

	asBuilding := DefaultPropagationConfig()
	asBuilding.Buildings = []Building{{
		ID: "block", Footprint: footprint, HeightM: 12, ReflectionLossDB: 0.5,
	}}

	asReflector := DefaultPropagationConfig()
	asReflector.Reflectors = []Reflector{{
		ID: "block", Geometry: closedPolygon(footprint), HeightM: 12, ReflectionLossDB: 0.5,
	}}

	building, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, asBuilding)
	if err != nil {
		t.Fatalf("building: %v", err)
	}

	reflector, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, asReflector)
	if err != nil {
		t.Fatalf("reflector: %v", err)
	}

	if !almostEqual(building.LrDay, reflector.LrDay, 1e-12) {
		t.Errorf("a building must not shield its own mirrored path: as building=%.12f dB, as reflector=%.12f dB",
			building.LrDay, reflector.LrDay)
	}

	// Guard: the reflection has to be real, otherwise both scenes would agree
	// trivially at the unreflected level.
	open, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("open scene: %v", err)
	}

	if building.LrDay <= open.LrDay {
		t.Fatalf("the facade did not reflect: with=%.6f dB, open=%.6f dB", building.LrDay, open.LrDay)
	}
}

// TestReflectedPath_ReflectionLossIsAddedAfterTheMaximum pins D_RV outside
// Eq. 11's max{D_gr; D_z}. D_RV is a term of Eq. 2 and Eq. 3, not a candidate
// in that maximum: were it folded in, two losses both smaller than the
// shielding of this scene would leave the level untouched.
func TestReflectedPath_ReflectionLossIsAddedAfterTheMaximum(t *testing.T) {
	t.Parallel()

	source, receiver, reflector, barrier := reflectedOnlyScene()

	levelForLoss := func(lossDB float64) float64 {
		cfg := DefaultPropagationConfig()
		reflector.ReflectionLossDB = lossDB
		cfg.Reflectors = []Reflector{reflector}

		levels, err := ComputeReceiverLevels(receiver, []RoadSource{source}, []Barrier{barrier}, cfg)
		if err != nil {
			t.Fatalf("reflection loss %.1f dB: %v", lossDB, err)
		}

		return levels.LrDay
	}

	quiet := levelForLoss(5.0)
	loud := levelForLoss(0.5)

	if quiet >= loud {
		t.Errorf("reflection loss must still change a shielded mirrored path: 5.0 dB=%.6f dB, 0.5 dB=%.6f dB",
			quiet, loud)
	}
}
