package wasmkernel_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/wasmkernel"
)

func transformJSON(t *testing.T, req wasmkernel.TransformRequest) wasmkernel.TransformResponse {
	t.Helper()

	in, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	out, err := wasmkernel.Transform(in)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}

	var resp wasmkernel.TransformResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	return resp
}

func transformError(t *testing.T, req wasmkernel.TransformRequest) string {
	t.Helper()

	in, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, err := wasmkernel.Transform(in); err != nil {
		return err.Error()
	}

	t.Fatal("transform accepted a request it should have refused")

	return ""
}

// A projected source CRS is left exactly where it is, coordinate for
// coordinate. cli.resolveComputeModel returns the model untouched in that case,
// so a round trip through an identity pipeline — which can move the last bit —
// would make browser mode disagree with the CLI over a model neither of them
// meant to change.
func TestAutoLeavesAProjectedCRSUntouched(t *testing.T) {
	t.Parallel()

	in := []float64{681_000.5, 5_646_000.25, 681_120.125, 5_646_030.75}

	resp := transformJSON(t, wasmkernel.TransformRequest{
		SourceCRS:   "EPSG:25832",
		TargetCRS:   wasmkernel.AutoTargetCRS,
		Coordinates: in,
	})

	if resp.Applied {
		t.Error("applied = true for a projected source CRS, want false")
	}

	if resp.TargetCRS != "EPSG:25832" {
		t.Errorf("target_crs = %q, want EPSG:25832", resp.TargetCRS)
	}

	for i, want := range in {
		if resp.Coordinates[i] != want {
			t.Errorf("coordinate %d = %v, want %v verbatim", i, resp.Coordinates[i], want)
		}
	}
}

// The whole point of the entry point: a geographic model resolves to the same
// zone `aconiq run` resolves it to, through the same function.
func TestAutoProjectsGeographicIntoTheZoneTheCLIWouldPick(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		lon, lat float64
		want     string
	}{
		{"zone 31", 3.5, 51.0, "EPSG:25831"},
		{"zone 32", 9.99, 53.55, "EPSG:25832"},
		{"zone 33", 13.4, 52.52, "EPSG:25833"},
		{"zone 34", 21.0, 52.2, "EPSG:25834"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp := transformJSON(t, wasmkernel.TransformRequest{
				SourceCRS:   "EPSG:4326",
				TargetCRS:   wasmkernel.AutoTargetCRS,
				Coordinates: []float64{tc.lon, tc.lat},
			})

			if !resp.Applied {
				t.Fatal("applied = false for a geographic source CRS")
			}

			if resp.TargetCRS != tc.want {
				t.Errorf("target_crs = %q, want %q", resp.TargetCRS, tc.want)
			}

			cliCRS, err := geo.ComputeCRSForGeographic(tc.lon, tc.lat)
			if err != nil {
				t.Fatalf("ComputeCRSForGeographic: %v", err)
			}

			if resp.TargetCRS != cliCRS.ID {
				t.Errorf("kernel resolved %q where the CLI resolves %q", resp.TargetCRS, cliCRS.ID)
			}
		})
	}
}

// The zone comes from the centre of the whole batch, not from its first point.
// A model straddling a zone boundary must land in one CRS, and it must be the
// one cli.resolveComputeModel picks off Model.Bounds.
func TestAutoTakesTheZoneFromTheBatchCentre(t *testing.T) {
	t.Parallel()

	// First point sits in zone 32, the centre in zone 33.
	resp := transformJSON(t, wasmkernel.TransformRequest{
		SourceCRS:   "EPSG:4326",
		TargetCRS:   wasmkernel.AutoTargetCRS,
		Coordinates: []float64{11.0, 52.0, 14.0, 52.0},
	})

	if resp.TargetCRS != "EPSG:25833" {
		t.Errorf("target_crs = %q, want EPSG:25833 (the centre's zone)", resp.TargetCRS)
	}
}

// Metres, not degrees: two points 0.001° apart in longitude are tens of metres
// apart once projected, which is what lifts them off the modules'
// minimum-distance clamp.
func TestProjectedDistancesBecomeMetres(t *testing.T) {
	t.Parallel()

	resp := transformJSON(t, wasmkernel.TransformRequest{
		SourceCRS:   "EPSG:4326",
		TargetCRS:   wasmkernel.AutoTargetCRS,
		Coordinates: []float64{9.0, 52.0, 9.001, 52.0},
	})

	before := math.Hypot(0.001, 0)
	after := math.Hypot(resp.Coordinates[2]-resp.Coordinates[0], resp.Coordinates[3]-resp.Coordinates[1])

	if before > 0.01 {
		t.Fatalf("the degree distance is %.6f, so this test is not measuring what it claims", before)
	}

	if after < 50 || after > 80 {
		t.Errorf("projected separation = %.3f m, want roughly 68 m (0.001° of longitude at 52°N)", after)
	}
}

// An explicit target always transforms, which is what makes the inverse free.
func TestExplicitTargetRoundTrips(t *testing.T) {
	t.Parallel()

	forward := transformJSON(t, wasmkernel.TransformRequest{
		SourceCRS:   "EPSG:4326",
		TargetCRS:   "EPSG:25832",
		Coordinates: []float64{9.0, 52.0},
	})

	if !forward.Applied {
		t.Error("applied = false for an explicit target")
	}

	back := transformJSON(t, wasmkernel.TransformRequest{
		SourceCRS:   "EPSG:25832",
		TargetCRS:   "EPSG:4326",
		Coordinates: forward.Coordinates,
	})

	if math.Abs(back.Coordinates[0]-9.0) > 1e-7 || math.Abs(back.Coordinates[1]-52.0) > 1e-7 {
		t.Errorf("round trip returned (%.9f, %.9f), want (9, 52)", back.Coordinates[0], back.Coordinates[1])
	}
}

// A geographic model with no coordinates has no centre to resolve a zone from.
// cli.resolveComputeModel returns early there and lets extraction report the
// empty model, which is the more useful of the two errors.
func TestAutoWithNoCoordinatesIsNotApplied(t *testing.T) {
	t.Parallel()

	resp := transformJSON(t, wasmkernel.TransformRequest{
		SourceCRS:   "EPSG:4326",
		TargetCRS:   wasmkernel.AutoTargetCRS,
		Coordinates: []float64{},
	})

	if resp.Applied {
		t.Error("applied = true for an empty batch")
	}

	if resp.TargetCRS != "EPSG:4326" {
		t.Errorf("target_crs = %q, want the source CRS back", resp.TargetCRS)
	}
}

// The refusal carries ComputeCRSForGeographic's own words, so a site outside
// zones 31-34 is refused identically by both targets.
func TestUnsupportedZoneIsRefusedInTheCLIsWords(t *testing.T) {
	t.Parallel()

	message := transformError(t, wasmkernel.TransformRequest{
		SourceCRS:   "EPSG:4326",
		TargetCRS:   wasmkernel.AutoTargetCRS,
		Coordinates: []float64{-74.0, 40.7},
	})

	_, err := geo.ComputeCRSForGeographic(-74.0, 40.7)
	if err == nil {
		t.Fatal("ComputeCRSForGeographic accepted a longitude outside zones 31-34")
	}

	if !strings.Contains(message, err.Error()) {
		t.Errorf("refusal %q does not carry ComputeCRSForGeographic's message %q", message, err.Error())
	}
}

func TestSouthernHemisphereIsRefused(t *testing.T) {
	t.Parallel()

	message := transformError(t, wasmkernel.TransformRequest{
		SourceCRS:   "EPSG:4326",
		TargetCRS:   wasmkernel.AutoTargetCRS,
		Coordinates: []float64{9.0, -33.9},
	})

	if !strings.Contains(message, "southern hemisphere") {
		t.Errorf("refusal %q does not name the southern hemisphere", message)
	}
}

func TestOddCoordinateCountIsRefused(t *testing.T) {
	t.Parallel()

	message := transformError(t, wasmkernel.TransformRequest{
		SourceCRS:   "EPSG:4326",
		TargetCRS:   wasmkernel.AutoTargetCRS,
		Coordinates: []float64{9.0, 52.0, 9.1},
	})

	if !strings.Contains(message, "even number") {
		t.Errorf("refusal %q does not explain the interleaving", message)
	}
}

func TestUnparsableCRSAreRefused(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		req  wasmkernel.TransformRequest
		want string
	}{
		{
			name: "source",
			req:  wasmkernel.TransformRequest{SourceCRS: "web map", TargetCRS: wasmkernel.AutoTargetCRS},
			want: "parse source CRS",
		},
		{
			name: "target",
			req:  wasmkernel.TransformRequest{SourceCRS: "EPSG:4326", TargetCRS: "web map"},
			want: "parse target CRS",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if message := transformError(t, tc.req); !strings.Contains(message, tc.want) {
				t.Errorf("refusal %q does not contain %q", message, tc.want)
			}
		})
	}
}

func TestInvalidJSONIsRefused(t *testing.T) {
	t.Parallel()

	if _, err := wasmkernel.Transform([]byte("not json")); err == nil {
		t.Error("Transform accepted input that is not JSON")
	}
}
