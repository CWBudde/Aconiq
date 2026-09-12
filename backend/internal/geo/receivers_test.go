package geo

import (
	"math"
	"testing"
)

func TestGridReceiverGenerate(t *testing.T) {
	t.Parallel()

	grid := GridReceiverSet{
		ID:         "grid",
		Extent:     BBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
		Resolution: 5,
		HeightM:    4,
	}

	receivers, err := grid.Generate()
	if err != nil {
		t.Fatalf("generate grid receivers: %v", err)
	}

	if len(receivers) != 9 {
		t.Fatalf("expected 9 receivers, got %d", len(receivers))
	}
}

func TestPointReceiverSetValidateDuplicateID(t *testing.T) {
	t.Parallel()

	set := PointReceiverSet{
		ID: "points",
		Receivers: []PointReceiver{
			{ID: "r1", Point: Point2D{X: 0, Y: 0}, HeightM: 4},
			{ID: "r1", Point: Point2D{X: 1, Y: 1}, HeightM: 4},
		},
	}

	err := set.Validate()
	if err == nil {
		t.Fatal("expected duplicate id validation error")
	}
}

func TestPointReceiverSetValidateErrors(t *testing.T) {
	t.Parallel()

	tests := []PointReceiverSet{
		{},
		{
			ID: "points",
			Receivers: []PointReceiver{
				{Point: Point2D{X: 0, Y: 0}, HeightM: 4},
			},
		},
		{
			ID: "points",
			Receivers: []PointReceiver{
				{ID: "r1", Point: Point2D{X: math.NaN(), Y: 0}, HeightM: 4},
			},
		},
		{
			ID: "points",
			Receivers: []PointReceiver{
				{ID: "r1", Point: Point2D{X: 0, Y: 0}, HeightM: math.Inf(1)},
			},
		},
	}

	for _, tc := range tests {
		err := tc.Validate()
		if err == nil {
			t.Fatalf("expected validation error for %#v", tc)
		}
	}
}

func TestGridReceiverSetValidateErrors(t *testing.T) {
	t.Parallel()

	tests := []GridReceiverSet{
		{},
		{
			ID:         "grid",
			Extent:     BBox{MinX: 1, MinY: 0, MaxX: 0, MaxY: 1},
			Resolution: 1,
			HeightM:    4,
		},
		{
			ID:         "grid",
			Extent:     BBox{MinX: 0, MinY: 0, MaxX: 1, MaxY: 1},
			Resolution: 0,
			HeightM:    4,
		},
		{
			ID:         "grid",
			Extent:     BBox{MinX: 0, MinY: 0, MaxX: 1, MaxY: 1},
			Resolution: 1,
			HeightM:    math.NaN(),
		},
	}

	for _, tc := range tests {
		err := tc.Validate()
		if err == nil {
			t.Fatalf("expected validation error for %#v", tc)
		}
	}
}

func TestFacadeReceiverSetValidateErrors(t *testing.T) {
	t.Parallel()

	tests := []FacadeReceiverSet{
		{},
		{ID: "facade"},
		{ID: "facade", BuildingIDs: []string{"b1"}, OffsetM: math.Inf(1), VerticalStepM: 3},
		{ID: "facade", BuildingIDs: []string{"b1"}, OffsetM: 1, VerticalStepM: 0},
	}

	for _, tc := range tests {
		err := tc.Validate()
		if err == nil {
			t.Fatalf("expected validation error for %#v", tc)
		}
	}
}

func TestFacadeReceiverSetStub(t *testing.T) {
	t.Parallel()

	facade := FacadeReceiverSet{
		ID:            "facade-a",
		BuildingIDs:   []string{"b1"},
		OffsetM:       1.0,
		VerticalStepM: 3.0,
	}

	if facade.IsImplemented() {
		t.Fatal("facade generation should be deferred in phase 5")
	}

	err := facade.Validate()
	if err != nil {
		t.Fatalf("expected valid facade stub, got %v", err)
	}
}

// CellCount exists so an oversized grid can be refused before Generate
// allocates it, which is only sound if the two agree about how many receivers
// the grid holds.
func TestGridCellCountMatchesGenerate(t *testing.T) {
	t.Parallel()

	cases := map[string]GridReceiverSet{
		"square on the origin":  {ID: "g", Extent: BBox{MaxX: 10, MaxY: 10}, Resolution: 5, HeightM: 4},
		"single cell":           {ID: "g", Extent: BBox{MaxX: 0, MaxY: 0}, Resolution: 5, HeightM: 4},
		"partial last step":     {ID: "g", Extent: BBox{MaxX: 11, MaxY: 7}, Resolution: 5, HeightM: 4},
		"projected coordinates": {ID: "g", Extent: BBox{MinX: 601000, MinY: 5701000, MaxX: 601200, MaxY: 5701200}, Resolution: 50, HeightM: 4},
		"negative origin":       {ID: "g", Extent: BBox{MinX: -30, MinY: -20, MaxX: 30, MaxY: 20}, Resolution: 10, HeightM: 4},
		"fractional resolution": {ID: "g", Extent: BBox{MaxX: 5, MaxY: 5}, Resolution: 2.5, HeightM: 4},
	}

	for name, grid := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			count, err := grid.CellCount()
			if err != nil {
				t.Fatalf("count grid cells: %v", err)
			}

			receivers, err := grid.Generate()
			if err != nil {
				t.Fatalf("generate grid receivers: %v", err)
			}

			if count != float64(len(receivers)) {
				t.Fatalf("CellCount says %.0f, Generate produced %d", count, len(receivers))
			}
		})
	}
}

// The whole point of counting arithmetically: this grid describes more
// receivers than the machine could hold, and sizing it costs two divisions.
func TestGridCellCountSizesAGridTooLargeToBuild(t *testing.T) {
	t.Parallel()

	grid := GridReceiverSet{
		ID:         "grid",
		Extent:     BBox{MaxX: 100_000, MaxY: 100_000},
		Resolution: 0.01,
		HeightM:    4,
	}

	count, err := grid.CellCount()
	if err != nil {
		t.Fatalf("count grid cells: %v", err)
	}

	// 10^7 + 1 samples on each axis.
	if want := float64(10_000_001) * float64(10_000_001); count != want {
		t.Fatalf("expected %.0f cells, got %.0f", want, count)
	}
}

func TestGridCellCountRejectsAnInvalidGrid(t *testing.T) {
	t.Parallel()

	grid := GridReceiverSet{ID: "grid", Extent: BBox{MaxX: 10, MaxY: 10}, Resolution: 0, HeightM: 4}

	_, err := grid.CellCount()
	if err == nil {
		t.Fatal("expected a zero resolution to be refused")
	}
}
