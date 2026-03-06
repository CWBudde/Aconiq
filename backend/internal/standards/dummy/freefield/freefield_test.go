package freefield

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
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

func TestDescriptorIsValid(t *testing.T) {
	t.Parallel()

	descriptor := Descriptor()

	err := descriptor.Validate()
	if err != nil {
		t.Fatalf("descriptor should validate: %v", err)
	}

	resolved, err := descriptor.ResolveVersionProfile("v0", "highres")
	if err != nil {
		t.Fatalf("resolve highres profile: %v", err)
	}

	if resolved.StandardID != StandardID {
		t.Fatalf("expected standard id %s, got %s", StandardID, resolved.StandardID)
	}

	if len(resolved.SupportedIndicators) != 1 || resolved.SupportedIndicators[0] != IndicatorLdummy {
		t.Fatalf("unexpected indicators: %#v", resolved.SupportedIndicators)
	}
}
