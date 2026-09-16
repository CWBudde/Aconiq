package cli

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/io/soundplanimport"
)

// The receiver expansion is tested here without the licensed fixture, because
// the fixture is absent in CI and the property being tested — one receiver per
// floor, with the floor's own height above ground — is what the comparison
// keys on.

func TestBuildSoundPlanReceiverColumn(t *testing.T) {
	t.Parallel()

	// The shape of the reference project's own data: floors 2.4 m above the
	// reference elevation and 2.8 m apart, with the cached ground height a
	// little above the reference.
	point := soundplanimport.ImmissionPoint{
		ObjID:             65710,
		Name:              "Grasmückenweg 11",
		X:                 7576.75,
		Y:                 6789.19,
		FloorRefZ:         239.77296447753906,
		GroundHeightM:     239.94715881347656,
		FloorCount:        3,
		FirstFloorOffsetM: 2.4,
		FloorSpacingM:     2.8,
		LimitDayDB:        72,
		LimitNightDB:      62,
		HasFloorAttrs:     true,
	}

	features, warnings := buildSoundPlanReceiverColumn(point, 0, 2.0)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}

	if len(features) != 3 {
		t.Fatalf("got %d receivers, want one per floor", len(features))
	}

	wantIDs := []string{
		"soundplan-receiver-65710-f1",
		"soundplan-receiver-65710-f2",
		"soundplan-receiver-65710-f3",
	}

	for i, feature := range features {
		if feature.ID != wantIDs[i] {
			t.Fatalf("receiver %d id = %q, want %q", i, feature.ID, wantIDs[i])
		}

		if feature.HeightM == nil {
			t.Fatalf("receiver %s carries no height_m", feature.ID)
		}

		wantHeight := point.FloorZ(i+1) - point.GroundHeightM
		if math.Abs(*feature.HeightM-wantHeight) > 1e-9 {
			t.Fatalf("receiver %s height_m = %v, want %v", feature.ID, *feature.HeightM, wantHeight)
		}

		if got := feature.Properties["soundplan_obj_id"]; got != int64(65710) {
			t.Fatalf("receiver %s soundplan_obj_id = %v, want 65710", feature.ID, got)
		}

		if got := feature.Properties["soundplan_floor"]; got != i+1 {
			t.Fatalf("receiver %s soundplan_floor = %v, want %d", feature.ID, got, i+1)
		}

		if got := feature.Properties["soundplan_floor_count"]; got != 3 {
			t.Fatalf("receiver %s soundplan_floor_count = %v, want 3", feature.ID, got)
		}

		if got := feature.Properties["soundplan_receiver_name"]; got != point.Name {
			t.Fatalf("receiver %s soundplan_receiver_name = %v, want %q", feature.ID, got, point.Name)
		}

		if got := feature.Properties["soundplan_limit_day_db"]; got != 72.0 {
			t.Fatalf("receiver %s soundplan_limit_day_db = %v, want 72", feature.ID, got)
		}
	}

	// The first floor of this point sits 2.23 m above ground even though the
	// floor offset is 2.4 m, because the ground is higher than the reference
	// elevation. That difference is the reason the height is derived rather
	// than taken from the project default.
	if got := *features[0].HeightM; math.Abs(got-2.2258) > 1e-3 {
		t.Fatalf("first floor height = %v, want ~2.226 m", got)
	}
}

// TestBuildSoundPlanReceiverColumnWithoutFloorAttributes covers the path that
// keeps an unrecognised binary layout importable: one receiver at the project
// default, floor 0, and a warning that names the point.
func TestBuildSoundPlanReceiverColumnWithoutFloorAttributes(t *testing.T) {
	t.Parallel()

	point := soundplanimport.ImmissionPoint{
		ObjID:     4242,
		Name:      "Hauptstraße 4",
		X:         10,
		Y:         20,
		FloorRefZ: 100,
	}

	features, warnings := buildSoundPlanReceiverColumn(point, 6, 2.0)

	if len(features) != 1 {
		t.Fatalf("got %d receivers, want 1", len(features))
	}

	if features[0].ID != "soundplan-receiver-4242-f0" {
		t.Fatalf("receiver id = %q, want soundplan-receiver-4242-f0", features[0].ID)
	}

	if features[0].HeightM == nil || *features[0].HeightM != 2.0 {
		t.Fatalf("height_m = %v, want the project default 2.0", features[0].HeightM)
	}

	if got := features[0].Properties["soundplan_floor_attributes_missing"]; got != true {
		t.Fatalf("soundplan_floor_attributes_missing = %v, want true", got)
	}

	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want one naming the point", warnings)
	}
}

// TestBuildSoundPlanReceiverColumnBelowGround covers a floor the geometry puts
// at or below ground level. The column is not expanded at all: a partial
// column would drop reference rows without saying so.
func TestBuildSoundPlanReceiverColumnBelowGround(t *testing.T) {
	t.Parallel()

	point := soundplanimport.ImmissionPoint{
		ObjID:             77,
		Name:              "Kellergeschoss 1",
		FloorRefZ:         100,
		GroundHeightM:     104,
		FloorCount:        2,
		FirstFloorOffsetM: 2.4,
		FloorSpacingM:     2.8,
		HasFloorAttrs:     true,
	}

	features, warnings := buildSoundPlanReceiverColumn(point, 0, 2.0)

	if len(features) != 1 || features[0].ID != "soundplan-receiver-77-f0" {
		t.Fatalf("features = %v, want a single fallback receiver", features)
	}

	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want one naming the point and the floor", warnings)
	}
}

// TestSoundPlanReceiverFeatureIDFallsBackToPosition checks that an undecoded
// object id does not collide: 0 is not an identity.
func TestSoundPlanReceiverFeatureIDFallsBackToPosition(t *testing.T) {
	t.Parallel()

	first := soundPlanReceiverFeatureID(soundplanimport.ImmissionPoint{}, 0, 1)
	second := soundPlanReceiverFeatureID(soundplanimport.ImmissionPoint{}, 1, 1)

	if first == second {
		t.Fatalf("two points without an object id share the id %q", first)
	}

	if first != "soundplan-receiver-p0001-f1" {
		t.Fatalf("id = %q, want soundplan-receiver-p0001-f1", first)
	}
}
