package wasmkernel_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/wasmkernel"
)

// The witness: an origin, a resolution and a column at which x = minX +
// col*res rounds to a different double when the product and the sum are
// rounded once (a fused multiply-add) than when each is rounded on its own.
// JavaScript always rounds twice, so browser mode's grid sits on the unfused
// value; Go fuses this expression on arm64 unless told not to. Found by
// searching col over 1..10000 for UTM-like origins and inexact resolutions;
// frontend/src/model/receiver-grid.test.ts pins the same triple.
const (
	witnessOriginX    = 412345.678
	witnessResolution = 7.7
	witnessColumn     = 8513
	witnessUnfusedX   = 477895.77800000005
)

func TestGridReceiversAreUnfused(t *testing.T) {
	t.Parallel()

	col := float64(witnessColumn)
	unfused := witnessOriginX + float64(col*witnessResolution)
	fused := math.FMA(col, witnessResolution, witnessOriginX)

	// Without this the test could pass on a witness that proves nothing.
	if unfused == fused {
		t.Fatalf("the witness does not separate fused from unfused: both %.17g", unfused)
	}

	if unfused != witnessUnfusedX {
		t.Fatalf("unfused reference = %.17g, want %.17g", unfused, witnessUnfusedX)
	}

	grid := wasmkernel.ReceiverGrid{
		OriginX: witnessOriginX, OriginY: 5_800_000, ResolutionM: witnessResolution,
		Width: witnessColumn + 1, Height: 1, HeightM: 4,
	}

	receivers := grid.Receivers()
	if got := receivers[witnessColumn].Point.X; got != witnessUnfusedX {
		t.Fatalf("x of column %d = %.17g, want the unfused %.17g (fused would be %.17g)",
			witnessColumn, got, witnessUnfusedX, fused)
	}
}

// The ids are the browser's: R + the 1-based sequence number, zero-padded to
// four digits and never truncated, so the 10 000th receiver is R10000 — what
// String(seq).padStart(4, "0") gives.
func TestGridReceiverIDsMatchPadStart(t *testing.T) {
	t.Parallel()

	grid := wasmkernel.ReceiverGrid{OriginX: 0, OriginY: 0, ResolutionM: 1, Width: 400, Height: 309, HeightM: 4}
	receivers := grid.Receivers()

	for seq, want := range map[int]string{1: "R0001", 999: "R0999", 9999: "R9999", 10000: "R10000", 123456: "R123456"} {
		if got := receivers[seq-1].ID; got != want {
			t.Errorf("receiver %d id = %q, want %q", seq, got, want)
		}
	}
}

// Row-major from the south-west corner, every coordinate a product of the
// index and the resolution rather than an accumulation, and every receiver at
// the grid's height.
func TestGridReceiversAreRowMajor(t *testing.T) {
	t.Parallel()

	grid := wasmkernel.ReceiverGrid{OriginX: 100.5, OriginY: 200.25, ResolutionM: 0.1, Width: 3, Height: 2, HeightM: 4.5}
	receivers := grid.Receivers()

	if len(receivers) != 6 {
		t.Fatalf("%d receivers, want 6", len(receivers))
	}

	for i, receiver := range receivers {
		row, col := i/3, i%3
		want := geo.PointReceiver{
			ID:      fmt.Sprintf("R%04d", i+1),
			Point:   geo.Point2D{X: 100.5 + float64(float64(col)*0.1), Y: 200.25 + float64(float64(row)*0.1)},
			HeightM: 4.5,
		}

		if receiver != want {
			t.Errorf("receiver %d = %+v, want %+v", i, receiver, want)
		}
	}
}

func TestReceiverGridRefusesWhatIsNotAGrid(t *testing.T) {
	t.Parallel()

	valid := wasmkernel.ReceiverGrid{OriginX: 0, OriginY: 0, ResolutionM: 10, Width: 2, Height: 2, HeightM: 4}

	for name, mutate := range map[string]func(*wasmkernel.ReceiverGrid){
		"zero width":          func(g *wasmkernel.ReceiverGrid) { g.Width = 0 },
		"negative height":     func(g *wasmkernel.ReceiverGrid) { g.Height = -1 },
		"zero resolution":     func(g *wasmkernel.ReceiverGrid) { g.ResolutionM = 0 },
		"NaN resolution":      func(g *wasmkernel.ReceiverGrid) { g.ResolutionM = math.NaN() },
		"infinite origin":     func(g *wasmkernel.ReceiverGrid) { g.OriginX = math.Inf(1) },
		"NaN receiver height": func(g *wasmkernel.ReceiverGrid) { g.HeightM = math.NaN() },
		"overflowing count":   func(g *wasmkernel.ReceiverGrid) { g.Width, g.Height = math.MaxInt/2, 3 },
		"a billion receivers": func(g *wasmkernel.ReceiverGrid) { g.Width, g.Height = 1_000_000_000, 1 },
		"one over the cap":    func(g *wasmkernel.ReceiverGrid) { g.Width, g.Height = geo.MaxGridReceivers+1, 1 },
	} {
		grid := valid
		mutate(&grid)

		if err := grid.Validate(); err == nil {
			t.Errorf("%s: Validate accepted %+v", name, grid)
		}
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate refused a valid grid: %v", err)
	}

	// The cap is the CLI's, and a grid exactly at it is still a grid: both
	// targets refuse the same runs.
	atCap := wasmkernel.ReceiverGrid{OriginX: 0, OriginY: 0, ResolutionM: 1, Width: geo.MaxGridReceivers / 500, Height: 500, HeightM: 4}
	if err := atCap.Validate(); err != nil {
		t.Fatalf("Validate refused a grid at the cap: %v", err)
	}
}

// A request that names its receivers as a grid must reach the compute exactly
// as the same request listing them would: the same receivers in the same
// order, and — with a terrain loaded — the same centroid elevation, which is
// derived from the receiver list and decides every level of the run.
func TestPrepareComputeRequestExpandsAGridIntoTheReceiversItNames(t *testing.T) {
	t.Parallel()

	var store wasmkernel.TerrainStore
	if _, err := store.Load(slopedDegreeDTM(t), "EPSG:4326"); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// A grid over the same patch hamburgReceivers covers, so the centroid
	// query lands on the DTM rather than reading a miss as zero.
	first := hamburgReceivers()[0].Point
	grid := wasmkernel.ReceiverGrid{
		OriginX: first.X, OriginY: first.Y, ResolutionM: 7.7, Width: 6, Height: 4, HeightM: 4,
	}

	base := map[string]any{
		"sources":    sampleSources(),
		"barriers":   sampleBarriers(),
		"config":     road.PropagationConfig{},
		"projection": wasmkernel.ComputeProjection{ProjectCRS: "EPSG:4326", ComputeCRS: "EPSG:25832", Applied: true},
	}

	viaGrid := mustPrepare(t, &store, withMember(base, "grid", grid))
	viaList := mustPrepare(t, &store, withMember(base, "receivers", grid.Receivers()))

	if !reflect.DeepEqual(viaGrid.Receivers, viaList.Receivers) {
		t.Fatal("the grid expanded to different receivers than the list it names")
	}

	if viaGrid.Config.ReceiverTerrainZ != viaList.Config.ReceiverTerrainZ || viaGrid.Config.ReceiverTerrainZ == 0 {
		t.Fatalf("centroid elevation: grid %v, list %v (and it must not be a miss)",
			viaGrid.Config.ReceiverTerrainZ, viaList.Config.ReceiverTerrainZ)
	}

	if viaGrid.Grid == nil || viaList.Grid != nil {
		t.Fatal("the prepared request must still say whether it came as a grid")
	}
}

func TestPrepareComputeRequestRefusesAmbiguousReceivers(t *testing.T) {
	t.Parallel()

	grid := wasmkernel.ReceiverGrid{OriginX: 0, OriginY: 0, ResolutionM: 10, Width: 2, Height: 2, HeightM: 4}

	var store wasmkernel.TerrainStore

	for name, request := range map[string]map[string]any{
		"grid and receivers": {"grid": grid, "receivers": grid.Receivers()},
		"an invalid grid":    {"grid": wasmkernel.ReceiverGrid{ResolutionM: 10, Width: 0, Height: 2}},
	} {
		input, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := store.PrepareComputeRequest(input); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// rls19RoadGrid answers indicators only, in chunks, and must answer exactly
// what the receiver-echoing path does: for the whole run and for every pool
// size, merged in chunk order.
func TestComputeRLS19RoadGridMatchesTheEchoingPath(t *testing.T) {
	t.Parallel()

	grid := wasmkernel.ReceiverGrid{OriginX: -90, OriginY: 25, ResolutionM: 7.7, Width: 6, Height: 4, HeightM: 4}

	var store wasmkernel.TerrainStore

	req := mustPrepare(t, &store, map[string]any{
		"grid": grid, "sources": sampleSources(), "barriers": sampleBarriers(), "config": road.PropagationConfig{},
	})

	reference, err := wasmkernel.ComputeRLS19Road(req.Receivers, req.Sources, req.Barriers, req.Config, 0, nil)
	if err != nil {
		t.Fatalf("ComputeRLS19Road: %v", err)
	}

	want := make([]road.ReceiverIndicators, len(reference))
	for i, output := range reference {
		want[i] = output.Indicators
	}

	whole, err := wasmkernel.ComputeRLS19RoadGrid(req, 0, nil)
	if err != nil {
		t.Fatalf("ComputeRLS19RoadGrid (whole run): %v", err)
	}

	if got := mergeIndicatorChunks(t, whole); !reflect.DeepEqual(got, want) {
		t.Fatal("the whole-run grid answer differs from the echoing path")
	}

	for count := 1; count <= 4; count++ {
		var chunks []wasmkernel.IndicatorChunk

		for index := range count {
			sharded := req
			sharded.Shard = &wasmkernel.Shard{Index: index, Count: count}

			part, err := wasmkernel.ComputeRLS19RoadGrid(sharded, 0, nil)
			if err != nil {
				t.Fatalf("shard %d/%d: %v", index, count, err)
			}

			chunks = append(chunks, part...)
		}

		if got := mergeIndicatorChunks(t, chunks); !reflect.DeepEqual(got, want) {
			t.Fatalf("pool of %d: merged shards differ from the echoing path", count)
		}
	}
}

func TestComputeRLS19RoadGridRefusesAReceiverList(t *testing.T) {
	t.Parallel()

	var store wasmkernel.TerrainStore

	req := mustPrepare(t, &store, map[string]any{
		"receivers": sampleReceivers(), "sources": sampleSources(), "config": road.PropagationConfig{},
	})

	_, err := wasmkernel.ComputeRLS19RoadGrid(req, 0, nil)
	if err == nil || !strings.Contains(err.Error(), "grid") {
		t.Fatalf("a request listing its receivers must be refused with a reason naming the grid, got %v", err)
	}
}

// The footprint mask takes the same descriptor, and must mask the same cells
// it masks when handed the grid's points one by one.
func TestMaskFootprintsAcceptsAGrid(t *testing.T) {
	t.Parallel()

	grid := wasmkernel.ReceiverGrid{OriginX: 0, OriginY: 0, ResolutionM: 1, Width: 5, Height: 5, HeightM: 4}
	footprints := [][][][2]float64{{{{0.5, 0.5}, {2.5, 0.5}, {2.5, 2.5}, {0.5, 2.5}, {0.5, 0.5}}}}

	points := make([]float64, 0, 50)
	for _, receiver := range grid.Receivers() {
		points = append(points, receiver.Point.X, receiver.Point.Y)
	}

	viaPoints := mustMask(t, map[string]any{"points": points, "footprints": footprints})
	viaGrid := mustMask(t, map[string]any{"grid": grid, "footprints": footprints})

	if viaPoints != viaGrid || viaGrid == `{"masked":null}` || viaGrid == `{"masked":[]}` {
		t.Fatalf("grid mask %s, point mask %s", viaGrid, viaPoints)
	}

	input, err := json.Marshal(map[string]any{"grid": grid, "points": points, "footprints": footprints})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := wasmkernel.MaskFootprints(input); err == nil {
		t.Fatal("a mask request with both a grid and points was accepted")
	}
}

func mustPrepare(t *testing.T, store *wasmkernel.TerrainStore, request map[string]any) wasmkernel.ComputeRequest {
	t.Helper()

	input, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req, err := store.PrepareComputeRequest(input)
	if err != nil {
		t.Fatalf("PrepareComputeRequest: %v", err)
	}

	return req
}

func mustMask(t *testing.T, request map[string]any) string {
	t.Helper()

	input, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	out, err := wasmkernel.MaskFootprints(input)
	if err != nil {
		t.Fatalf("MaskFootprints: %v", err)
	}

	return string(out)
}

func withMember(base map[string]any, key string, value any) map[string]any {
	out := make(map[string]any, len(base)+1)
	maps.Copy(out, base)

	out[key] = value

	return out
}

// mergeIndicatorChunks sorts chunks by index and concatenates them, checking
// each starts where the previous ended — the merge the browser's pool does.
func mergeIndicatorChunks(t *testing.T, chunks []wasmkernel.IndicatorChunk) []road.ReceiverIndicators {
	t.Helper()

	sorted := append([]wasmkernel.IndicatorChunk(nil), chunks...)
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Chunk < sorted[i].Chunk {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var merged []road.ReceiverIndicators

	for _, chunk := range sorted {
		if chunk.Start != len(merged) {
			t.Fatalf("chunk %d starts at %d, want %d", chunk.Chunk, chunk.Start, len(merged))
		}

		merged = append(merged, chunk.Outputs...)
	}

	return merged
}
