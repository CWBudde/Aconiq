package road

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/aconiq/backend/internal/geo"
)

// This file holds the receiver's half of the reflection search: everything
// about a mirrored path that depends on where the receiver stands and not on
// where the Teilstück is, computed once per receiver and read by every one of
// that receiver's Teilstücke.
//
// # Why the receiver side
//
// The image-source search used to start from the source. For each Teilstück it
// mirrored the source across every wall, and for second order cast each first
// wall's shadow from that image and asked the wall index what the shadow
// reached. On a building-dense model that is thousands of shadow queries per
// (Teilstück, receiver) pair, each answering hundreds of candidate walls, to
// find the handful of paths that exist — and all of it was recomputed for the
// next Teilstück although the receiver had not moved.
//
// The geometry is symmetric. A path S → P1 → P2 → R can be unfolded from
// either end, and unfolding it from the receiver gives conditions in which the
// source appears only once, at the very end:
//
//   - First order off wall w. The search accepts the path when the segment
//     S' → R, S' = mirror(S, w), crosses w. Mirroring that segment back across
//     w's line — an isometry that fixes the line pointwise — turns it into
//     S → R', R' = mirror(R, w), crossing w at the same point. So S lies on a
//     ray from R' through a point of w, beyond w: S is in the shadow w casts
//     from a light at R'.
//   - Second order off w1 then w2. The search accepts when S'' → R crosses w2
//     at P2 and P2 → S' crosses w1 at P1. Mirroring across w2 maps S'' → R
//     onto S' → R' with P2 fixed, so S', P1, P2, R' lie on one line in that
//     order; hence P1 is in the shadow of w2 cast from R'. Mirroring that line
//     across w1 maps S' → S and R' → R'' = mirror(R', w1), with P1 fixed, so
//     S, P1, R'' are collinear with P1 between them: S is in the shadow of w1
//     cast from R''.
//
// R', R'' and which w1 can stand in a shadow of w2 are all receiver-only. So
// once per receiver this file lists, for every wall, the light R' its first-
// order shadow is cast from, and for every admissible wall pair the light R''
// the source must be shadowed from. A Teilstück then only has to ask, per
// entry, whether it stands in one wedge — a handful of cross products — and
// runs the unchanged exact search (mirror, wallSeg.crossing, secondOrderPath
// and Tabelle 8) on the few entries that admit it.
//
// # Why it is exact
//
// The wedge test is a filter in front of the old acceptance test, not a
// replacement for it, so the accepted set can only differ if the filter
// rejects a pair the old test would have accepted. The argument above says it
// never does in exact arithmetic; what remains is to show the margins cover
// floating point.
//
// The old test is computed, not exact: its mirror images and crossing points
// carry rounding of order ε — a few units in the last place of the coordinates,
// ~1e-9 m at UTM magnitudes; the constants below assume ε ≤ 1e-7 m, a hundred
// times that. The test also accepts a crossing up to reflectionEndToleranceM
// past a wall's end, which adds that much to ε and is covered the same way
// (see the constant). A crossing found by SegmentIntersection may sit off its exact
// position along the wall by ε/sin θ, θ the angle between ray and wall, which
// is unbounded at grazing incidence. But the wedge is bounded by rays from the
// light through the wall's endpoints, and moving a point along the wall by δ
// moves the ray through it by δ·sin θ at the wall — the same θ, because the
// unfolded ray and the mirrored one meet the wall at the same angle. The sin θ
// cancels: at the wall the error is ε, and at S, a distance |S−A| from the
// light A, it is ε·|S−A|/|P−A|, P the crossing. That ratio is what has to be
// bounded, and |P−A| is at least the light's distance from the wall's line.
// The same holds one bounce further back: an error of ε/sin θ₂ in P2 along w2
// moves the ray P2 → S' by ε perpendicular to itself, again because incidence
// and reflection share θ₂.
//
// Hence two rules. A light closer than wedgeApexClearanceM to its wall's line
// gets no wedge at all — the entry admits every source and the exact test
// decides alone. Every other wedge is widened on each side by
// wedgeMarginM + wedgeSlope·|S−A|, which exceeds ε + ε·|S−A|/wedgeApexClearanceM
// with a factor of five to spare. The distances are bounded above by their L1
// norms, which only widens the wedge further.
//
// The shadow query that lists the w1 candidates for each w2 is the same
// geo.SegmentShadow prune the source-side search used, and the same argument
// applies to it with the light R' and the crossing P1; its margin is
// receiverShadowMargin, sized to the scene, because a query cannot carry a
// distance-dependent widening.
//
// # The one way it can differ from the search it replaced
//
// "Exact" above means: it accepts exactly the paths the exact test accepts.
// The source-side search did not quite do that. Before the exact test it
// pruned second bounces with geo.SegmentShadow cast from the image source S',
// with a fixed 1 mm margin (queryMarginM) that is also the shadow's degeneracy
// threshold. By the argument above, its error at the second wall is
// ε·|P2−S'|/d, d the image source's distance from w1's line — so with the
// source a few millimetres off w1's line (d of the same order, since S' is S
// mirrored) and w2 a kilometre away, rounding can exceed that millimetre and
// the old prune can drop a path the exact test would have accepted. This
// search applies no source-side prune, so in that case it keeps the path: it
// can only ever *add* a path the old search wrongly pruned, never drop or
// change one. No such case has turned up in any comparison — the property
// tests, the city goldens, or the full Linden calc-area, all bit-identical — but
// the possibility is real, and a seeded mismatch of exactly that shape (an
// extra second-order path, source millimetres off the first wall's line, a
// distant second wall) is this and not a defect.
//
// # What it keeps
//
// The order of the paths is the old order: first order in wall order, then
// second order by first wall and, within one first wall, by second wall. The
// entries are generated second-wall-major and grouped by first wall; within a
// group they are laid out for search rather than in order (see pairGroup), so
// the few that admit a Teilstück are put back into second-wall order before
// any of them is turned into a path. Tabelle 8's range prune is the old one,
// with the receiver's half of it decided here once.
//
// # What it costs
//
// The entry list is the price: one per wall pair the receiver can see a
// second-order path through, 32 bytes each plus 8 bytes of staging, rebuilt
// per receiver into buffers the walk keeps. It is held per walker — per worker
// of a parallel run, for the worker's lifetime (see Walker), per Worker in the
// browser — never per receiver. It is not bounded: the pair count grows
// faster than the facade count, ~600 000 pairs on an 8 000-facade synthetic
// city and ~1 M (~40 MB) on the 8 083-facade Linden extract, and a larger
// extract costs proportionally more per worker. PLAN.md tracks that.

const (
	// wedgeMarginM is the absolute widening of every wedge boundary.
	wedgeMarginM = 1e-3

	// wedgeSlope is the widening per metre of distance from the light, on each
	// side of the wedge — 1 cm per kilometre.
	wedgeSlope = 1e-5

	// wedgeApexClearanceM is how far a light must stand off its wall's line
	// for the wedge to narrow anything.
	wedgeApexClearanceM = 0.05

	// receiverShadowFloorM is the least margin the receiver-side shadow query
	// is given; receiverShadowMargin grows it with the scene.
	receiverShadowFloorM = 0.05
)

// Flags on a mirrorWedge.
const (
	// wedgeOpen marks a wedge that admits every source: its light stands too
	// close to the wall's line for the wedge to be trusted.
	wedgeOpen uint8 = 1 << iota

	// wedgeNegative records the wedge's orientation: the sign of
	// cross(a−A, b−A) for wall [a,b] and light A.
	wedgeNegative

	// wedgeNeedsSourceRange marks an entry whose wall is in RLS-19 Tabelle 8
	// range only through the leg from the source, so a source out of range of
	// it fails the condition the old search tested.
	wedgeNeedsSourceRange

	// wedgeInadmissible marks a first-order wall that Tabelle 8 rejects at any
	// distance, because it is lower than 1.0 m.
	wedgeInadmissible
)

// mirrorWedge is one shadow a source has to stand in: the shadow of one wall
// cast from apex. For a first-order entry the wall is the entry's own; for a
// second-order entry it is the first wall of the pair, and other is the
// second.
type mirrorWedge struct {
	apex  geo.Point2D
	other int32
	flags uint8
}

// stagedPair is a second-order entry as the search finds it, second wall by
// second wall: just the two walls. The wedge is computed when the pair is
// placed in its group, so that the staging buffer — as long as the entry list
// and kept from one receiver to the next — holds eight bytes per entry rather
// than a whole wedge.
type stagedPair struct {
	first, second int32
}

// receiverMirrors is the receiver-side half of the search for one receiver.
//
// It is per walk, never per scene, and it is rebuilt whenever the walk moves
// to another receiver. The slices are kept from one receiver to the next so
// that a grid walk allocates them once.
type receiverMirrors struct {
	field    *reflectorField
	receiver geo.Point2D
	ready    bool

	// first is parallel to the field's walls.
	first []mirrorWedge

	// pairs holds the second-order entries grouped by first wall: the entries
	// whose first wall is i are pairs[start[i]:start[i+1]], laid out as
	// groups[i] describes.
	pairs  []mirrorWedge
	start  []int32
	groups []pairGroup

	// slopes is parallel to pairs and holds, for the entries of the sorted
	// runs, the light slope each run is sorted by — kept rather than
	// recomputed because every Teilstück binary-searches it.
	slopes []float64

	staged     []stagedPair
	candidates []int
	keyed      []keyedWedge

	// admitted is the per-group result buffer of the Teilstück walk.
	admitted []int32
}

// pairGroup describes how one first wall's entries are laid out in pairs.
//
// A group is split three ways. Open wedges come first, in second-wall order,
// and are tested one by one. Then the wedges whose light is on the negative
// side of the wall's line (right of a→b), then those on the positive side;
// each of those two runs is sorted by the light's slope, so that a source
// picks out the run of lights that can see it through the wall by binary
// search rather than by testing them all. See admittedPairs.
type pairGroup struct {
	openEnd, belowEnd int32
	below, above      slopeRun
}

// slopeRun bounds one sorted run of a group.
type slopeRun struct {
	// width is the largest L/H in the run, L the wall's length and H a light's
	// distance from its line: how far a light's two slopes are apart.
	width float64

	// reach is the largest L1 distance from the wall's first endpoint to a
	// light in the run, which bounds the wedges' distance-dependent margin.
	reach float64
}

// keyedWedge is an entry with the slope it is sorted by.
type keyedWedge struct {
	slope float64
	wedge mirrorWedge
}

// mirrorsFor returns the receiver-side search for receiver over f, building it
// when the scratch last served another receiver or another field.
//
// A nil scratch gets a fresh, unindexed one: the same entries from a walk over
// every wall, built for this one call.
func (f *reflectorField) mirrorsFor(receiver geo.Point2D, scratch *pathScratch) *receiverMirrors {
	if scratch == nil {
		mirrors := &receiverMirrors{}
		mirrors.build(f, receiver, nil)

		return mirrors
	}

	mirrors := &scratch.mirrors
	if !mirrors.ready || mirrors.field != f || mirrors.receiver != receiver {
		mirrors.build(f, receiver, scratch.wallCursor)
	}

	return mirrors
}

// build computes every wedge for one receiver.
func (m *receiverMirrors) build(f *reflectorField, receiver geo.Point2D, cursor *geo.BBoxGridCursor) {
	walls := f.walls

	m.field = f
	m.receiver = receiver
	m.ready = true
	m.first = m.first[:0]
	m.staged = m.staged[:0]

	margin := f.receiverShadowMargin(receiver)

	for j := range walls {
		w2 := &walls[j]
		if w2.maxRangeM < 0 {
			// Below 1.0 m: Tabelle 8 rejects it as the only bounce and as the
			// second, and the old search tested exactly that.
			m.first = append(m.first, mirrorWedge{flags: wedgeInadmissible})

			continue
		}

		receiverGapSq := w2.bbox.SquaredDistanceToPoint(receiver)
		light := mirrorPoint(receiver, w2.a, w2.b)

		firstOrder := newMirrorWedge(light, w2.a, w2.b)
		if receiverGapSq > w2.maxRangeSqM {
			firstOrder.flags |= wedgeNeedsSourceRange
		}

		m.first = append(m.first, firstOrder)

		m.stageSecondOrder(f, j, light, receiverGapSq, margin, cursor)
	}

	m.groupByFirstWall(len(walls))
}

// stageSecondOrder lists the first walls that can precede w2 = walls[j] on a
// path to the receiver; secondOrderWedge computes each pair's wedge later.
func (m *receiverMirrors) stageSecondOrder(
	f *reflectorField,
	j int,
	light geo.Point2D,
	receiverGapSq, margin float64,
	cursor *geo.BBoxGridCursor,
) {
	w2 := &f.walls[j]

	// P1 lies in the shadow w2 casts from the receiver's image R'.
	shadow := geo.NewSegmentShadow(light, w2.a, w2.b, margin)

	candidates := f.all
	if cursor.Ready() {
		m.candidates = cursor.QueryShadow(&shadow, margin, m.candidates[:0])
		candidates = m.candidates
	}

	for _, i := range candidates {
		if i == j {
			continue
		}

		w1 := &f.walls[i]
		if w1.maxRangeM < 0 {
			continue // below 1.0 m: Tabelle 8 rejects it at the first bounce
		}

		if !shadow.MayContainSegment(w1.a, w1.b) {
			continue
		}

		// Tabelle 8 at the second bounce reads only the gap between the walls
		// and the receiver, so it is decided here; at the first bounce it
		// may need the source, and the entry says so.
		gapSq := w1.bbox.SquaredDistanceToBBox(w2.bbox)
		if !w2.inTabelle8Range(gapSq, receiverGapSq) {
			continue
		}

		m.staged = append(m.staged, stagedPair{first: mustInt32(i), second: mustInt32(j)})
	}
}

// secondOrderWedge is the wedge a pair puts the source in: the shadow of the
// first wall cast from R” — the receiver mirrored across the second wall,
// which is that wall's first-order light, and then across the first.
func (m *receiverMirrors) secondOrderWedge(pair stagedPair) mirrorWedge {
	walls := m.field.walls
	w1 := &walls[pair.first]
	w2 := &walls[pair.second]

	wedge := newMirrorWedge(mirrorPoint(m.first[pair.second].apex, w1.a, w1.b), w1.a, w1.b)
	wedge.other = pair.second

	// Tabelle 8 at the first bounce: in range through the gap to the second
	// wall, or else only through the source leg, which the walk checks.
	if w1.bbox.SquaredDistanceToBBox(w2.bbox) > w1.maxRangeSqM {
		wedge.flags |= wedgeNeedsSourceRange
	}

	return wedge
}

// groupByFirstWall sorts the staged entries into pairs by first wall, stably,
// so that the second walls within one group stay ascending.
func (m *receiverMirrors) groupByFirstWall(wallCount int) {
	m.start = growInt32(m.start, wallCount+1)
	clear(m.start)

	for _, staged := range m.staged {
		m.start[staged.first+1]++
	}

	for i := 1; i <= wallCount; i++ {
		m.start[i] += m.start[i-1]
	}

	if cap(m.pairs) < len(m.staged) {
		m.pairs = make([]mirrorWedge, len(m.staged))
	}

	m.pairs = m.pairs[:len(m.staged)]
	m.slopes = growFloat64(m.slopes, len(m.staged))

	// Scatter, advancing each group's start as it fills, then shift the
	// starts back down by one group: start[i] has advanced to where group i+1
	// begins.
	for _, staged := range m.staged {
		m.pairs[m.start[staged.first]] = m.secondOrderWedge(staged)
		m.start[staged.first]++
	}

	copy(m.start[1:], m.start[:wallCount])
	m.start[0] = 0

	m.groups = m.groups[:0]

	for i := range wallCount {
		m.groups = append(m.groups, m.layOutGroup(&m.field.walls[i], m.start[i], m.start[i+1]))
	}
}

// layOutGroup reorders one group into its open wedges, in second-wall order,
// and its two sorted runs, and returns where they are.
func (m *receiverMirrors) layOutGroup(w1 *wallSeg, from, to int32) pairGroup {
	group := m.pairs[from:to]
	m.keyed = m.keyed[:0]

	open := 0

	for _, wedge := range group {
		if wedge.flags&wedgeOpen != 0 {
			group[open] = wedge
			open++

			continue
		}

		m.keyed = append(m.keyed, keyedWedge{slope: w1.lightSlope(wedge.apex), wedge: wedge})
	}

	// Lights below the line first, then above; within each, by slope, with
	// the second wall to break a tie so that the layout does not depend on
	// the sort's own choices.
	slices.SortFunc(m.keyed, func(x, y keyedWedge) int {
		xBelow := x.wedge.flags&wedgeNegative != 0
		yBelow := y.wedge.flags&wedgeNegative != 0

		switch {
		case xBelow != yBelow:
			if xBelow {
				return -1
			}

			return 1
		case x.slope != y.slope:
			return cmp.Compare(x.slope, y.slope)
		default:
			return cmp.Compare(x.wedge.other, y.wedge.other)
		}
	})

	layout := pairGroup{
		openEnd:  from + mustInt32(open),
		belowEnd: from + mustInt32(open),
		below:    slopeRun{width: 0, reach: 0},
		above:    slopeRun{width: 0, reach: 0},
	}

	for k, keyed := range m.keyed {
		group[open+k] = keyed.wedge
		m.slopes[int(from)+open+k] = keyed.slope

		run := &layout.above
		if keyed.wedge.flags&wedgeNegative != 0 {
			run = &layout.below
			layout.belowEnd++
		}

		_, across := w1.lightFrame(keyed.wedge.apex)

		run.width = math.Max(run.width, w1.lengthM/math.Abs(across))
		run.reach = math.Max(run.reach, math.Abs(keyed.wedge.apex.X-w1.a.X)+math.Abs(keyed.wedge.apex.Y-w1.a.Y))
	}

	// One part in a billion more, for the rounding between a light's slope
	// as sorted and its other slope as the window bounds it.
	layout.below.width = layout.below.width*(1+1e-9) + 1e-12
	layout.above.width = layout.above.width*(1+1e-9) + 1e-12

	return layout
}

// lightFrame returns p in the wall's own frame: along the wall from a towards
// b, and across it, positive to the left of a→b.
func (w *wallSeg) lightFrame(p geo.Point2D) (float64, float64) {
	dx, dy := p.X-w.a.X, p.Y-w.a.Y

	return dx*w.tangent.X + dy*w.tangent.Y, dy*w.tangent.X - dx*w.tangent.Y
}

// lightSlope is the slope a run is sorted by: for a light at distance H from
// the wall's line and t along it, (t − L)/H — the tangent, against the wall's
// normal, of the ray from the light through the wall's far endpoint b. The
// ray through a has slope t/H, which is this plus L/H.
func (w *wallSeg) lightSlope(light geo.Point2D) float64 {
	along, across := w.lightFrame(light)

	return (along - w.lengthM) / math.Abs(across)
}

func growInt32(s []int32, n int) []int32 {
	if cap(s) < n {
		return make([]int32, n)
	}

	return s[:n]
}

// newMirrorWedge builds the wedge of wall [a,b] lit from apex.
func newMirrorWedge(apex, a, b geo.Point2D) mirrorWedge {
	wedge := mirrorWedge{apex: apex, other: -1, flags: 0}

	eax, eay := a.X-apex.X, a.Y-apex.Y
	ebx, eby := b.X-apex.X, b.Y-apex.Y

	// cross(a−A, b−A) is twice the triangle's area, so divided by the wall's
	// length it is the light's distance from the wall's line.
	orientation := eax*eby - eay*ebx
	span := math.Hypot(b.X-a.X, b.Y-a.Y)

	if !(span > 0) || !(math.Abs(orientation) > wedgeApexClearanceM*span) {
		wedge.flags |= wedgeOpen

		return wedge
	}

	if orientation < 0 {
		wedge.flags |= wedgeNegative
	}

	return wedge
}

// admittedPairs returns the indices into pairs of the entries of first wall i
// whose wedge admits source and whose Tabelle 8 range the source does not
// rule out, in second-wall order — the order the source-side search tested
// them in.
//
// Every entry it returns has passed admits, and admits is the only filter: the
// sorted runs exist to skip, by binary search, the entries admits would
// reject. Measured in the wall's frame, with a light at t along the wall and H
// off its line, a source at tS along and h beyond it, and L the wall's length,
// admits' two edge tests imply
//
//	(t/H)·h + tS ≥ −s·(|t/H| + 1)
//	(L − tS) − ((t−L)/H)·h ≥ −s·(|(t−L)/H| + 1)
//
// with s = slopeWindowFactor times the run's largest wedge margin: the margin
// admits applies is measured in L1 norm against an edge vector of length
// H·√(slope²+1), and √2·(|slope|+1) bounds both conversions. For h > s each
// left-hand side is increasing in its slope, so the first bounds the slope
// through a from below, the second the slope through b from above — and as
// the two differ by L/H ≤ width, the run's lights that can pass are those
// whose slope through b lies in [lower − width, upper]. A source within s of
// the wall's line gets the whole run tested.
func (m *receiverMirrors) admittedPairs(i int, w1 *wallSeg, source geo.Point2D) []int32 {
	layout := &m.groups[i]
	admitted := m.admitted[:0]

	sourceInRange := w1.bbox.SquaredDistanceToPoint(source) <= w1.maxRangeSqM
	side := sideOf(w1.a, w1.b, source)

	admitted = m.admitRange(admitted, w1, source, side, sourceInRange, m.start[i], layout.openEnd)

	along, across := w1.lightFrame(source)
	sourceReach := math.Abs(source.X-w1.a.X) + math.Abs(source.Y-w1.a.Y)

	// Lights below the line see a source above it, and the other way round.
	// A run whose lights share the source's side is skipped whole, which is
	// exactly what admits would answer for each of them.
	if !side.negative {
		lo, hi := m.slopeWindow(w1, layout.openEnd, layout.belowEnd, layout.below, along, across, sourceReach)
		admitted = m.admitRange(admitted, w1, source, side, sourceInRange, lo, hi)
	}

	if !side.positive {
		lo, hi := m.slopeWindow(w1, layout.belowEnd, m.start[i+1], layout.above, along, -across, sourceReach)
		admitted = m.admitRange(admitted, w1, source, side, sourceInRange, lo, hi)
	}

	// Three runs, each in its own order: put the survivors back into
	// second-wall order. There are rarely more than a handful.
	for k := 1; k < len(admitted); k++ {
		for j := k; j > 0 && m.pairs[admitted[j]].other < m.pairs[admitted[j-1]].other; j-- {
			admitted[j], admitted[j-1] = admitted[j-1], admitted[j]
		}
	}

	m.admitted = admitted

	return admitted
}

// admitRange tests pairs[lo:hi] one by one.
func (m *receiverMirrors) admitRange(
	dst []int32,
	w1 *wallSeg,
	source geo.Point2D,
	side wallSide,
	sourceInRange bool,
	lo, hi int32,
) []int32 {
	for k := lo; k < hi; k++ {
		wedge := &m.pairs[k]
		if wedge.flags&wedgeNeedsSourceRange != 0 && !sourceInRange {
			continue
		}

		if wedge.admits(w1.a, w1.b, source, side) {
			dst = append(dst, k)
		}
	}

	return dst
}

// slopeWindowFactor is s/(largest wedge margin) in admittedPairs: √2 for the
// norm conversions, rounded up to 3 so that the bound holds with room to spare
// against the rounding of the slopes themselves.
const slopeWindowFactor = 3

// slopeWindow narrows the sorted run pairs[lo:hi] to the lights that can see
// a source at along, beyond the wall's line by beyond, through the wall. See
// admittedPairs.
func (m *receiverMirrors) slopeWindow(
	w1 *wallSeg,
	lo, hi int32,
	run slopeRun,
	along, beyond, sourceReach float64,
) (int32, int32) {
	if lo >= hi {
		return lo, hi
	}

	s := slopeWindowFactor * (wedgeMarginM + wedgeSlope*(sourceReach+run.reach))
	if !(beyond > s) {
		return lo, hi
	}

	// The inverses of h·σ ± s·|σ|, both increasing for h > s.
	atLeast := func(y float64) float64 {
		if y >= 0 {
			return y / (beyond + s)
		}

		return y / (beyond - s)
	}
	atMost := func(y float64) float64 {
		if y >= 0 {
			return y / (beyond - s)
		}

		return y / (beyond + s)
	}

	lower := atLeast(-along-s) - run.width
	upper := atMost(w1.lengthM - along + s)

	slopes := m.slopes[lo:hi]
	if upper < slopes[0] || lower > slopes[len(slopes)-1] {
		return lo, lo
	}

	first := searchSlopes(slopes, func(slope float64) bool { return slope >= lower })
	last := first + searchSlopes(slopes[first:], func(slope float64) bool { return slope > upper })

	return lo + mustInt32(first), lo + mustInt32(last)
}

// searchSlopes is sort.Search over a run of slopes.
func searchSlopes(slopes []float64, at func(float64) bool) int {
	low, high := 0, len(slopes)

	for low < high {
		mid := int(uint(low+high) >> 1)
		if at(slopes[mid]) {
			high = mid
		} else {
			low = mid + 1
		}
	}

	return low
}

// mustInt32 narrows an index into one of the walk's tables — walls, entries,
// barriers — to the int32 they are stored as. Both bounds are checked; a
// model with 2³¹ of any of them would not fit in memory to begin with.
func mustInt32(value int) int32 {
	if value < 0 || value > math.MaxInt32 {
		panic(fmt.Sprintf("index %d is out of range for int32", value))
	}

	return int32(value)
}

// wallSide is where a source stands relative to one wall's line, as the wedge
// test needs it: whether it is clearly on the positive or clearly on the
// negative side, with the same widening the wedge edges get.
type wallSide struct {
	positive, negative bool
}

// sideOf classifies source against the line of wall [a,b].
//
// The source must be beyond the wall as seen from the light. The light of a
// wedge with positive orientation is on the positive side of the line — the
// orientation is cross(b−a, A−a) as well as cross(a−A, b−A) — so such a wedge
// rejects a source clearly on the positive side, and a negative one a source
// clearly on the negative side.
func sideOf(a, b, source geo.Point2D) wallSide {
	abx, aby := b.X-a.X, b.Y-a.Y
	sx, sy := source.X-a.X, source.Y-a.Y

	value := abx*sy - aby*sx
	slack := (math.Abs(abx) + math.Abs(aby)) * (wedgeMarginM + wedgeSlope*(math.Abs(sx)+math.Abs(sy)))

	return wallSide{positive: value > slack, negative: value < -slack}
}

// admits reports whether source may stand in the shadow of wall [a,b] cast
// from the wedge's light, widened as the file comment requires.
func (w *mirrorWedge) admits(a, b, source geo.Point2D, side wallSide) bool {
	if w.flags&wedgeOpen != 0 {
		return true
	}

	negative := w.flags&wedgeNegative != 0
	if (negative && side.negative) || (!negative && side.positive) {
		return false // on the light's side of the wall
	}

	eax, eay := a.X-w.apex.X, a.Y-w.apex.Y
	ebx, eby := b.X-w.apex.X, b.Y-w.apex.Y
	dx, dy := source.X-w.apex.X, source.Y-w.apex.Y

	slack := wedgeMarginM + wedgeSlope*(math.Abs(dx)+math.Abs(dy))

	// The source must be on b's side of the ray through a, and on a's side of
	// the ray through b. With the orientation folded in, both read "≥ 0".
	alongA := eax*dy - eay*dx
	alongB := dx*eby - dy*ebx

	if negative {
		alongA, alongB = -alongA, -alongB
	}

	return alongA >= -slack*(math.Abs(eax)+math.Abs(eay)) &&
		alongB >= -slack*(math.Abs(ebx)+math.Abs(eby))
}

// receiverShadowMargin is the margin for the shadow a wall casts from the
// receiver's image, sized to the scene.
//
// The shadow query cannot widen with distance the way a wedge does, so it
// takes the widest the scene needs: the error at P1 is ε·|P1−R'|/m for a light
// at least m off the wall's line — m is also the query's degeneracy threshold
// — and |P1−R'| is at most twice the walls' extent plus the receiver's
// distance from them. m² ≥ 1e-6·reach covers ε ≤ 1e-7 m tenfold.
func (f *reflectorField) receiverShadowMargin(receiver geo.Point2D) float64 {
	if !f.boundsOK {
		return receiverShadowFloorM
	}

	diagonal := math.Hypot(f.bounds.Width(), f.bounds.Height())
	reach := 2*diagonal + math.Sqrt(f.bounds.SquaredDistanceToPoint(receiver))

	return math.Max(receiverShadowFloorM, math.Sqrt(1e-6*reach))
}
