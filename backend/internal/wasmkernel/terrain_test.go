package wasmkernel_test

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/wasmkernel"
)

// The DTM these tests load: a 20x20 grid of 0.001-degree cells over Hamburg,
// i.e. a terrain imported into a geographic project, while the run computes in
// metres. That is the shape the defect took — every ElevationAt arriving in
// UTM against a grid in degrees.
const (
	degreeOriginX   = 9.99
	degreeOriginY   = 53.56
	degreePixelSize = 0.001
	degreeGridSide  = 20
)

// A point inside the grid, in degrees and in ETRS89 / UTM 32N.
const (
	hamburgLon    = 10.0
	hamburgLat    = 53.55
	hamburgEastM  = 566252.0
	hamburgNorthM = 5934021.0
)

// slopedDegreeDTM is a plane rising eastwards, so a query that lands in the
// wrong cell reads a different elevation rather than the same flat number.
func slopedDegreeDTM(t *testing.T) []byte {
	t.Helper()

	return geoTIFFBytes(t, degreeGridSide, degreeGridSide, degreeOriginX, degreeOriginY, degreePixelSize,
		func(col, _ int) float64 { return 30 + float64(col) })
}

// Loading declares the CRS because nothing else can: the GeoTIFF loader reads
// the tie point and the pixel scale, not the GeoKeyDirectory, and the compute
// request carries bare coordinates. The info that comes back is the browser's
// only description of the grid it just handed over.
func TestLoadTerrainAnswersWithTheGridsOwnExtent(t *testing.T) {
	t.Parallel()

	var store wasmkernel.TerrainStore

	raw, err := store.Load(slopedDegreeDTM(t), "EPSG:4326")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	var info terrain.Info
	if err := json.Unmarshal(raw, &info); err != nil {
		t.Fatalf("unmarshal terrain info: %v", err)
	}

	if info.GridSize != [2]int{degreeGridSide, degreeGridSide} {
		t.Errorf("grid_size = %v, want [20 20]", info.GridSize)
	}

	if !store.Loaded() {
		t.Error("the store reports no terrain after a successful load")
	}

	if store.CRS() != "EPSG:4326" {
		t.Errorf("CRS = %q, want EPSG:4326", store.CRS())
	}
}

// The CRS is required. Defaulting it to "whatever the run computes in" is the
// defect itself, written down as a default — so a caller that cannot say what
// its raster is in is refused at load, where the message still names the file
// it is about.
func TestLoadTerrainRefusesADTMWithNoCRS(t *testing.T) {
	t.Parallel()

	var store wasmkernel.TerrainStore

	_, err := store.Load(slopedDegreeDTM(t), "")
	if err == nil {
		t.Fatal("a DTM with no declared CRS was accepted")
	}

	if !strings.Contains(err.Error(), "CRS") {
		t.Errorf("the refusal does not mention the CRS: %v", err)
	}

	if store.Loaded() {
		t.Error("the refused DTM was kept anyway")
	}
}

func TestLoadTerrainRefusesACRSItCannotParse(t *testing.T) {
	t.Parallel()

	var store wasmkernel.TerrainStore

	if _, err := store.Load(slopedDegreeDTM(t), "not-a-crs"); err == nil {
		t.Fatal("a DTM declared in an unparseable CRS was accepted")
	}

	if store.Loaded() {
		t.Error("the refused DTM was kept anyway")
	}
}

// The defect, pinned. A DTM in degrees queried with metres must answer for the
// point those metres name, and the unwrapped grid must not — otherwise the run
// reads a miss, and `cli.terrainElevationAt` turns every miss into sea level.
func TestTerrainIsQueriedInItsOwnCRSNotTheComputeCRS(t *testing.T) {
	t.Parallel()

	data := slopedDegreeDTM(t)

	var store wasmkernel.TerrainStore
	if _, err := store.Load(data, "EPSG:4326"); err != nil {
		t.Fatalf("Load: %v", err)
	}

	model, err := store.InComputeCRS("EPSG:25832")
	if err != nil {
		t.Fatalf("InComputeCRS: %v", err)
	}

	elevation, ok := model.ElevationAt(hamburgEastM, hamburgNorthM)
	if !ok {
		t.Fatal("a point inside the terrain was reported as a miss; the query was passed through in metres")
	}

	// The same query against the grid itself, which is what the kernel used to
	// do: metres against a grid in degrees.
	bare, err := terrain.LoadFromBytes(data)
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	if _, ok := bare.ElevationAt(hamburgEastM, hamburgNorthM); ok {
		t.Fatal("the unwrapped grid answered a query in metres; this fixture cannot show the defect")
	}

	// And the answer is the grid's own, read at the point the transform lands
	// on — same direction, compute CRS into the terrain's.
	nativeX, nativeY := project(t, "EPSG:25832", "EPSG:4326", hamburgEastM, hamburgNorthM)

	if math.Abs(nativeX-hamburgLon) > 0.01 || math.Abs(nativeY-hamburgLat) > 0.01 {
		t.Fatalf("the fixture's metric point projects to (%.6f, %.6f), not near (%.2f, %.2f)",
			nativeX, nativeY, hamburgLon, hamburgLat)
	}

	want, ok := bare.ElevationAt(nativeX, nativeY)
	if !ok {
		t.Fatal("the transformed point is outside the grid; the fixture is wrong")
	}

	if elevation != want {
		t.Errorf("elevation = %v, want the grid's own %v at (%.6f, %.6f)",
			elevation, want, nativeX, nativeY)
	}
}

// A point genuinely outside the terrain stays a miss. The wrapper must not
// launder a refused transform, or a query far outside the projection's domain,
// into an elevation of zero that reads like data.
func TestTerrainOutsideTheGridIsStillAMiss(t *testing.T) {
	t.Parallel()

	var store wasmkernel.TerrainStore
	if _, err := store.Load(slopedDegreeDTM(t), "EPSG:4326"); err != nil {
		t.Fatalf("Load: %v", err)
	}

	model, err := store.InComputeCRS("EPSG:25832")
	if err != nil {
		t.Fatalf("InComputeCRS: %v", err)
	}

	// Roughly 200 km north of the grid.
	if _, ok := model.ElevationAt(hamburgEastM, hamburgNorthM+200000); ok {
		t.Fatal("a point outside the terrain was reported as a hit")
	}
}

// Terrain and run in the same CRS: nothing is transformed, and the proof is
// behavioural — the grid answers for its own coordinates and for nothing else.
func TestTerrainInTheComputeCRSIsQueriedDirectly(t *testing.T) {
	t.Parallel()

	// The same 20x20 grid, this time written at metric coordinates.
	data := geoTIFFBytes(t, 8, 8, 566000, 5934400, 100,
		func(col, _ int) float64 { return 30 + float64(col) })

	var store wasmkernel.TerrainStore
	if _, err := store.Load(data, "EPSG:25832"); err != nil {
		t.Fatalf("Load: %v", err)
	}

	model, err := store.InComputeCRS("EPSG:25832")
	if err != nil {
		t.Fatalf("InComputeCRS: %v", err)
	}

	if _, ok := model.ElevationAt(hamburgEastM, hamburgNorthM); !ok {
		t.Fatal("a metric query against a metric grid missed; the model was wrapped in a transform it does not need")
	}

	if _, ok := model.ElevationAt(hamburgLon, hamburgLat); ok {
		t.Fatal("a query in degrees hit a grid in metres; the model was wrapped the wrong way round")
	}
}

func TestInComputeCRSWithoutATerrainIsNotAnError(t *testing.T) {
	t.Parallel()

	var store wasmkernel.TerrainStore

	model, err := store.InComputeCRS("EPSG:25832")
	if err != nil {
		t.Fatalf("InComputeCRS: %v", err)
	}

	if model != nil {
		t.Fatalf("a store holding no terrain produced one: %T", model)
	}
}

// The request that loads no terrain is every request browser mode sends today,
// and it must be untouched: no projection, no terrain, no error.
func TestApplyLeavesATerrainlessRequestAlone(t *testing.T) {
	t.Parallel()

	var store wasmkernel.TerrainStore

	cfg, err := store.Apply(road.PropagationConfig{ReceiverTerrainZ: 12}, hamburgReceivers(), nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if cfg.TerrainModel != nil {
		t.Error("a request with no terrain loaded gained a terrain model")
	}

	if cfg.ReceiverTerrainZ != 12 {
		t.Errorf("ReceiverTerrainZ = %v, want the caller's own 12", cfg.ReceiverTerrainZ)
	}
}

// The kernel is told which CRS a run computes in; it cannot work it out from a
// request full of bare numbers. A request that does not say, while a DTM is
// loaded, is refused rather than answered from a grid queried in the wrong CRS.
func TestApplyRefusesARequestThatNamesNoComputeCRS(t *testing.T) {
	t.Parallel()

	var store wasmkernel.TerrainStore
	if _, err := store.Load(slopedDegreeDTM(t), "EPSG:4326"); err != nil {
		t.Fatalf("Load: %v", err)
	}

	for name, projection := range map[string]*wasmkernel.ComputeProjection{
		"absent":   nil,
		"CRS-less": {ProjectCRS: "EPSG:4326", Applied: false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := store.Apply(road.PropagationConfig{}, hamburgReceivers(), projection); err == nil {
				t.Fatal("a request with a loaded terrain and no compute CRS was accepted")
			}
		})
	}
}

// Both halves travel: the model itself, which is the ground h_m is measured
// above, and the one elevation the receiver heights stack on. Fixing only the
// second would leave the larger half of the defect live.
func TestApplyCarriesBothTheModelAndTheCentroidElevation(t *testing.T) {
	t.Parallel()

	var store wasmkernel.TerrainStore
	if _, err := store.Load(slopedDegreeDTM(t), "EPSG:4326"); err != nil {
		t.Fatalf("Load: %v", err)
	}

	receivers := hamburgReceivers()

	cfg, err := store.Apply(road.PropagationConfig{}, receivers, &wasmkernel.ComputeProjection{
		ProjectCRS: "EPSG:4326", ComputeCRS: "EPSG:25832", Applied: true,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if cfg.TerrainModel == nil {
		t.Fatal("the config carries no terrain model; only the centroid elevation was resolved")
	}

	want := wasmkernel.TerrainAtGridCenter(cfg.TerrainModel, receivers)
	if cfg.ReceiverTerrainZ != want {
		t.Errorf("ReceiverTerrainZ = %v, want %v", cfg.ReceiverTerrainZ, want)
	}

	if cfg.ReceiverTerrainZ == 0 {
		t.Error("ReceiverTerrainZ is 0; the centroid query missed the grid it was transformed into")
	}
}

// Shared with `cli.terrainElevationAt` on purpose: an uncovered centroid reads
// 0 in both targets. Sea level is a poor reading of "the DTM does not reach
// here", but the two targets have to agree on it — changing it is a decision
// that moves both, not a divergence introduced in the kernel.
func TestTerrainAtGridCenterReadsAMissAsZero(t *testing.T) {
	t.Parallel()

	var store wasmkernel.TerrainStore
	if _, err := store.Load(slopedDegreeDTM(t), "EPSG:4326"); err != nil {
		t.Fatalf("Load: %v", err)
	}

	model, err := store.InComputeCRS("EPSG:25832")
	if err != nil {
		t.Fatalf("InComputeCRS: %v", err)
	}

	far := []geo.PointReceiver{{
		ID:      "R1",
		Point:   geo.Point2D{X: hamburgEastM, Y: hamburgNorthM + 200000},
		HeightM: 4,
	}}

	if z := wasmkernel.TerrainAtGridCenter(model, far); z != 0 {
		t.Errorf("TerrainAtGridCenter = %v over an uncovered centroid, want 0 for parity with the CLI", z)
	}

	if z := wasmkernel.TerrainAtGridCenter(nil, hamburgReceivers()); z != 0 {
		t.Errorf("TerrainAtGridCenter = %v with no model, want 0", z)
	}

	if z := wasmkernel.TerrainAtGridCenter(model, nil); z != 0 {
		t.Errorf("TerrainAtGridCenter = %v with no receivers, want 0", z)
	}
}

// The package-level store is the one `window.aconiq` operates on. It is not
// parallel: there is exactly one of it, by design.
func TestKernelTerrainLoadsAndClears(t *testing.T) {
	if wasmkernel.TerrainLoaded() {
		t.Fatal("the kernel started with a terrain already loaded")
	}

	t.Cleanup(wasmkernel.ClearTerrain)

	if _, err := wasmkernel.LoadTerrain(slopedDegreeDTM(t), "EPSG:4326"); err != nil {
		t.Fatalf("LoadTerrain: %v", err)
	}

	if !wasmkernel.TerrainLoaded() {
		t.Fatal("the kernel reports no terrain after LoadTerrain")
	}

	cfg, err := wasmkernel.ApplyTerrain(road.PropagationConfig{}, hamburgReceivers(),
		&wasmkernel.ComputeProjection{ProjectCRS: "EPSG:4326", ComputeCRS: "EPSG:25832", Applied: true})
	if err != nil {
		t.Fatalf("ApplyTerrain: %v", err)
	}

	if cfg.TerrainModel == nil {
		t.Error("ApplyTerrain attached no terrain model")
	}

	wasmkernel.ClearTerrain()

	if wasmkernel.TerrainLoaded() {
		t.Fatal("ClearTerrain left a terrain loaded")
	}

	cfg, err = wasmkernel.ApplyTerrain(road.PropagationConfig{}, hamburgReceivers(), nil)
	if err != nil {
		t.Fatalf("ApplyTerrain after ClearTerrain: %v", err)
	}

	if cfg.TerrainModel != nil {
		t.Error("a cleared kernel still attached a terrain model")
	}
}

// cmd/wasm/main.go is `//go:build js && wasm`, so nothing in it is reachable
// from `go test`. Reading it as text is the only assertion this repository can
// make about it — the same mechanism TestEveryStandardHasItsEntryPoint uses —
// and what has to hold is that the terrain entry points are registered *and*
// that they delegate. A main.go that called terrain.LoadFromBytes itself would
// keep its own untestable copy of the state and the CRS, which is how the
// defect got there.
func TestTerrainEntryPointsDelegateToTheKernel(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile(filepath.Join("..", "..", "cmd", "wasm", "main.go"))
	if err != nil {
		t.Fatalf("read cmd/wasm/main.go: %v", err)
	}

	main := string(source)

	for _, want := range []string{
		`aconiq.Set("loadTerrain"`,
		`aconiq.Set("clearTerrain"`,
		"wasmkernel.LoadTerrain(",
		"wasmkernel.ClearTerrain()",
		"wasmkernel.ApplyTerrain(",
	} {
		if !strings.Contains(main, want) {
			t.Errorf("cmd/wasm/main.go does not contain %q", want)
		}
	}

	for _, unwanted := range []string{
		"terrain.LoadFromBytes(",
		"terrain.Model",
		"ElevationAt(",
	} {
		if strings.Contains(main, unwanted) {
			t.Errorf("cmd/wasm/main.go still holds terrain logic of its own: %q", unwanted)
		}
	}
}

func hamburgReceivers() []geo.PointReceiver {
	return []geo.PointReceiver{
		{ID: "R1", Point: geo.Point2D{X: hamburgEastM - 50, Y: hamburgNorthM}, HeightM: 4},
		{ID: "R2", Point: geo.Point2D{X: hamburgEastM + 50, Y: hamburgNorthM}, HeightM: 4},
	}
}

func project(t *testing.T, from, to string, x, y float64) (float64, float64) {
	t.Helper()

	source, err := geo.ParseCRS(from)
	if err != nil {
		t.Fatalf("parse %q: %v", from, err)
	}

	target, err := geo.ParseCRS(to)
	if err != nil {
		t.Fatalf("parse %q: %v", to, err)
	}

	pipeline, err := geo.BuildTransformPipeline(target, source)
	if err != nil {
		t.Fatalf("build %s -> %s: %v", from, to, err)
	}

	point, err := pipeline.ApplyPoint(geo.Point2D{X: x, Y: y})
	if err != nil {
		t.Fatalf("apply %s -> %s: %v", from, to, err)
	}

	return point.X, point.Y
}

// geoTIFFBytes builds an uncompressed single-strip float32 GeoTIFF in memory.
//
// Its own helper rather than a shared one: internal/geo/terrain's fixture
// builders are unexported, and internal/app/cli's writes to disk for a CLI
// invocation. This one hands back the bytes a Uint8Array would carry.
func geoTIFFBytes(
	t *testing.T,
	width, height int,
	originX, originY, pixelSize float64,
	elevation func(col, row int) float64,
) []byte {
	t.Helper()

	order := binary.LittleEndian

	const (
		bytesPerPixel = 4
		numTags       = 9
		pixelOffset   = 8
	)

	pixelDataSize := width * height * bytesPerPixel
	ifdOffset := pixelOffset + pixelDataSize
	ifdSize := 2 + numTags*12 + 4
	scaleDataOffset := ifdOffset + ifdSize
	tiepointOffset := scaleDataOffset + 24

	buf := make([]byte, tiepointOffset+48)

	buf[0], buf[1] = 'I', 'I'
	order.PutUint16(buf[2:], 42)
	order.PutUint32(buf[4:], uint32(ifdOffset))

	for row := range height {
		for col := range width {
			at := pixelOffset + (row*width+col)*bytesPerPixel
			order.PutUint32(buf[at:], math.Float32bits(float32(elevation(col, row))))
		}
	}

	pos := ifdOffset
	order.PutUint16(buf[pos:], uint16(numTags))
	pos += 2

	writeTag := func(tag, dataType uint16, count, value uint32) {
		order.PutUint16(buf[pos:], tag)
		order.PutUint16(buf[pos+2:], dataType)
		order.PutUint32(buf[pos+4:], count)
		order.PutUint32(buf[pos+8:], value)
		pos += 12
	}

	writeTag(256, 3, 1, uint32(width))              // ImageWidth
	writeTag(257, 3, 1, uint32(height))             // ImageLength
	writeTag(258, 3, 1, 32)                         // BitsPerSample
	writeTag(259, 3, 1, 1)                          // Compression = none
	writeTag(273, 4, 1, uint32(pixelOffset))        // StripOffsets
	writeTag(279, 4, 1, uint32(pixelDataSize))      // StripByteCounts
	writeTag(339, 3, 1, 3)                          // SampleFormat = float
	writeTag(33550, 12, 3, uint32(scaleDataOffset)) // ModelPixelScale
	writeTag(33922, 12, 6, uint32(tiepointOffset))  // ModelTiepoint

	order.PutUint32(buf[pos:], 0) // next IFD

	order.PutUint64(buf[scaleDataOffset:], math.Float64bits(pixelSize))
	order.PutUint64(buf[scaleDataOffset+8:], math.Float64bits(pixelSize))
	order.PutUint64(buf[scaleDataOffset+16:], 0)

	order.PutUint64(buf[tiepointOffset+24:], math.Float64bits(originX))
	order.PutUint64(buf[tiepointOffset+32:], math.Float64bits(originY))

	return buf
}
