package geo

import (
	"math"
	"math/bits"
	"slices"
)

// This file holds the candidate index a propagation walk queries once per
// (source, receiver) pair. It is deliberately not GridSpatialIndex.
//
// GridSpatialIndex keys an item by a string ID, which is what a model-level
// index wants. A propagation walk has no ID to offer: its obstacles arrive as
// a slice, and an obstacle's slice position is both the identity the caller
// needs back and the tie-break order RayCrossings sorts on. Keying that by
// string would mean inventing an ID per obstacle, a map lookup per candidate
// and a freshly allocated map per query — on a path a grid run walks millions
// of times. Worse, GridSpatialIndex refuses a duplicate ID, and barriers
// synthesised from building footprints carry the building's ID, which nothing
// guarantees is unique.

const (
	// gridCellsPerItem is how many cells the builder aims to have per indexed
	// box. Much below one and the cell lists stop thinning; much above four
	// and walking the cells of a query costs more than it saves.
	gridCellsPerItem = 2

	// gridMaxCellsPerSide caps the grid so that one far-flung obstacle cannot
	// turn the index into an array larger than the model it indexes.
	gridMaxCellsPerSide = 512

	// gridWideCellLimit is the cell count above which a box is held in the
	// wide list and tested on every query, rather than listed in each of its
	// cells. It keeps the build linear in the item count when a single
	// obstacle spans the scene.
	gridWideCellLimit = 64
)

// BBoxGrid is a read-only uniform-cell index over a fixed set of bounding
// boxes: built once, queried many times, never modified afterwards.
//
// A query allocates nothing but the caller's own result slice, and answers in
// ascending item order. That ordering is not a convenience — it is the
// contract. Every consumer here feeds a stable sort whose ties fall back on
// slice order, and a cell-walk order, though perfectly deterministic, is not
// slice order.
//
// The grid is immutable after construction and safe for concurrent queries,
// provided each goroutine holds its own cursor.
type BBoxGrid struct {
	boxes  []BBox
	bounds BBox

	cellSize float64
	nx, ny   int

	// starts and items hold the cell lists in compressed-row form: cell c owns
	// items[starts[c]:starts[c+1]], ascending.
	starts []int32
	items  []int32

	// wide holds the items whose box covers more cells than listing it in each
	// of them is worth.
	wide []int32
}

// NewBBoxGrid builds an index over a copy of boxes; a query answers in indices
// into that same slice.
//
// It returns nil when there is nothing to index or when any box is malformed.
// A nil grid is not an error — it is the caller's signal to test every item,
// which is what it did before the index existed, so a degenerate model loses
// the speed-up and keeps the answer.
func NewBBoxGrid(boxes []BBox) *BBoxGrid {
	bounds, ok := gridBounds(boxes)
	if !ok {
		return nil
	}

	cellSize := gridCellSize(boxes, bounds)

	grid := &BBoxGrid{
		boxes:    slices.Clone(boxes),
		bounds:   bounds,
		cellSize: cellSize,
		nx:       gridSideCount(bounds.Width(), cellSize),
		ny:       gridSideCount(bounds.Height(), cellSize),
		starts:   nil,
		items:    nil,
		wide:     nil,
	}

	grid.fill()

	return grid
}

// Bounds returns the box enclosing every indexed box. A caller narrowing an
// unbounded region — a cone, a half-plane — clips it against this rather than
// against the model extent: nothing outside it can ever be a candidate.
func (g *BBoxGrid) Bounds() BBox {
	if g == nil {
		return BBox{}
	}

	return g.bounds
}

// Len reports how many boxes are indexed.
func (g *BBoxGrid) Len() int {
	if g == nil {
		return 0
	}

	return len(g.boxes)
}

// gridBounds returns the box enclosing every input box, and false when any of
// them — or the union — is unusable.
func gridBounds(boxes []BBox) (BBox, bool) {
	if len(boxes) == 0 {
		return BBox{}, false
	}

	bounds := boxes[0]

	for _, box := range boxes {
		if !box.IsFinite() || !box.IsValid() {
			return BBox{}, false
		}

		bounds = bounds.ExpandToIncludeBBox(box)
	}

	if !bounds.IsFinite() || !bounds.IsValid() {
		return BBox{}, false
	}

	return bounds, true
}

// gridCellSize picks a cell edge that is at least the average box extent — a
// box narrower than a cell lands in one or two of them — and no finer than
// the item count justifies.
func gridCellSize(boxes []BBox, bounds BBox) float64 {
	var extent float64

	for _, box := range boxes {
		extent += math.Max(box.Width(), box.Height())
	}

	average := extent / float64(len(boxes))
	target := math.Sqrt(bounds.Width() * bounds.Height() / float64(gridCellsPerItem*len(boxes)))

	cellSize := math.Max(average, target)
	if math.IsNaN(cellSize) || math.IsInf(cellSize, 0) || cellSize <= 0 {
		// Every box is a point, or the bounds are flat in one axis. One metre
		// is as good a cell as any here, and the side caps keep it sane.
		cellSize = 1
	}

	for gridSideCount(bounds.Width(), cellSize) > gridMaxCellsPerSide ||
		gridSideCount(bounds.Height(), cellSize) > gridMaxCellsPerSide {
		cellSize *= 2
	}

	return cellSize
}

// gridSideCount returns how many cells of cellSize an extent spans. It reports
// one past the cap rather than the true count when that count is large, which
// is all gridCellSize's doubling loop needs and keeps the conversion to int
// away from values int cannot hold.
func gridSideCount(extent, cellSize float64) int {
	if extent <= 0 || cellSize <= 0 {
		return 1
	}

	count := extent/cellSize + 1
	if count > float64(gridMaxCellsPerSide) {
		return gridMaxCellsPerSide + 1
	}

	return int(count)
}

// gridAxisIndex maps an offset from the grid origin to a cell index, clamped
// into the grid. Clamping is what lets a query box reach outside the indexed
// extent without the caller checking first.
func gridAxisIndex(offset, cellSize float64, count int) int {
	if math.IsNaN(offset) || offset <= 0 {
		return 0
	}

	cell := offset / cellSize
	if cell >= float64(count-1) {
		return count - 1
	}

	return int(cell)
}

// bboxGridUseBitset selects the candidate collector a cursor is built with.
//
// The bitset collector marks one bit per item and emits the marks in index
// order; the legacy collector it replaced stamped a uint32 generation per item,
// concatenated the cells' hits in walk order and sorted the tail. Both are
// still here only so TestBBoxGridCollectorsAgree can hold one against the other
// over a randomised corpus. Once that has been read and believed, delete this
// const, BBoxGridCursor.legacy, the seen/generation fields, nextGeneration and
// the legacy branches in collect and finish — all in one commit.
const bboxGridUseBitset = true

// BBoxGridCursor is one goroutine's view of a grid. It carries the scratch a
// query needs — the per-item marks — so that the query itself allocates
// nothing and the grid stays shareable.
type BBoxGridCursor struct {
	grid *BBoxGrid

	// marks holds two bits per indexed item: a tested bit, set once the walk
	// has run Intersects on the item in this query, and an answered bit, set
	// when that test said yes. Two bits rather than one because the questions
	// differ — an item tested and rejected must not be tested again, and must
	// not be emitted either.
	//
	// The two bitsets are interleaved a word at a time rather than kept in two
	// arrays: marks[2w] is the tested word for items 64w..64w+63 and
	// marks[2w|1] the answered word for the same items. The emit pass reads
	// both for every word a query touched, at an address the item indices —
	// which follow the caller's slice, not space — scatter across the array,
	// so keeping the pair sixteen bytes apart turns what were two cache misses
	// per touched word into one.
	marks []uint64

	// touched is the second level: bit w of it says word w of marks holds a
	// mark from the current query. It is what keeps the emit pass
	// proportional to the answer rather than to the index.
	//
	// A bound on the lowest and highest item alone would not do that. A query
	// is local in space, but an item's index is its position in the caller's
	// slice, and nothing makes that follow space: over a hundred thousand
	// scattered boxes even a one-candidate query lands marks near both ends,
	// so a flat scan between them reads most of the bitset. Measured, that
	// sank BenchmarkBBoxGridQuerySegment/boxes=100000/candidates=1 to six
	// times slower than the sort it replaced. The summary shrinks the span
	// that has to be scanned by another factor of 64 — 25 words for a hundred
	// thousand items, worst case, which is two cache lines.
	touched []uint64

	// loTouched and hiTouched bound the summary words this query has written,
	// so the scan starts and stops at the marks instead of at the array.
	loTouched, hiTouched int

	// legacy selects the pre-bitset collector; see bboxGridUseBitset. Under it
	// seen[i] == generation means item i has been tested in the current query.
	legacy     bool
	seen       []uint32
	generation uint32
}

// NewCursor returns a cursor over this grid, or nil for a nil grid so that a
// caller can hold the result unconditionally.
func (g *BBoxGrid) NewCursor() *BBoxGridCursor {
	return g.newCursor(!bboxGridUseBitset)
}

// newCursor builds a cursor over either collector. Tests reach for it to run
// the two against each other; production goes through NewCursor.
func (g *BBoxGrid) newCursor(legacy bool) *BBoxGridCursor {
	if g == nil {
		return nil
	}

	cursor := &BBoxGridCursor{
		grid:      g,
		marks:     nil,
		touched:   nil,
		loTouched: 0,
		hiTouched: -1,
		legacy:    legacy,
		seen:      nil,

		generation: 0,
	}

	if legacy {
		cursor.seen = make([]uint32, len(g.boxes))

		return cursor
	}

	words := (len(g.boxes) + 63) / 64
	summary := (words + 63) / 64

	cursor.marks = make([]uint64, 2*words)
	cursor.touched = make([]uint64, summary)
	cursor.loTouched = summary

	return cursor
}

// fill lays out the cell lists. It counts first and places second so that the
// items array is exactly the size it needs and each cell's list comes out in
// ascending item order.
func (g *BBoxGrid) fill() {
	g.starts = make([]int32, g.nx*g.ny+1)

	for index := range g.boxes {
		x0, x1, y0, y1, wide := g.cellRange(g.boxes[index])
		if wide {
			g.wide = append(g.wide, int32(index))

			continue
		}

		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				g.starts[y*g.nx+x+1]++
			}
		}
	}

	for i := 1; i < len(g.starts); i++ {
		g.starts[i] += g.starts[i-1]
	}

	g.items = make([]int32, g.starts[len(g.starts)-1])
	next := slices.Clone(g.starts[:len(g.starts)-1])

	for index := range g.boxes {
		x0, x1, y0, y1, wide := g.cellRange(g.boxes[index])
		if wide {
			continue
		}

		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				cell := y*g.nx + x
				g.items[next[cell]] = int32(index)
				next[cell]++
			}
		}
	}
}

// cellRange returns the inclusive cell range a box covers, and whether that
// range is wide enough that the box belongs in the wide list instead.
func (g *BBoxGrid) cellRange(box BBox) (int, int, int, int, bool) {
	x0 := gridAxisIndex(box.MinX-g.bounds.MinX, g.cellSize, g.nx)
	x1 := gridAxisIndex(box.MaxX-g.bounds.MinX, g.cellSize, g.nx)
	y0 := gridAxisIndex(box.MinY-g.bounds.MinY, g.cellSize, g.ny)
	y1 := gridAxisIndex(box.MaxY-g.bounds.MinY, g.cellSize, g.ny)

	return x0, x1, y0, y1, int64(x1-x0+1)*int64(y1-y0+1) > gridWideCellLimit
}

// Query appends the index of every indexed box intersecting q to dst, in
// ascending order, and returns the extended slice. Pass dst[:0] to reuse a
// buffer across calls.
//
// A nil cursor answers with dst unchanged, which is not "no candidates" — it
// is "there is no index", and the caller must fall back to testing everything.
// Ask Ready first.
func (c *BBoxGridCursor) Query(q BBox, dst []int) []int {
	if !c.Ready() || !q.IsFinite() || !q.IsValid() || !c.grid.bounds.Intersects(q) {
		return dst
	}

	grid := c.grid

	x0 := gridAxisIndex(q.MinX-grid.bounds.MinX, grid.cellSize, grid.nx)
	x1 := gridAxisIndex(q.MaxX-grid.bounds.MinX, grid.cellSize, grid.nx)
	y0 := gridAxisIndex(q.MinY-grid.bounds.MinY, grid.cellSize, grid.ny)
	y1 := gridAxisIndex(q.MaxY-grid.bounds.MinY, grid.cellSize, grid.ny)

	// A query reaching across most of the grid is cheaper answered by walking
	// the items than by walking the cells: the cell lists then hold every item
	// several times over, so the walk pays for a deduplication and a sort it
	// can avoid. Walking items answers ascending by construction.
	if 2*(x1-x0+1)*(y1-y0+1) >= len(grid.boxes) {
		return grid.scanAll(q, dst)
	}

	c.beginQuery()

	start := len(dst)

	for y := y0; y <= y1; y++ {
		row := y * grid.nx
		for x := x0; x <= x1; x++ {
			cell := row + x
			dst = c.collect(grid.items[grid.starts[cell]:grid.starts[cell+1]], q, dst)
		}
	}

	dst = c.collect(grid.wide, q, dst)

	return c.finish(start, dst)
}

// shadowRowLimit is how many grid rows a shadow query will clip one at a
// time. Each row costs a polygon clip, so past this many the per-row walk is
// dearer than the bounding-box walk it replaces.
const shadowRowLimit = 32

// QueryShadow appends, in ascending order, the index of every indexed box
// that intersects the shadow's clipped bounding box, walking the grid a row
// at a time so that the walk follows the shadow rather than the rectangle
// around it.
//
// A shadow cast along a grid is a thin wedge, and the box around a thin
// diagonal wedge is mostly empty; clipping per row is what keeps the answer
// close to the wedge. The result is a subset of what a box query returns and
// still a superset of what the shadow can reach, because a point in the
// shadow lies in a row this walk visits and within that row's clipped span.
func (c *BBoxGridCursor) QueryShadow(shadow *SegmentShadow, marginM float64, dst []int) []int {
	if !c.Ready() {
		return dst
	}

	grid := c.grid

	full, reachable := shadow.Bounds(grid.bounds, marginM)
	if !reachable {
		return dst
	}

	if !grid.bounds.Intersects(full) {
		return dst
	}

	y0 := gridAxisIndex(full.MinY-grid.bounds.MinY, grid.cellSize, grid.ny)
	y1 := gridAxisIndex(full.MaxY-grid.bounds.MinY, grid.cellSize, grid.ny)

	if y1-y0 >= shadowRowLimit {
		return c.Query(full, dst)
	}

	c.beginQuery()

	start := len(dst)

	for y := y0; y <= y1; y++ {
		low := grid.bounds.MinY + float64(y)*grid.cellSize

		span, crosses := shadow.Bounds(BBox{
			MinX: full.MinX, MinY: low,
			MaxX: full.MaxX, MaxY: low + grid.cellSize,
		}, marginM)
		if !crosses {
			continue
		}

		row := y * grid.nx
		x0 := gridAxisIndex(span.MinX-grid.bounds.MinX, grid.cellSize, grid.nx)
		x1 := gridAxisIndex(span.MaxX-grid.bounds.MinX, grid.cellSize, grid.nx)

		for x := x0; x <= x1; x++ {
			cell := row + x
			dst = c.collect(grid.items[grid.starts[cell]:grid.starts[cell+1]], full, dst)
		}
	}

	dst = c.collect(grid.wide, full, dst)

	return c.finish(start, dst)
}

// QuerySegment appends, in ascending order, the index of every indexed box
// that intersects the bounding box of segment [p,q] grown by marginM — but
// walks only the cells the segment itself passes through, not the cells its
// bounding box covers.
//
// That distinction is the whole point. A ray from a distant image source to a
// receiver has a bounding box the size of the scene and touches almost none
// of it, so Query would hand back most of the model. The answer here is a
// subset of Query's, and still a superset of the boxes the segment can meet:
// a point on both the segment and a box lies in a cell this walk visits, and
// that box is listed in that cell.
//
// marginM widens the walk, so a point a rounding error off the exact segment
// still falls inside it.
func (c *BBoxGridCursor) QuerySegment(p, q Point2D, marginM float64, dst []int) []int {
	if !c.Ready() || !p.IsFinite() || !q.IsFinite() {
		return dst
	}

	grid := c.grid

	bounds := BBox{
		MinX: math.Min(p.X, q.X) - marginM,
		MinY: math.Min(p.Y, q.Y) - marginM,
		MaxX: math.Max(p.X, q.X) + marginM,
		MaxY: math.Max(p.Y, q.Y) + marginM,
	}
	if !grid.bounds.Intersects(bounds) {
		return dst
	}

	c.beginQuery()

	start := len(dst)

	y0 := gridAxisIndex(bounds.MinY-grid.bounds.MinY, grid.cellSize, grid.ny)
	y1 := gridAxisIndex(bounds.MaxY-grid.bounds.MinY, grid.cellSize, grid.ny)

	for y := y0; y <= y1; y++ {
		low := grid.bounds.MinY + float64(y)*grid.cellSize - marginM
		high := low + grid.cellSize + 2*marginM

		xLow, xHigh, crosses := segmentSpanInBand(p, q, low, high)
		if !crosses {
			continue
		}

		row := y * grid.nx
		x0 := gridAxisIndex(xLow-marginM-grid.bounds.MinX, grid.cellSize, grid.nx)
		x1 := gridAxisIndex(xHigh+marginM-grid.bounds.MinX, grid.cellSize, grid.nx)

		for x := x0; x <= x1; x++ {
			cell := row + x
			dst = c.collect(grid.items[grid.starts[cell]:grid.starts[cell+1]], bounds, dst)
		}
	}

	dst = c.collect(grid.wide, bounds, dst)

	return c.finish(start, dst)
}

// segmentSpanInBand returns the x extent of the part of segment [p,q] whose y
// lies in [low, high], and false when no part of it does.
func segmentSpanInBand(p, q Point2D, low, high float64) (float64, float64, bool) {
	first, last := 0.0, 1.0

	dy := q.Y - p.Y
	if dy == 0 {
		if p.Y < low || p.Y > high {
			return 0, 0, false
		}
	} else {
		enter, leave := (low-p.Y)/dy, (high-p.Y)/dy
		if enter > leave {
			enter, leave = leave, enter
		}

		first = math.Max(first, enter)
		last = math.Min(last, leave)

		if first > last {
			return 0, 0, false
		}
	}

	dx := q.X - p.X

	return math.Min(p.X+first*dx, p.X+last*dx), math.Max(p.X+first*dx, p.X+last*dx), true
}

// scanAll answers a query by testing every indexed box in order.
func (g *BBoxGrid) scanAll(q BBox, dst []int) []int {
	for i := range g.boxes {
		if g.boxes[i].Intersects(q) {
			dst = append(dst, i)
		}
	}

	return dst
}

// Ready reports whether the cursor can answer a query. A false here means the
// caller must test every item rather than none.
func (c *BBoxGridCursor) Ready() bool {
	return c != nil && c.grid != nil
}

// beginQuery resets the per-query scratch. Every early return in a query — an
// unusable cursor, a degenerate or disjoint query box, an unreachable shadow,
// the scanAll shortcut, QueryShadow's delegation past shadowRowLimit — must
// come before this call, exactly as it came before nextGeneration: a walk that
// marks nothing must also leave nothing behind.
func (c *BBoxGridCursor) beginQuery() {
	if c.legacy {
		c.nextGeneration()

		return
	}

	// An empty [loTouched, hiTouched] range. finish clears every word it
	// visits, so marks and touched are already zero on entry and only the
	// bounds need resetting.
	c.loTouched = len(c.touched)
	c.hiTouched = -1
}

// collect takes one cell list — or the wide list — through the Intersects
// test, skipping the items this query has already tested.
//
// Under the bitset collector nothing is appended and dst comes back unchanged;
// the answer is emitted by finish. Returning dst anyway is what lets the three
// query walks stay one body across both collectors.
func (c *BBoxGridCursor) collect(list []int32, q BBox, dst []int) []int {
	if c.legacy {
		for _, item := range list {
			if c.seen[item] == c.generation {
				continue
			}

			c.seen[item] = c.generation

			if c.grid.boxes[item].Intersects(q) {
				dst = append(dst, int(item))
			}
		}

		return dst
	}

	for _, item := range list {
		word := int(item) >> 6
		slot := word << 1 // The tested word; slot|1 is the answered word.
		// item & 63 is an int32 in [0,63]: a shift count that is provably
		// non-negative and needs no conversion to say so.
		bit := uint64(1) << (item & 63)

		if c.marks[slot]&bit != 0 {
			continue
		}

		if c.marks[slot] == 0 {
			// The first mark in this word, so the summary does not know about
			// it yet. A cleared word is a reliable signal here because finish
			// zeroes every word it visits.
			summary := word >> 6
			c.touched[summary] |= uint64(1) << (word & 63)

			if summary < c.loTouched {
				c.loTouched = summary
			}

			if summary > c.hiTouched {
				c.hiTouched = summary
			}
		}

		c.marks[slot] |= bit

		if c.grid.boxes[item].Intersects(q) {
			c.marks[slot|1] |= bit
		}
	}

	return dst
}

// finish turns the marks a walk left into the query's answer, appended to dst
// in ascending item order, and returns the extended slice. start is where this
// query's own output begins; everything before it is the caller's and is never
// read or moved.
//
// The bitset path emits nothing that the sort it replaces did not, in the order
// that sort produced, and the two claims come out of the same property: bit
// order is item order. Both levels are scanned low to high and TrailingZeros64
// takes the lowest set bit first, so the nested walk visits summary words, then
// item words, then items, each ascending — which composes to items ascending,
// the unique arrangement slices.Sort left behind. The walk's own order never
// reaches the output, and the duplicates the sort had to carry are the ones the
// tested bit already suppressed. The set is likewise unchanged: a mark is set on
// an item's first sighting and only ever suppresses a repeat, never a first
// occurrence, and Intersects still runs exactly once per distinct candidate.
// That it now runs in a different order is immaterial — it is a pure predicate
// over one box and the query.
//
// The uint32 generation's wraparound case goes with the legacy path and is not
// replaced by a wider counter: there is no counter left to wrap, because the
// emit pass clears every word it reads and hands the next query a zeroed bitset.
func (c *BBoxGridCursor) finish(start int, dst []int) []int {
	if c.legacy {
		slices.Sort(dst[start:])

		return dst
	}

	for summary := c.loTouched; summary <= c.hiTouched; summary++ {
		words := c.touched[summary]
		c.touched[summary] = 0

		for words != 0 {
			word := summary<<6 + bits.TrailingZeros64(words)
			words &= words - 1 // Clear the lowest set bit.

			slot := word << 1
			hits := c.marks[slot|1]

			c.marks[slot] = 0
			c.marks[slot|1] = 0

			for hits != 0 {
				dst = append(dst, word<<6+bits.TrailingZeros64(hits))
				hits &= hits - 1
			}
		}
	}

	return dst
}

// nextGeneration advances the legacy stamp, wiping the stamps on the one query
// in four billion that would otherwise wrap onto a live value.
func (c *BBoxGridCursor) nextGeneration() {
	c.generation++
	if c.generation != 0 {
		return
	}

	for i := range c.seen {
		c.seen[i] = 0
	}

	c.generation = 1
}
