package acoustics

import (
	"errors"
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

type testSource struct{ levelDB float64 }

// finiteReceiver places a receiver on the y = 0 axis, so a test can vary x
// alone and still say which receiver it means.
func finiteReceiver(id string, x float64) geo.PointReceiver {
	return geo.PointReceiver{ID: id, Point: geo.Point2D{X: x, Y: 0}, HeightM: 4}
}

func TestComputeReceiverOutputsKeepsTheReceiverOrder(t *testing.T) {
	t.Parallel()

	receivers := []geo.PointReceiver{
		finiteReceiver("c", 2),
		finiteReceiver("a", 0),
		finiteReceiver("b", 1),
	}

	outputs, err := ComputeReceiverOutputs(receivers, []testSource{{levelDB: 60}},
		func(receiver geo.PointReceiver, sources []testSource) (PeriodLevels, error) {
			base := sources[0].levelDB + receiver.Point.X

			return PeriodLevels{Lday: base, Levening: base - 5, Lnight: base - 10}, nil
		})
	if err != nil {
		t.Fatalf("ComputeReceiverOutputs: %v", err)
	}

	if len(outputs) != len(receivers) {
		t.Fatalf("unexpected output count: got %d want %d", len(outputs), len(receivers))
	}

	for i, output := range outputs {
		if output.Receiver.ID != receivers[i].ID {
			t.Fatalf("output %d: got receiver %q want %q", i, output.Receiver.ID, receivers[i].ID)
		}

		wantLday := 60 + receivers[i].Point.X
		if output.Indicators.Lday != wantLday {
			t.Fatalf("output %d: got Lday %v want %v", i, output.Indicators.Lday, wantLday)
		}

		// The indicator payload is filled through PeriodLevels, so Lden must be
		// the directive's composite rather than a copy of any one period.
		want := ComputeLden(PeriodLevels{Lday: wantLday, Levening: wantLday - 5, Lnight: wantLday - 10})
		if output.Indicators.Lden != want {
			t.Fatalf("output %d: got Lden %v want %v", i, output.Indicators.Lden, want)
		}
	}
}

func TestComputeReceiverOutputsRefusesUnusableReceivers(t *testing.T) {
	t.Parallel()

	levels := func(geo.PointReceiver, []testSource) (PeriodLevels, error) {
		return PeriodLevels{}, nil
	}

	cases := []struct {
		name      string
		receivers []geo.PointReceiver
		wantErr   string
	}{
		{
			name:      "no receivers",
			receivers: nil,
			wantErr:   "at least one receiver is required",
		},
		{
			name:      "a receiver without an id",
			receivers: []geo.PointReceiver{finiteReceiver("", 0)},
			wantErr:   "receiver id is required",
		},
		{
			name:      "a receiver nowhere",
			receivers: []geo.PointReceiver{{ID: "r1", Point: geo.Point2D{X: math.NaN(), Y: 0}}},
			wantErr:   `receiver "r1" coordinates are not finite`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := ComputeReceiverOutputs(testCase.receivers, []testSource{}, levels)
			if err == nil || err.Error() != testCase.wantErr {
				t.Fatalf("unexpected error: got %v want %q", err, testCase.wantErr)
			}
		})
	}
}

func TestComputeReceiverOutputsPropagatesTheModuleError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("source is not valid")

	_, err := ComputeReceiverOutputs([]geo.PointReceiver{finiteReceiver("r1", 0)}, []testSource{},
		func(geo.PointReceiver, []testSource) (PeriodLevels, error) {
			return PeriodLevels{}, sentinel
		})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected the module's error unwrapped, got %v", err)
	}
}
