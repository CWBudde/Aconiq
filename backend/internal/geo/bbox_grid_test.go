package geo

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// randomBoxes lays out boxes of wildly different sizes so that the builder's
// wide list, its cell-size choice and its clamping all see traffic.
func randomBoxes(rng *rand.Rand, count int) []BBox {
	boxes := make([]BBox, 0, count)

	for range count {
		x := rng.Float64()*2000 - 500
		y := rng.Float64()*400 - 100
		w := math.Pow(10, rng.Float64()*3-1)
		h := math.Pow(10, rng.Float64()*3-1)

		boxes = append(boxes, BBox{MinX: x, MinY: y, MaxX: x + w, MaxY: y + h})
	}

	return boxes
}

// bruteForceQuery is the answer the index has to reproduce exactly.
func bruteForceQuery(boxes []BBox, q BBox) []int {
	var want []int

	for i, box := range boxes {
		if box.Intersects(q) {
			want = append(want, i)
		}
	}

	return want
}

func TestBBoxGridQueryMatchesBruteForce(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(1, 2))
	boxes := randomBoxes(rng, 400)

	grid := NewBBoxGrid(boxes)
	if grid == nil {
		t.Fatal("expected a grid")
	}

	cursor := grid.NewCursor()

	for range 500 {
		x := rng.Float64()*2400 - 700
		y := rng.Float64()*600 - 200
		q := BBox{MinX: x, MinY: y, MaxX: x + rng.Float64()*300, MaxY: y + rng.Float64()*120}

		got := cursor.Query(q, nil)
		if !slices.Equal(got, bruteForceQuery(boxes, q)) {
			t.Fatalf("query %v: got %v, want %v", q, got, bruteForceQuery(boxes, q))
		}

		if !slices.IsSorted(got) {
			t.Fatalf("query %v answered out of order: %v", q, got)
		}
	}
}

// TestBBoxGridQueryReusesBuffer guards the contract the propagation walk
// depends on: a query appends to the caller's slice and leaves what was
// already in it alone, so one buffer serves a whole run.
func TestBBoxGridQueryReusesBuffer(t *testing.T) {
	t.Parallel()

	boxes := []BBox{
		{MinX: 0, MinY: 0, MaxX: 1, MaxY: 1},
		{MinX: 10, MinY: 10, MaxX: 11, MaxY: 11},
	}

	cursor := NewBBoxGrid(boxes).NewCursor()

	buffer := cursor.Query(BBox{MinX: 0, MinY: 0, MaxX: 1, MaxY: 1}, nil)
	if !slices.Equal(buffer, []int{0}) {
		t.Fatalf("first query: got %v", buffer)
	}

	buffer = cursor.Query(BBox{MinX: 10, MinY: 10, MaxX: 11, MaxY: 11}, buffer[:0])
	if !slices.Equal(buffer, []int{1}) {
		t.Fatalf("second query: got %v", buffer)
	}
}

// TestBBoxGridQuerySegmentIsSupersetOfCrossings is the exactness guard for
// the segment walk: whatever the segment can actually touch must come back,
// even though the walk visits far fewer cells than the segment's box covers.
func TestBBoxGridQuerySegmentIsSupersetOfCrossings(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(3, 4))
	boxes := randomBoxes(rng, 400)

	cursor := NewBBoxGrid(boxes).NewCursor()

	for range 500 {
		p := Point2D{X: rng.Float64()*2400 - 700, Y: rng.Float64()*600 - 200}
		q := Point2D{X: rng.Float64()*2400 - 700, Y: rng.Float64()*600 - 200}

		got := cursor.QuerySegment(p, q, 1e-3, nil)
		if !slices.IsSorted(got) {
			t.Fatalf("segment query answered out of order: %v", got)
		}

		// Every box the segment genuinely enters has to be in the answer.
		for i, box := range boxes {
			if !segmentEntersBox(p, q, box) {
				continue
			}

			if !slices.Contains(got, i) {
				t.Fatalf("segment (%v→%v) enters box %d %v but the walk missed it", p, q, i, box)
			}
		}
	}
}

// segmentEntersBox reports whether any point of the segment lies in the box,
// by sampling densely enough that a box of the sizes used above cannot hide
// between two samples.
func segmentEntersBox(p, q Point2D, box BBox) bool {
	const samples = 4000

	for i := range samples + 1 {
		t := float64(i) / samples
		if box.ContainsPoint(Point2D{X: p.X + t*(q.X-p.X), Y: p.Y + t*(q.Y-p.Y)}) {
			return true
		}
	}

	return false
}

// TestBBoxGridQueryShadowIsSupersetOfShadowedBoxes is the exactness guard for
// the row-clipped shadow walk: it visits fewer cells than the shadow's box
// covers, so it has to be shown that nothing the shadow reaches is lost.
func TestBBoxGridQueryShadowIsSupersetOfShadowedBoxes(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(13, 17))
	boxes := randomBoxes(rng, 400)

	grid := NewBBoxGrid(boxes)
	cursor := grid.NewCursor()

	narrowed := 0

	for range 150 {
		apex := Point2D{X: rng.Float64()*2400 - 700, Y: rng.Float64()*600 - 200}
		a := Point2D{X: rng.Float64()*2400 - 700, Y: rng.Float64()*600 - 200}
		b := Point2D{X: rng.Float64()*2400 - 700, Y: rng.Float64()*600 - 200}

		shadow := NewSegmentShadow(apex, a, b, 1e-3)

		got := cursor.QueryShadow(&shadow, 1e-3, nil)
		if !slices.IsSorted(got) {
			t.Fatalf("shadow query answered out of order: %v", got)
		}

		// Any box with a point of the shadow in it has to come back.
		for i, box := range boxes {
			if !boxMeetsShadow(box, &shadow) || slices.Contains(got, i) {
				continue
			}

			t.Fatalf("box %d %v reaches into the shadow of (%v,%v) from %v but the walk missed it",
				i, box, a, b, apex)
		}

		// And it has to be worth doing: the walk must return less than a
		// query over the shadow's own box at least sometimes.
		full, ok := shadow.Bounds(grid.Bounds(), 1e-3)
		if ok && len(got) < len(cursor.Query(full, nil)) {
			narrowed++
		}
	}

	if narrowed == 0 {
		t.Fatal("the row-clipped walk never narrowed anything, so it is only cost")
	}
}

// boxMeetsShadow reports whether a sampled point of the box is in the shadow.
// It is one-sided on purpose: a box that only clips a corner of the shadow
// may go unsampled and unasserted, but nothing it does report is wrong.
func boxMeetsShadow(box BBox, shadow *SegmentShadow) bool {
	const steps = 8

	for iy := range steps + 1 {
		for ix := range steps + 1 {
			p := Point2D{
				X: box.MinX + box.Width()*float64(ix)/steps,
				Y: box.MinY + box.Height()*float64(iy)/steps,
			}
			if shadow.MayContainSegment(p, p) {
				return true
			}
		}
	}

	return false
}

func TestBBoxGridRefusesUnusableInput(t *testing.T) {
	t.Parallel()

	if NewBBoxGrid(nil) != nil {
		t.Fatal("an empty box set has nothing to index")
	}

	if NewBBoxGrid([]BBox{{MinX: math.NaN(), MinY: 0, MaxX: 1, MaxY: 1}}) != nil {
		t.Fatal("a non-finite box must fall back to no index")
	}

	if NewBBoxGrid([]BBox{{MinX: 5, MinY: 0, MaxX: 1, MaxY: 1}}) != nil {
		t.Fatal("an inverted box must fall back to no index")
	}

	var absent *BBoxGrid
	if absent.NewCursor().Ready() {
		t.Fatal("a cursor over no grid must report itself unusable")
	}
}
