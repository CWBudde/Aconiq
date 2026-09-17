package crstransform_test

import (
	"errors"
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo/crstransform"
)

// Every refusal carries a Reason, and every refusal carries its message
// unaltered.  The second half is the load-bearing one: POST /api/v1/transform
// puts this text in an error envelope and `aconiq run` prints it, so a prefix
// added here would break a parity nothing else checks.
func TestEveryRefusalIsTaggedAndKeepsItsWording(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		request crstransform.Request
		reason  crstransform.Reason
		index   int
		message string
	}{
		{
			name: "odd coordinate count",
			request: crstransform.Request{
				SourceCRS:   "EPSG:4326",
				TargetCRS:   crstransform.AutoTarget,
				Coordinates: []float64{10.0, 51.0, 10.1},
			},
			reason:  crstransform.ReasonOddCoordinateCount,
			index:   crstransform.NoIndex,
			message: "coordinates must hold an even number of values (x, y interleaved), got 3",
		},
		{
			name: "unparsable source CRS",
			request: crstransform.Request{
				SourceCRS:   "EPSG:not-a-code",
				TargetCRS:   crstransform.AutoTarget,
				Coordinates: []float64{10.0, 51.0},
			},
			reason: crstransform.ReasonSourceCRS,
			index:  crstransform.NoIndex,
		},
		{
			name: "unparsable target CRS",
			request: crstransform.Request{
				SourceCRS:   "EPSG:4326",
				TargetCRS:   "EPSG:not-a-code",
				Coordinates: []float64{10.0, 51.0},
			},
			reason: crstransform.ReasonTargetCRS,
			index:  crstransform.NoIndex,
		},
		{
			name: "geographic centre outside the supported zones",
			request: crstransform.Request{
				SourceCRS:   "EPSG:4326",
				TargetCRS:   crstransform.AutoTarget,
				Coordinates: []float64{-120.0, 37.0},
			},
			reason: crstransform.ReasonGeographicRefused,
			index:  crstransform.NoIndex,
		},
		{
			name: "coordinate the pipeline refuses",
			request: crstransform.Request{
				SourceCRS:   "EPSG:4326",
				TargetCRS:   "EPSG:25832",
				Coordinates: []float64{10.0, 51.0, math.NaN(), 51.0},
			},
			reason: crstransform.ReasonCoordinate,
			index:  1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := crstransform.Transform(testCase.request)
			if err == nil {
				t.Fatal("expected a refusal, got none")
			}

			var refusal *crstransform.Error
			if !errors.As(err, &refusal) {
				t.Fatalf("refusal is not a *crstransform.Error: %v", err)
			}

			if refusal.Reason != testCase.reason {
				t.Errorf("reason = %q, want %q", refusal.Reason, testCase.reason)
			}

			if refusal.Index != testCase.index {
				t.Errorf("index = %d, want %d", refusal.Index, testCase.index)
			}

			if testCase.message != "" && refusal.Error() != testCase.message {
				t.Errorf("message = %q, want %q", refusal.Error(), testCase.message)
			}

			if refusal.Error() != err.Error() {
				t.Errorf("tagging changed the message: %q vs %q", refusal.Error(), err.Error())
			}
		})
	}
}

// The zone refusal is quoted verbatim from geo.ComputeCRSForGeographic and
// prefixed once, here.  If this sentence moves, `aconiq run` and the browser
// kernel say something different from the local API about the same site.
func TestTheGeographicRefusalNamesTheProjectCRSAndTheCause(t *testing.T) {
	t.Parallel()

	_, err := crstransform.Transform(crstransform.Request{
		SourceCRS:   "EPSG:4326",
		TargetCRS:   crstransform.AutoTarget,
		Coordinates: []float64{10.0, -51.0},
	})
	if err == nil {
		t.Fatal("expected a refusal for a southern-hemisphere site")
	}

	const prefix = "project CRS EPSG:4326 is geographic, so the model has to be " +
		"projected before levels can be computed: "

	if got := err.Error(); len(got) <= len(prefix) || got[:len(prefix)] != prefix {
		t.Fatalf("refusal does not open with the shared sentence: %q", got)
	}
}

// A refusal must stay inspectable through errors.Is/As after it has been
// wrapped again by a caller, which is how the API handler reaches the Reason.
func TestARefusalUnwrapsToItsCause(t *testing.T) {
	t.Parallel()

	_, err := crstransform.Transform(crstransform.Request{
		SourceCRS:   "EPSG:not-a-code",
		TargetCRS:   crstransform.AutoTarget,
		Coordinates: []float64{10.0, 51.0},
	})
	if err == nil {
		t.Fatal("expected a refusal")
	}

	var refusal *crstransform.Error
	if !errors.As(err, &refusal) {
		t.Fatal("refusal is not a *crstransform.Error")
	}

	if errors.Unwrap(refusal) == nil {
		t.Error("refusal does not unwrap to the cause it was built from")
	}
}
