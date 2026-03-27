package road

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// Hand-calculated reference values for TestComputeParkingEmission.
//
// Eq. 10 (corrected): L_W = 63 + 10·lg[N·n] + D_{P,PT}
//
// Pkw (D=0):       N=0.5, n=100  → L_W = 63 + 10·lg(50)      = 79.9897 dB(A)
// LkwOmnibus(D=10):N=0.3, n=50   → L_W = 63 + 10·lg(15) + 10 = 84.7609 dB(A)
// Motorrad (D=5):  N=1.0, n=30   → L_W = 63 + 10·lg(30) + 5  = 82.7712 dB(A)

func TestComputeParkingEmission_Pkw(t *testing.T) {
	t.Parallel()

	source := ParkingSource{
		ID:                     "p1",
		Center:                 geo.Point2D{X: 0, Y: 0},
		AreaM2:                 2000,
		NumSpaces:              100,
		VehicleType:            ParkingPkw,
		MovementsPerSpaceDay:   0.5,
		MovementsPerSpaceNight: 0.1,
	}

	result, err := ComputeParkingEmission(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantDay := 63 + 10*math.Log10(0.5*100)
	wantNight := 63 + 10*math.Log10(0.1*100)

	if !almostEqual(result.LWDay, wantDay, 1e-6) {
		t.Fatalf("LWDay: want %.6f, got %.6f", wantDay, result.LWDay)
	}

	if !almostEqual(result.LWNight, wantNight, 1e-6) {
		t.Fatalf("LWNight: want %.6f, got %.6f", wantNight, result.LWNight)
	}
}

func TestComputeParkingEmission_LkwOmnibus(t *testing.T) {
	t.Parallel()

	source := ParkingSource{
		ID:                     "p2",
		Center:                 geo.Point2D{X: 0, Y: 0},
		AreaM2:                 5000,
		NumSpaces:              50,
		VehicleType:            ParkingLkwOmnibus,
		MovementsPerSpaceDay:   0.3,
		MovementsPerSpaceNight: 0.05,
	}

	result, err := ComputeParkingEmission(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const dPT = 10.0
	wantDay := 63 + 10*math.Log10(0.3*50) + dPT
	wantNight := 63 + 10*math.Log10(0.05*50) + dPT

	if !almostEqual(result.LWDay, wantDay, 1e-6) {
		t.Fatalf("LWDay: want %.6f, got %.6f", wantDay, result.LWDay)
	}

	if !almostEqual(result.LWNight, wantNight, 1e-6) {
		t.Fatalf("LWNight: want %.6f, got %.6f", wantNight, result.LWNight)
	}
}

func TestComputeParkingEmission_Motorrad(t *testing.T) {
	t.Parallel()

	source := ParkingSource{
		ID:                     "p3",
		Center:                 geo.Point2D{X: 0, Y: 0},
		AreaM2:                 300,
		NumSpaces:              30,
		VehicleType:            ParkingMotorrad,
		MovementsPerSpaceDay:   1.0,
		MovementsPerSpaceNight: 0.2,
	}

	result, err := ComputeParkingEmission(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const dPT = 5.0
	wantDay := 63 + 10*math.Log10(1.0*30) + dPT
	wantNight := 63 + 10*math.Log10(0.2*30) + dPT

	if !almostEqual(result.LWDay, wantDay, 1e-6) {
		t.Fatalf("LWDay: want %.6f, got %.6f", wantDay, result.LWDay)
	}

	if !almostEqual(result.LWNight, wantNight, 1e-6) {
		t.Fatalf("LWNight: want %.6f, got %.6f", wantNight, result.LWNight)
	}
}

func TestDefaultMovementsPerHour_PR(t *testing.T) {
	t.Parallel()

	got := DefaultMovementsPerHour(ParkingFacilityPR, TimePeriodDay)
	if !almostEqual(got, 0.3, 1e-9) {
		t.Fatalf("P+R day: want 0.3, got %g", got)
	}

	got = DefaultMovementsPerHour(ParkingFacilityPR, TimePeriodNight)
	if !almostEqual(got, 0.06, 1e-9) {
		t.Fatalf("P+R night: want 0.06, got %g", got)
	}
}

func TestDefaultMovementsPerHour_TankRast(t *testing.T) {
	t.Parallel()

	got := DefaultMovementsPerHour(ParkingFacilityTankRast, TimePeriodDay)
	if !almostEqual(got, 1.5, 1e-9) {
		t.Fatalf("TankRast day: want 1.5, got %g", got)
	}

	got = DefaultMovementsPerHour(ParkingFacilityTankRast, TimePeriodNight)
	if !almostEqual(got, 0.8, 1e-9) {
		t.Fatalf("TankRast night: want 0.8, got %g", got)
	}
}

func TestParkingSource_Validate(t *testing.T) {
	t.Parallel()

	valid := ParkingSource{
		ID:                     "p1",
		Center:                 geo.Point2D{X: 0, Y: 0},
		AreaM2:                 1000,
		NumSpaces:              50,
		VehicleType:            ParkingPkw,
		MovementsPerSpaceDay:   0.3,
		MovementsPerSpaceNight: 0.06,
	}

	err := valid.Validate()
	if err != nil {
		t.Fatalf("valid source should not error: %v", err)
	}

	// Missing ID.
	s := valid
	s.ID = ""

	err = s.Validate()
	if err == nil {
		t.Fatal("expected error for empty ID")
	}

	// Non-finite center.
	s = valid
	s.Center = geo.Point2D{X: math.NaN(), Y: 0}

	err = s.Validate()
	if err == nil {
		t.Fatal("expected error for non-finite center")
	}

	// Zero area.
	s = valid
	s.AreaM2 = 0

	err = s.Validate()
	if err == nil {
		t.Fatal("expected error for zero area")
	}

	// Negative area.
	s = valid
	s.AreaM2 = -1

	err = s.Validate()
	if err == nil {
		t.Fatal("expected error for negative area")
	}

	// Zero spaces.
	s = valid
	s.NumSpaces = 0

	err = s.Validate()
	if err == nil {
		t.Fatal("expected error for zero spaces")
	}

	// Negative day movements.
	s = valid
	s.MovementsPerSpaceDay = -0.1

	err = s.Validate()
	if err == nil {
		t.Fatal("expected error for negative day movements")
	}

	// Negative night movements.
	s = valid
	s.MovementsPerSpaceNight = -0.1

	err = s.Validate()
	if err == nil {
		t.Fatal("expected error for negative night movements")
	}
}

// TestComputeReceiverLevels_ParkingSourceIncreasesLevel verifies that adding a
// high-emission parking lot near the road raises the receiver level.
func TestComputeReceiverLevels_ParkingSourceIncreasesLevel(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()
	receiver := geo.Point2D{X: 0, Y: 25}
	source := sampleSource()

	baseLevels, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}

	// Parking lot between road and receiver: 500 spaces, N=1.0, at y=5.
	cfgWithParking := cfg
	cfgWithParking.ParkingSources = []ParkingSource{{
		ID:                     "park-1",
		Center:                 geo.Point2D{X: 0, Y: 5},
		AreaM2:                 5000,
		NumSpaces:              500,
		VehicleType:            ParkingPkw,
		MovementsPerSpaceDay:   1.0,
		MovementsPerSpaceNight: 0.3,
	}}

	withParking, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfgWithParking)
	if err != nil {
		t.Fatalf("with parking: %v", err)
	}

	if withParking.LrDay <= baseLevels.LrDay {
		t.Fatalf("parking should increase level: base=%.2f dB, with=%.2f dB",
			baseLevels.LrDay, withParking.LrDay)
	}
}
