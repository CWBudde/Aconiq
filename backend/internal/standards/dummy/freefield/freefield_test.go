package freefield

import (
	"math"
	"testing"

	"github.com/soundplan/soundplan/backend/internal/geo"
)

func TestComputeReceiverLevelDBSingleSourceAtOneMeter(t *testing.T) {
	t.Parallel()

	got := ComputeReceiverLevelDB(
		geo.Point2D{X: 1, Y: 0},
		[]Source{
			{
				ID:         "s1",
				Point:      geo.Point2D{X: 0, Y: 0},
				EmissionDB: 90,
			},
		},
	)

	if math.Abs(got-90) > 1e-9 {
		t.Fatalf("expected 90 dB at 1 meter, got %.12f", got)
	}
}

func TestComputeReceiverLevelDBEnergeticSummation(t *testing.T) {
	t.Parallel()

	got := ComputeReceiverLevelDB(
		geo.Point2D{X: 0, Y: 0},
		[]Source{
			{
				ID:         "s1",
				Point:      geo.Point2D{X: 0, Y: 0},
				EmissionDB: 80,
			},
			{
				ID:         "s2",
				Point:      geo.Point2D{X: 0, Y: 0},
				EmissionDB: 80,
			},
		},
	)

	expected := 83.01029995663981
	if math.Abs(got-expected) > 1e-9 {
		t.Fatalf("expected %.12f dB, got %.12f dB", expected, got)
	}
}
