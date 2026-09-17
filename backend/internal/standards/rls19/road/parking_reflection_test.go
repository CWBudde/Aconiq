package road

import (
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// parkingReflectionScene puts the lot between a reflecting back wall and the
// receiver, so the mirrored path runs from behind the wall. It mirrors
// reflectedOnlyScene on the road side: the screen sits between the wall and the
// lot, where only the mirrored path can cross it.
type parkingReflectionFixture struct {
	cfg       PropagationConfig
	receiver  geo.Point2D
	reflector Reflector
	screen    Barrier
}

func parkingReflectionScene() parkingReflectionFixture {
	cfg := DefaultPropagationConfig()
	cfg.ParkingSources = []ParkingSource{{
		ID:                     "lot",
		Center:                 geo.Point2D{X: 0, Y: 0},
		AreaM2:                 1000,
		NumSpaces:              100,
		LotType:                ParkingLotLkwOmnibus,
		MovementsPerSpaceDay:   MovementRate(1.5),
		MovementsPerSpaceNight: MovementRate(0.8),
	}}

	receiver := geo.Point2D{X: 0, Y: 60}

	reflector := Reflector{
		ID:       "back-wall",
		Geometry: []geo.Point2D{{X: -200, Y: -30}, {X: 200, Y: -30}},
		HeightM:  12,
	}

	screen := Barrier{
		ID:       "screen",
		Geometry: []geo.Point2D{{X: -200, Y: -10}, {X: 200, Y: -10}},
		HeightM:  6,
	}

	return parkingReflectionFixture{cfg: cfg, receiver: receiver, reflector: reflector, screen: screen}
}

// TestParkingContributionIsReflected pins RLS-19 Eq. 3, which gives the level
// of a Parkplatzteilfläche as L_W” + 10·lg[P] − D_A − D_RV1 − D_RV2 and names
// D_RV1 the Reflexionsverlust "für die Parkplatzteilfläche j nach dem
// Abschnitt 3.6". Eq. 1 sums Teilstücke and Teilflächen "jeweils einschließlich
// etwaiger Spiegelschallquellen". A lot in front of a facade is louder than the
// same lot in the open.
func TestParkingContributionIsReflected(t *testing.T) {
	t.Parallel()

	scene := parkingReflectionScene()
	cfg, receiver, reflector := scene.cfg, scene.receiver, scene.reflector

	open, err := ComputeReceiverLevels(receiver, nil, nil, cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	cfg.Reflectors = []Reflector{reflector}

	reflected, err := ComputeReceiverLevels(receiver, nil, nil, cfg)
	if err != nil {
		t.Fatalf("reflected: %v", err)
	}

	if reflected.LrDay <= open.LrDay {
		t.Errorf("a facade must raise the parking contribution: open %.6f dB, reflected %.6f dB",
			open.LrDay, reflected.LrDay)
	}

	if reflected.LrNight <= open.LrNight {
		t.Errorf("night: open %.6f dB, reflected %.6f dB", open.LrNight, reflected.LrNight)
	}
}

// TestParkingReflectedPathIsShielded proves the lot goes through the shared
// helper rather than a second copy of it: the mirrored path of a Parkplatz is
// attenuated by a barrier exactly as a road Teilstück's is.
func TestParkingReflectedPathIsShielded(t *testing.T) {
	t.Parallel()

	scene := parkingReflectionScene()
	cfg, receiver, reflector, screen := scene.cfg, scene.receiver, scene.reflector, scene.screen
	cfg.Reflectors = []Reflector{reflector}

	unscreened, err := ComputeReceiverLevels(receiver, nil, nil, cfg)
	if err != nil {
		t.Fatalf("unscreened: %v", err)
	}

	screened, err := ComputeReceiverLevels(receiver, nil, []Barrier{screen}, cfg)
	if err != nil {
		t.Fatalf("screened: %v", err)
	}

	if screened.LrDay >= unscreened.LrDay {
		t.Errorf("a barrier across the mirrored path must lower the level: with=%.6f dB, without=%.6f dB",
			screened.LrDay, unscreened.LrDay)
	}

	// Guard: the screen must not reach the direct path, or this measures the
	// wrong thing.
	plainCfg := parkingReflectionScene().cfg

	open, err := ComputeReceiverLevels(receiver, nil, nil, plainCfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	openScreened, err := ComputeReceiverLevels(receiver, nil, []Barrier{screen}, plainCfg)
	if err != nil {
		t.Fatalf("open with screen: %v", err)
	}

	if !almostEqual(openScreened.LrDay, open.LrDay, 1e-12) {
		t.Fatalf("the screen must not touch the direct path: with=%.12f dB, without=%.12f dB",
			openScreened.LrDay, open.LrDay)
	}
}

// TestParkingSilenceSurvivesAReflector guards the −999 dB sentinel against the
// new path. A mirrored path only subtracts, so a lot with no movements stays
// below the silence threshold and is still dropped by acoustics.EnergySum.
func TestParkingSilenceSurvivesAReflector(t *testing.T) {
	t.Parallel()

	scene := parkingReflectionScene()
	cfg, receiver, reflector := scene.cfg, scene.receiver, scene.reflector
	cfg.Reflectors = []Reflector{reflector}
	cfg.ParkingSources[0].MovementsPerSpaceNight = MovementRate(0)

	levels, err := ComputeReceiverLevels(receiver, nil, nil, cfg)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}

	if levels.LrNight > silenceThresholdDB {
		t.Errorf("a lot with no night movements must stay silent, got LrNight = %.6f dB", levels.LrNight)
	}

	if levels.LrDay <= silenceThresholdDB {
		t.Fatalf("the day period must still be audible, got LrDay = %.6f dB", levels.LrDay)
	}
}
