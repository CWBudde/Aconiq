package geo

import "testing"

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
