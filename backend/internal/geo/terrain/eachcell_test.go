package terrain

import (
	"errors"
	"testing"
)

// EachCell answers what the raster stores, at the centres the tie point and
// pixel scale place, north-west first — and skips nodata without dropping the
// valid cells beside it, which ElevationAt's four-neighbour rule would.
func TestEachCellYieldsStoredValuesAtCellCentres(t *testing.T) {
	t.Parallel()

	values := []float64{
		1, 2, 3,
		4, -9999, 6,
	}

	model, err := LoadFromBytes(float32GeoTIFF(t, 3, 2, values, 100, 220, 10, "-9999"))
	if err != nil {
		t.Fatal(err)
	}

	type cell struct{ x, y, z float64 }

	var got []cell

	err = EachCell(model, func(x, y, z float64) error {
		got = append(got, cell{x, y, z})

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []cell{{100, 220, 1}, {110, 220, 2}, {120, 220, 3}, {100, 210, 4}, {120, 210, 6}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cell %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestEachCellRefusesAModelWithNoCells(t *testing.T) {
	t.Parallel()

	wrapped, err := InComputeCRS(hamburgPatch(), "EPSG:4326", "EPSG:25832")
	if err != nil {
		t.Fatal(err)
	}

	for _, model := range []Model{wrapped, hamburgPatch()} {
		if EachCell(model, func(_, _, _ float64) error { return nil }) == nil {
			t.Errorf("%T was enumerated as if it were a grid", model)
		}
	}
}

func TestEachCellStopsAtTheFirstError(t *testing.T) {
	t.Parallel()

	model, err := LoadFromBytes(float32GeoTIFF(t, 2, 2, []float64{1, 2, 3, 4}, 0, 10, 10, ""))
	if err != nil {
		t.Fatal(err)
	}

	stop := errors.New("stop")
	calls := 0

	err = EachCell(model, func(_, _, _ float64) error {
		calls++

		return stop
	})
	if !errors.Is(err, stop) || calls != 1 {
		t.Fatalf("err %v after %d calls", err, calls)
	}
}
