package terrain

import (
	"encoding/binary"
	"math"
	"testing"
)

// float32GeoTIFF is a shorthand for an uncompressed single-strip float32 raster
// with the given values, laid out row-major.
func float32GeoTIFF(t *testing.T, width, height int, values []float64, originX, originY, pixelSize float64, noData string) []byte {
	t.Helper()

	pixels := encodeSamples(binary.LittleEndian, values, 4, func(o binary.ByteOrder, b []byte, v float64) {
		o.PutUint32(b, math.Float32bits(float32(v)))
	})

	return buildGeoTIFF(t, tiffSpec{
		width: width, height: height,
		bitsPerSample: 32,
		sampleFormat:  sampleFormatFloat,
		pixels:        pixels,
		originX:       originX, originY: originY,
		pixelSizeX: pixelSize, pixelSizeY: pixelSize,
		noData: noData,
	})
}

// ElevationAt returns (value, ok) and turns every miss into a bare 0. A caller
// that ignores ok therefore reads sea level wherever the DTM does not cover —
// and a real 0 m elevation is indistinguishable from that miss unless ok is
// read. This is the invariant the rest of the engine depends on, so it is
// pinned from both sides: a genuine zero must come back ok, and a miss must
// come back not-ok with the same numeric value.
func TestElevationAtDistinguishesARealZeroFromAMiss(t *testing.T) {
	t.Parallel()

	// A 4x4 raster whose top-left quadrant is a genuine 0 m plain and whose
	// bottom-right corner is nodata. They are far enough apart that the
	// four-neighbour interpolation around the plain never reads the gap.
	//
	//   0     0     0     0
	//   0     0     0     0
	//   0     0     10    20
	//   0     0     30    -9999
	values := []float64{
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 10, 20,
		0, 0, 30, -9999,
	}

	// Pixel (0,0) is centred at (0, 30); pixel size 10.
	model, err := LoadFromBytes(float32GeoTIFF(t, 4, 4, values, 0, 30, 10, "-9999"))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	// Pixel (0,0) really is 0 m.
	elevation, ok := model.ElevationAt(0, 30)
	if !ok {
		t.Fatal("a pixel holding a real 0 m elevation was reported as a miss")
	}

	if elevation != 0 {
		t.Fatalf("elevation = %v, want 0", elevation)
	}

	// Pixel (3,3) is nodata. The value returned is also 0, so only ok tells
	// the two apart.
	elevation, ok = model.ElevationAt(30, 0)
	if ok {
		t.Fatalf("a nodata pixel was reported as covered with elevation %v", elevation)
	}

	if elevation != 0 {
		t.Fatalf("a miss returned %v; the contract is a bare 0", elevation)
	}

	// Far outside the raster is the other kind of miss, and looks identical.
	elevation, ok = model.ElevationAt(10000, 10000)
	if ok {
		t.Fatalf("a point far outside the raster was reported as covered with elevation %v", elevation)
	}

	if elevation != 0 {
		t.Fatalf("an out-of-bounds miss returned %v; the contract is a bare 0", elevation)
	}
}

// Bilinear interpolation reads four neighbours, so a nodata cell poisons every
// query whose cell touches it — not only the query exactly on top of it. A
// reader that assumed otherwise would interpolate against -9999 and produce a
// deep pit next to the gap.
func TestElevationAtRefusesAnyCellTouchingNoData(t *testing.T) {
	t.Parallel()

	// 5x5, nodata at the centre pixel (2,2). Pixel (0,0) is centred at (0, 40),
	// pixel size 10, so pixel (c,r) sits at (10c, 40-10r).
	values := []float64{
		10, 11, 12, 13, 14,
		15, 16, 17, 18, 19,
		20, 21, -9999, 23, 24,
		25, 26, 27, 28, 29,
		30, 31, 32, 33, 34,
	}

	model, err := LoadFromBytes(float32GeoTIFF(t, 5, 5, values, 0, 40, 10, "-9999"))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	// The four cells around the centre pixel all include it as a corner, so
	// none of them can be interpolated. Their midpoints are half a pixel from
	// the centre in each direction.
	poisoned := [][2]float64{{15, 25}, {25, 25}, {15, 15}, {25, 15}}
	for _, point := range poisoned {
		elevation, ok := model.ElevationAt(point[0], point[1])
		if ok {
			t.Fatalf("query at %v returned %v although its cell touches nodata", point, elevation)
		}
	}

	// The next cell out does not touch the gap and must still resolve. The
	// midpoint of the cell between pixels (0,0) and (1,1) is (5, 35).
	elevation, ok := model.ElevationAt(5, 35)
	if !ok {
		t.Fatal("a cell that does not touch nodata was reported as a miss")
	}

	if want := (10.0 + 11 + 15 + 16) / 4; elevation != want {
		t.Fatalf("elevation at (5, 35) = %v, want %v", elevation, want)
	}

	// The nodata pixel itself is a miss, and so is every point in its own
	// cells — but the raster beyond them is unaffected.
	if _, ok := model.ElevationAt(40, 40); !ok {
		t.Fatal("the far corner, four pixels from the gap, was reported as a miss")
	}
}

// NaN is treated as nodata whatever the declared nodata value is: a NaN sample
// would otherwise propagate through the interpolation and make every downstream
// level NaN.
func TestElevationAtTreatsNaNAsNoData(t *testing.T) {
	t.Parallel()

	values := []float64{10, 11, math.NaN(), 13}

	model, err := LoadFromBytes(float32GeoTIFF(t, 2, 2, values, 0, 10, 10, "-9999"))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	elevation, ok := model.ElevationAt(0, 0)
	if ok {
		t.Fatalf("a NaN sample was reported as covered with elevation %v", elevation)
	}

	if math.IsNaN(elevation) {
		t.Fatal("a miss returned NaN; the contract is a bare 0")
	}
}

// Without a GDAL_NODATA tag there is no nodata value, so a raster that happens
// to hold -9999 as a real elevation must return it rather than report a miss.
func TestElevationAtWithoutTheNoDataTagReturnsEveryValue(t *testing.T) {
	t.Parallel()

	values := []float64{-9999, 11, 12, 13}

	// 25 m pixels, the resolution of the German DGM25 series.
	model, err := LoadFromBytes(float32GeoTIFF(t, 2, 2, values, 0, 25, 25, ""))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	if got := model.Info().PixelSize; got != [2]float64{25, 25} {
		t.Fatalf("PixelSize = %v, want [25 25]", got)
	}

	elevation, ok := model.ElevationAt(0, 25)
	if !ok {
		t.Fatal("a raster without a nodata tag reported a miss")
	}

	if elevation != -9999 {
		t.Fatalf("elevation = %v, want -9999", elevation)
	}
}

// The coverage of a raster runs half a pixel past the outermost pixel centres,
// because a sample describes the cell around its centre. Getting that margin
// wrong by any amount changes which receivers a terrain-aware run can answer
// for, so the boundary is pinned exactly: on it is inside, past it is outside.
func TestElevationAtBoundaryIsHalfAPixelPastTheOutermostCentres(t *testing.T) {
	t.Parallel()

	// 2x2, pixel size 10, pixel (0,0) centred at (0, 10).
	// Bounds are therefore [-5, -5] to [15, 15].
	values := []float64{10, 20, 30, 40}

	model, err := LoadFromBytes(float32GeoTIFF(t, 2, 2, values, 0, 10, 10, ""))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	if got := model.Bounds(); got != [4]float64{-5, -5, 15, 15} {
		t.Fatalf("Bounds() = %v, want [-5 -5 15 15]", got)
	}

	inside := []struct {
		name string
		x, y float64
		want float64
	}{
		// Exactly on each corner of the envelope. Outside the pixel centres,
		// the interpolation clamps to the nearest sample.
		{name: "upper-left corner", x: -5, y: 15, want: 10},
		{name: "upper-right corner", x: 15, y: 15, want: 20},
		{name: "lower-left corner", x: -5, y: -5, want: 30},
		{name: "lower-right corner", x: 15, y: -5, want: 40},
		// On the edges, halfway along.
		{name: "top edge midpoint", x: 5, y: 15, want: 15},
		{name: "left edge midpoint", x: -5, y: 5, want: 20},
	}

	for _, testCase := range inside {
		t.Run("inside/"+testCase.name, func(t *testing.T) {
			t.Parallel()

			got, ok := model.ElevationAt(testCase.x, testCase.y)
			if !ok {
				t.Fatalf("(%g, %g) is on the boundary and must be covered", testCase.x, testCase.y)
			}

			if math.Abs(got-testCase.want) > 1e-9 {
				t.Fatalf("elevation at (%g, %g) = %v, want %v", testCase.x, testCase.y, got, testCase.want)
			}
		})
	}

	// A nanometre past the envelope in each direction must be a miss. The
	// margin is small enough that only a strict bound passes, and large enough
	// that the subtraction inside ElevationAt cannot round it away: the Y
	// comparison goes through (originY - y), where a one-ulp offset is lost to
	// rounding before the bound is ever applied.
	const epsilon = 1e-9

	outside := []struct {
		name string
		x, y float64
	}{
		{name: "just left", x: -5 - epsilon, y: 5},
		{name: "just right", x: 15 + epsilon, y: 5},
		{name: "just below", x: 5, y: -5 - epsilon},
		{name: "just above", x: 5, y: 15 + epsilon},
	}

	for _, testCase := range outside {
		t.Run("outside/"+testCase.name, func(t *testing.T) {
			t.Parallel()

			got, ok := model.ElevationAt(testCase.x, testCase.y)
			if ok {
				t.Fatalf("(%v, %v) is past the envelope but returned %v", testCase.x, testCase.y, got)
			}
		})
	}
}

// The interpolated surface must be exactly bilinear between the four samples of
// a cell: the weights are what a barrier-height or ground-effect calculation
// reads, and an off-by-one in the pixel indices would show up here as a value
// from the wrong neighbour rather than as an error.
func TestElevationAtIsBilinearWithinACell(t *testing.T) {
	t.Parallel()

	// A 2x2 cell with distinct corner values, pixel size 10, pixel (0,0)
	// centred at (0, 10).
	//   (0,10)=10   (10,10)=20
	//   (0, 0)=30   (10, 0)=70
	values := []float64{10, 20, 30, 70}

	model, err := LoadFromBytes(float32GeoTIFF(t, 2, 2, values, 0, 10, 10, ""))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	bilinear := func(fx, fy float64) float64 {
		return 10*(1-fx)*(1-fy) + 20*fx*(1-fy) + 30*(1-fx)*fy + 70*fx*fy
	}

	for _, fy := range []float64{0, 0.125, 0.25, 0.5, 0.75, 0.9, 1} {
		for _, fx := range []float64{0, 0.125, 0.25, 0.5, 0.75, 0.9, 1} {
			x := fx * 10
			y := 10 - fy*10

			got, ok := model.ElevationAt(x, y)
			if !ok {
				t.Fatalf("(%g, %g) is inside the cell but reported a miss", x, y)
			}

			want := bilinear(fx, fy)
			if math.Abs(got-want) > 1e-9 {
				t.Fatalf("elevation at (%g, %g) = %v, want %v", x, y, got, want)
			}
		}
	}
}

// Inside the half-pixel skirt beyond the outermost centres there is no second
// sample to interpolate against, so the value must be held flat rather than
// extrapolated: extrapolating a steep edge cell would invent terrain that the
// DTM never claimed.
func TestElevationAtClampsInsideTheHalfPixelSkirt(t *testing.T) {
	t.Parallel()

	// A steep left-to-right ramp in a 2x1 raster.
	values := []float64{0, 100}

	model, err := LoadFromBytes(float32GeoTIFF(t, 2, 1, values, 0, 0, 10, ""))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	for _, x := range []float64{-5, -4, -1, 0} {
		got, ok := model.ElevationAt(x, 0)
		if !ok {
			t.Fatalf("x=%g is inside the envelope", x)
		}

		if got != 0 {
			t.Fatalf("elevation at x=%g is %v; the skirt must hold the edge sample 0", x, got)
		}
	}

	for _, x := range []float64{10, 11, 14, 15} {
		got, ok := model.ElevationAt(x, 0)
		if !ok {
			t.Fatalf("x=%g is inside the envelope", x)
		}

		if got != 100 {
			t.Fatalf("elevation at x=%g is %v; the skirt must hold the edge sample 100", x, got)
		}
	}
}

// Info is what the CLI and the API report about an imported DTM, so it has to
// agree with Bounds and with the file rather than be assembled separately.
func TestInfoAgreesWithBoundsAndTheFile(t *testing.T) {
	t.Parallel()

	values := make([]float64, 4*3)
	for i := range values {
		values[i] = float64(i)
	}

	pixels := encodeSamples(binary.LittleEndian, values, 4, func(o binary.ByteOrder, b []byte, v float64) {
		o.PutUint32(b, math.Float32bits(float32(v)))
	})

	model, err := LoadFromBytes(buildGeoTIFF(t, tiffSpec{
		width: 4, height: 3,
		bitsPerSample: 32,
		sampleFormat:  sampleFormatFloat,
		pixels:        pixels,
		originX:       1000, originY: 2000,
		pixelSizeX: 20, pixelSizeY: 5,
	}))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	info := model.Info()

	if info.GridSize != [2]int{4, 3} {
		t.Fatalf("GridSize = %v, want [4 3]", info.GridSize)
	}

	if info.PixelSize != [2]float64{20, 5} {
		t.Fatalf("PixelSize = %v, want [20 5]", info.PixelSize)
	}

	if info.Bounds != model.Bounds() {
		t.Fatalf("Info().Bounds = %v but Bounds() = %v", info.Bounds, model.Bounds())
	}

	// 4 columns of 20 m from the centre at 1000, plus half a pixel each side.
	want := [4]float64{990, 1987.5, 1070, 2002.5}
	if info.Bounds != want {
		t.Fatalf("Bounds = %v, want %v", info.Bounds, want)
	}

	// Every corner of the declared envelope must be queryable: a Bounds that
	// claims more coverage than ElevationAt will answer for is a lie the
	// receiver-grid builder would act on.
	for _, corner := range [][2]float64{
		{want[0], want[1]}, {want[0], want[3]}, {want[2], want[1]}, {want[2], want[3]},
	} {
		_, ok := model.ElevationAt(corner[0], corner[1])
		if !ok {
			t.Fatalf("corner %v of the declared bounds is not covered", corner)
		}
	}
}

// A single-pixel raster degenerates every interpolation weight to the same
// sample, and both clamp branches run on the same pixel index.
func TestElevationAtOnASinglePixelRaster(t *testing.T) {
	t.Parallel()

	model, err := LoadFromBytes(float32GeoTIFF(t, 1, 1, []float64{42.5}, 100, 200, 10, ""))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	if got := model.Bounds(); got != [4]float64{95, 195, 105, 205} {
		t.Fatalf("Bounds() = %v, want [95 195 105 205]", got)
	}

	for _, point := range [][2]float64{{100, 200}, {95, 195}, {105, 205}, {95, 205}, {105, 195}} {
		got, ok := model.ElevationAt(point[0], point[1])
		if !ok {
			t.Fatalf("%v is inside a single-pixel raster's envelope", point)
		}

		if got != 42.5 {
			t.Fatalf("elevation at %v = %v, want 42.5", point, got)
		}
	}

	if _, ok := model.ElevationAt(106, 200); ok {
		t.Fatal("a point past the envelope of a single-pixel raster was reported as covered")
	}
}

// Anisotropic pixels mean the X and Y bounds checks use different scales. A
// single shared pixel size would pass a square-pixel suite and silently accept
// or reject the wrong points here.
func TestElevationAtBoundaryWithAnisotropicPixels(t *testing.T) {
	t.Parallel()

	values := []float64{10, 20, 30, 40}

	pixels := encodeSamples(binary.LittleEndian, values, 4, func(o binary.ByteOrder, b []byte, v float64) {
		o.PutUint32(b, math.Float32bits(float32(v)))
	})

	model, err := LoadFromBytes(buildGeoTIFF(t, tiffSpec{
		width: 2, height: 2,
		bitsPerSample: 32,
		sampleFormat:  sampleFormatFloat,
		pixels:        pixels,
		originX:       0, originY: 5,
		pixelSizeX: 20, pixelSizeY: 5,
	}))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	// Half a pixel is 10 m in X and 2.5 m in Y.
	if got := model.Bounds(); got != [4]float64{-10, -2.5, 30, 7.5} {
		t.Fatalf("Bounds() = %v, want [-10 -2.5 30 7.5]", got)
	}

	// 10 m out in Y is far outside, while 10 m out in X is exactly on the edge.
	if _, ok := model.ElevationAt(-10, 5); !ok {
		t.Fatal("x = -10 is on the X edge and must be covered")
	}

	if _, ok := model.ElevationAt(0, -10); ok {
		t.Fatal("y = -10 is well past the Y edge and must be a miss")
	}
}

// Load and LoadFromBytes must agree byte for byte: the CLI uses one and the
// HTTP terrain import the other, and a run started either way has to produce
// the same elevations.
func TestLoadAndLoadFromBytesAgree(t *testing.T) {
	t.Parallel()

	values := []float64{1, 2, 3, 4, 5, 6}
	data := float32GeoTIFF(t, 3, 2, values, 100, 200, 10, "")

	fromBytes, err := LoadFromBytes(data)
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	path := writeTestGeoTIFF(t, 3, 2, []float32{1, 2, 3, 4, 5, 6}, 100, 200, 10, 10)

	fromFile, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if fromFile.Bounds() != fromBytes.Bounds() {
		t.Fatalf("Load bounds %v != LoadFromBytes bounds %v", fromFile.Bounds(), fromBytes.Bounds())
	}

	for row := range 2 {
		for col := range 3 {
			x := 100 + float64(col)*10
			y := 200 - float64(row)*10

			a, okA := fromFile.ElevationAt(x, y)

			b, okB := fromBytes.ElevationAt(x, y)
			if okA != okB || a != b {
				t.Fatalf("at (%g,%g): Load gave (%v,%t), LoadFromBytes gave (%v,%t)", x, y, a, okA, b, okB)
			}
		}
	}
}
