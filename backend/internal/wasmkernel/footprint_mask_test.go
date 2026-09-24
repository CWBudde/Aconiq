package wasmkernel_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/wasmkernel"
)

// The kernel adds a JSON boundary and nothing else: over the same points and
// footprints it must answer exactly what the CLI's call answers, facade and
// courtyard included.
func TestMaskFootprintsAnswersWhatTheCLIAnswers(t *testing.T) {
	t.Parallel()

	block := [][][2]float64{
		{{20, 20}, {80, 20}, {80, 80}, {20, 80}, {20, 20}},
		{{40, 40}, {60, 40}, {60, 60}, {40, 60}, {40, 40}},
	}

	var (
		flat   []float64
		points []geo.Point2D
	)

	for y := 0.0; y <= 100; y += 10 {
		for x := 0.0; x <= 100; x += 10 {
			flat = append(flat, x, y)
			points = append(points, geo.Point2D{X: x, Y: y})
		}
	}

	in, err := json.Marshal(wasmkernel.FootprintMaskRequest{Points: flat, Footprints: [][][][2]float64{block}})
	if err != nil {
		t.Fatal(err)
	}

	out, err := wasmkernel.MaskFootprints(in)
	if err != nil {
		t.Fatalf("maskFootprints: %v", err)
	}

	var resp wasmkernel.FootprintMaskResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	rings := make([][]geo.Point2D, len(block))
	for i, ring := range block {
		for _, xy := range ring {
			rings[i] = append(rings[i], geo.Point2D{X: xy[0], Y: xy[1]})
		}
	}

	var want []int

	for i, masked := range geo.MaskPointsInFootprints(points, [][][]geo.Point2D{rings}) {
		if masked {
			want = append(want, i)
		}
	}

	if len(want) != 16 {
		t.Fatalf("the fixture masks %d cells, want 16", len(want))
	}

	if !slices.Equal(resp.Masked, want) {
		t.Fatalf("kernel masked %v, CLI masks %v", resp.Masked, want)
	}
}

func TestMaskFootprintsWithNothingToMaskAnswersAnEmptyList(t *testing.T) {
	t.Parallel()

	out, err := wasmkernel.MaskFootprints([]byte(`{"points":[1,2,3,4],"footprints":[]}`))
	if err != nil {
		t.Fatal(err)
	}

	// An empty array, not null: the browser indexes into it unconditionally.
	if string(out) != `{"masked":[]}` {
		t.Fatalf("got %s", out)
	}
}

func TestMaskFootprintsRefusesAnOddCoordinateCount(t *testing.T) {
	t.Parallel()

	_, err := wasmkernel.MaskFootprints([]byte(`{"points":[1,2,3],"footprints":[]}`))
	if err == nil {
		t.Fatal("an odd coordinate count was accepted")
	}
}
