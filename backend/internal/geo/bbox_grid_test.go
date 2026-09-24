package geo

import (
	"fmt"
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

// TestBBoxGridQueryMatchesBruteForce sweeps box counts wide enough to exercise
// the cell walk, the wide list, the scanAll shortcut and a bitset word
// boundary (64/65). Determinism policy leaves no output tolerance, so "the
// same set" is not enough: the index must answer element for element, in
// the same order.
func TestBBoxGridQueryMatchesBruteForce(t *testing.T) {
	t.Parallel()

	for _, count := range []int{1, 7, 64, 65, 400, 2000} {
		t.Run(fmt.Sprintf("boxes=%d", count), func(t *testing.T) {
			t.Parallel()

			rng := rand.New(rand.NewPCG(uint64(count), 99))
			boxes := randomBoxes(rng, count)

			grid := NewBBoxGrid(boxes)
			if grid == nil {
				t.Fatal("expected a grid")
			}

			cursor := grid.NewCursor()

			for range 500 {
				x := rng.Float64()*2400 - 700
				y := rng.Float64()*600 - 200
				q := BBox{MinX: x, MinY: y, MaxX: x + rng.Float64()*600, MaxY: y + rng.Float64()*240}

				if got, want := cursor.Query(q, nil), bruteForceQuery(boxes, q); !slices.Equal(got, want) {
					t.Fatalf("query %v: got %v, want %v", q, got, want)
				}
			}
		})
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

// strictlyAscending reports whether the answer is sorted *and* duplicate-free.
// slices.IsSorted alone would pass an answer that returned an item twice, and
// a deduplicating collector is exactly what these tests are guarding.
func strictlyAscending(got []int) bool {
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			return false
		}
	}

	return true
}

// TestBBoxGridQueryKeepsCallerPrefix repeats the brute-force comparison with
// a buffer that already holds a prefix and is reused between calls, because
// that is how the propagation walk calls these and because the emit pass
// appends after the walk rather than during it.
func TestBBoxGridQueryKeepsCallerPrefix(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(21, 22))
	boxes := randomBoxes(rng, 300)

	cursor := NewBBoxGrid(boxes).NewCursor()

	prefix := []int{-3, -2, -1}
	buffer := slices.Clone(prefix)

	for range 400 {
		x := rng.Float64()*2400 - 700
		y := rng.Float64()*600 - 200
		q := BBox{MinX: x, MinY: y, MaxX: x + rng.Float64()*300, MaxY: y + rng.Float64()*120}

		buffer = cursor.Query(q, buffer[:len(prefix)])

		want := append(slices.Clone(prefix), bruteForceQuery(boxes, q)...)
		if !slices.Equal(buffer, want) {
			t.Fatalf("query %v over a reused buffer: got %v, want %v", q, buffer, want)
		}
	}
}

// TestBBoxGridQuerySegmentIsSubsetOfItsBox is the other half of the segment
// walk's contract. The superset test above shows nothing is lost; this shows
// nothing is invented: every answer intersects the segment's own margin box,
// and no item comes back twice.
func TestBBoxGridQuerySegmentIsSubsetOfItsBox(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(5, 6))
	boxes := randomBoxes(rng, 400)

	cursor := NewBBoxGrid(boxes).NewCursor()

	const marginM = 1e-3

	for range 500 {
		p := Point2D{X: rng.Float64()*2400 - 700, Y: rng.Float64()*600 - 200}
		q := Point2D{X: rng.Float64()*2400 - 700, Y: rng.Float64()*600 - 200}

		got := cursor.QuerySegment(p, q, marginM, nil)
		if !strictlyAscending(got) {
			t.Fatalf("segment query answered out of order or with a duplicate: %v", got)
		}

		box := BBox{
			MinX: math.Min(p.X, q.X) - marginM,
			MinY: math.Min(p.Y, q.Y) - marginM,
			MaxX: math.Max(p.X, q.X) + marginM,
			MaxY: math.Max(p.Y, q.Y) + marginM,
		}

		for _, i := range got {
			if !boxes[i].Intersects(box) {
				t.Fatalf("segment (%v→%v) answered box %d %v, which its margin box does not reach", p, q, i, boxes[i])
			}
		}
	}
}

// TestBBoxGridQueryShadowIsSubsetOfItsBox is the same other half for the
// row-clipped shadow walk.
func TestBBoxGridQueryShadowIsSubsetOfItsBox(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(7, 8))
	boxes := randomBoxes(rng, 400)

	grid := NewBBoxGrid(boxes)
	cursor := grid.NewCursor()

	for range 300 {
		apex := Point2D{X: rng.Float64()*2400 - 700, Y: rng.Float64()*600 - 200}
		a := Point2D{X: rng.Float64()*2400 - 700, Y: rng.Float64()*600 - 200}
		b := Point2D{X: rng.Float64()*2400 - 700, Y: rng.Float64()*600 - 200}

		shadow := NewSegmentShadow(apex, a, b, 1e-3)

		got := cursor.QueryShadow(&shadow, 1e-3, nil)
		if !strictlyAscending(got) {
			t.Fatalf("shadow query answered out of order or with a duplicate: %v", got)
		}

		full, ok := shadow.Bounds(grid.Bounds(), 1e-3)
		if !ok {
			if len(got) != 0 {
				t.Fatalf("an unreachable shadow answered %v", got)
			}

			continue
		}

		for _, i := range got {
			if !boxes[i].Intersects(full) {
				t.Fatalf("shadow answered box %d %v, which its clipped box %v does not reach", i, boxes[i], full)
			}
		}
	}
}

// FuzzBBoxGridQueryMatchesBruteForce drives the index from fuzzed boxes and
// queries rather than a seeded corpus, so that a degenerate layout the
// generator above never produces — every box a point, one box spanning the
// scene, a query exactly on a cell boundary — still has to answer exactly.
// The segment and shadow walks have no exact oracle, so they are held to
// their ordering contract here and to their bounds by the tests above.
func FuzzBBoxGridQueryMatchesBruteForce(f *testing.F) {
	f.Add(uint64(1), uint64(2), 17)
	f.Add(uint64(999), uint64(7), 1)
	f.Add(uint64(42), uint64(42), 130)

	f.Fuzz(func(t *testing.T, seedA, seedB uint64, rawCount int) {
		count := max(rawCount%257, 1)

		rng := rand.New(rand.NewPCG(seedA, seedB))

		boxes := make([]BBox, 0, count)
		for range count {
			x := rng.Float64()*200 - 100
			y := rng.Float64()*200 - 100
			w := math.Pow(10, rng.Float64()*4-2)
			h := math.Pow(10, rng.Float64()*4-2)

			boxes = append(boxes, BBox{MinX: x, MinY: y, MaxX: x + w, MaxY: y + h})
		}

		grid := NewBBoxGrid(boxes)
		if grid == nil {
			t.Skip("no index over this box set")
		}

		cursor := grid.NewCursor()

		for range 20 {
			x := rng.Float64()*300 - 150
			y := rng.Float64()*300 - 150
			q := BBox{MinX: x, MinY: y, MaxX: x + rng.Float64()*100, MaxY: y + rng.Float64()*100}

			if got, want := cursor.Query(q, nil), bruteForceQuery(boxes, q); !slices.Equal(got, want) {
				t.Fatalf("Query %v: got %v, brute force %v", q, got, want)
			}

			p1 := Point2D{X: rng.Float64()*300 - 150, Y: rng.Float64()*300 - 150}
			p2 := Point2D{X: rng.Float64()*300 - 150, Y: rng.Float64()*300 - 150}

			if got := cursor.QuerySegment(p1, p2, 1e-3, nil); !strictlyAscending(got) {
				t.Fatalf("QuerySegment (%v→%v) answered out of order or with a duplicate: %v", p1, p2, got)
			}

			shadow := NewSegmentShadow(
				Point2D{X: rng.Float64()*300 - 150, Y: rng.Float64()*300 - 150}, p1, p2, 1e-3,
			)

			if got := cursor.QueryShadow(&shadow, 1e-3, nil); !strictlyAscending(got) {
				t.Fatalf("QueryShadow answered out of order or with a duplicate: %v", got)
			}
		}
	})
}
