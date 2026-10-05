package rail

import (
	"context"
	"errors"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

func TestComputeReceiverOutputsContextStopsOnACancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	receivers := []geo.PointReceiver{{ID: "r1", Point: geo.Point2D{X: 10, Y: 10}, HeightM: 4}}

	outputs, err := ComputeReceiverOutputsContext(ctx, receivers, []RailSource{}, DefaultPropagationConfig())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if outputs != nil {
		t.Fatalf("expected no outputs, got %d", len(outputs))
	}
}
