package road

import (
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// SegmentLengthMode selects how long a Teilstück is.
//
// It is a declared run parameter rather than an internal optimisation,
// because the two modes discretise the source line differently and therefore
// produce different levels. Which one produced a number is part of what the
// number means, so it is stamped into provenance.json alongside
// segment_length_m and not merely logged.
type SegmentLengthMode string

const (
	// SegmentLengthFixed splits every source line at SegmentLengthM, however
	// far away the receiver is. It is the default, and it is also the zero
	// value, so a caller that has never heard of this parameter keeps the
	// behaviour it always had.
	SegmentLengthFixed SegmentLengthMode = "fixed"

	// SegmentLengthDistanceScaled lets a distant source line be split more
	// coarsely, up to the bound the standard publishes for exactly this.
	//
	// RLS-19 Nr. 3.3 requires source lines be divided "abhängig vom
	// Immissionsort ... in geeignete Teilstücke" — dependent on the receiver,
	// in so many words — and the Anmerkung to Nr. 3.2 gives the rule that
	// makes "geeignet" checkable for free propagation over flat ground:
	// l_i <= s_i / 2, the Teilstück no longer than half its distance to the
	// receiver. Fixed mode ignores both sentences and splits at one length
	// everywhere, which is why a 44 km OSM extract costs the same per
	// receiver whether the road is across the street or 900 m away.
	//
	// The saving is not marginal. On a city extract whose calculation area is
	// 86 m across, 96 % of the imported road is more than 100 m away and 80 %
	// is beyond 250 m; at s/2 those admit Teilstücke of 50 m and 125 m
	// against the 1 m fixed mode was using.
	//
	// The honest caveat, and the reason this is opt-in rather than the
	// default: the Faustregel is stated for "freie Schallausbreitung über
	// ebenem Boden", and this mode applies it without checking that the
	// precondition holds — a coarsened source line behind a building is
	// coarsened anyway. Inhibiting the coarsening inside a barrier or
	// building plan-shadow is the conservative form, and it is tracked in
	// PLAN.md as what has to land before this mode could ever become the
	// default.
	SegmentLengthDistanceScaled SegmentLengthMode = "distance_scaled"
)

// resolved reads the mode with the zero value spelled out, so that every
// comparison in the package is against a named constant rather than "".
func (m SegmentLengthMode) resolved() SegmentLengthMode {
	if m == "" {
		return SegmentLengthFixed
	}

	return m
}

// maxLadderLevels bounds how far a ladder coarsens, so that a degenerate
// input cannot spin here. Each rung doubles, so 24 of them take a 1 m
// Teilstück past 16 000 km; the loop stops long before this at the rung that
// holds the whole line in one Teilstück.
const maxLadderLevels = 24

// sourceLevel is one rung of the ladder: the whole source line split at
// lengthM, with the per-Teilstück base levels that go with that split.
//
// baseDayDB and baseNightDB are parallel to segments for the same reason they
// are on preparedSource, and they cannot be shared between rungs: the length
// weight 10·lg(l_i/l_0) is a function of the Teilstück length, so a rung with
// longer Teilstücke carries proportionally more power in each of them. That is
// what keeps the total emitted power the same at every rung.
type sourceLevel struct {
	// lengthM is the actual Teilstück length at this rung, which is
	// totalLength/n and therefore at or below the target it was built from.
	// The selection compares against this and never against the target.
	lengthM     float64
	segments    []Segment
	baseDayDB   []float64
	baseNightDB []float64
}

// newSourceLevel splits the line at targetLengthM and computes the base level
// of each Teilstück. ok is false for a line that yields no Teilstück, which is
// a degenerate geometry rather than an error — the same silence prepareSource
// gives it.
func newSourceLevel(
	line []geo.Point2D,
	elevations []float64,
	emission EmissionResult,
	targetLengthM float64,
) (sourceLevel, bool) {
	segments := SplitLineIntoSegments(line, elevations, targetLengthM)
	if len(segments) == 0 {
		return sourceLevel{}, false
	}

	baseDayDB := make([]float64, len(segments))
	baseNightDB := make([]float64, len(segments))

	for i, seg := range segments {
		// Written in the shape prepareSource wrote it, deliberately: the same
		// two statements in the same order, so that fixed mode — which reads
		// rung zero and nothing else — is byte-identical to the code this
		// replaced, on every architecture including the ones that contract a
		// multiply-add.
		lengthWeight := 10 * math.Log10(seg.LengthM/referenceLengthM)
		baseDayDB[i] = emission.LmEDay + lengthWeight
		baseNightDB[i] = emission.LmENight + lengthWeight
	}

	return sourceLevel{
		lengthM:     segments[0].LengthM,
		segments:    segments,
		baseDayDB:   baseDayDB,
		baseNightDB: baseNightDB,
	}, true
}

// buildCoarserLevels builds rungs 1..K at 2×, 4×, 8× … the finest length.
//
// Doubling, rather than a continuous function of the distance, is what keeps
// this affordable and what keeps it deterministic in the way the merge needs:
// the rungs are built once in PrepareScene, from the model alone, and a
// receiver only ever *chooses* between them. No receiver causes a split, so
// two runs that partition their receivers differently still read the same
// Teilstücke from the same slices.
//
// The whole ladder costs about one extra copy of rung zero: rung k holds
// n/2^k Teilstücke, and the series sums to under 2n.
func buildCoarserLevels(
	line []geo.Point2D,
	elevations []float64,
	emission EmissionResult,
	finest sourceLevel,
	finestTargetM float64,
) []sourceLevel {
	// A source already held in one Teilstück has nothing coarser to offer.
	if len(finest.segments) <= 1 {
		return nil
	}

	var levels []sourceLevel

	target := finestTargetM
	count := len(finest.segments)

	for range maxLadderLevels {
		target *= 2

		level, ok := newSourceLevel(line, elevations, emission, target)
		if !ok {
			break
		}

		// Doubling the target does not always halve the count — at the coarse
		// end, ceil rounds several targets to the same split. Keep only the
		// rungs that are actually coarser than the one before, so the
		// selection walk has no duplicates to step over.
		if len(level.segments) >= count {
			continue
		}

		levels = append(levels, level)
		count = len(level.segments)

		if count == 1 {
			break
		}
	}

	return levels
}

// levelFor returns the Teilstücke this receiver reads, and the base levels
// that belong to them.
//
// In fixed mode there is no ladder and this is rung zero and a nil check — the
// distance below is never computed, so the mode nobody opted into pays nothing
// for the one they did not.
//
// # Why the nearest point of the whole line
//
// The Anmerkung's s_i is the distance from *that* Teilstück to the receiver,
// so applying the rule per Teilstück would mean choosing a different rung for
// each one — and rungs are whole-line splits, so there would be nothing
// coherent to choose. Taking s as the distance to the nearest point of the
// entire source line makes one choice serve every Teilstück of it: every
// Teilstück midpoint is at least s away, so l <= s/2 implies l <= s_i/2 for
// all i. It is the conservative reading, and it is cheap — one point-to-
// polyline distance per source and receiver, against the thousands of
// propagation paths it decides.
//
// It is also why the ladder is worth having at the granularity the importer
// produces: an OSM extract arrives as one source per way, sixty metres on
// average, so "nearest point of the line" and "distance to the road" are the
// same number to within the length of a block. One very long source would
// pin itself to its closest end, which costs accuracy nowhere and speed only
// on that source.
func (p *preparedSource) levelFor(receiver geo.Point2D) ([]Segment, []float64, []float64) {
	if len(p.coarser) == 0 {
		return p.segments, p.baseDayDB, p.baseNightDB
	}

	admissibleM := geo.DistancePointToLineString(receiver, p.line) / 2

	segments, baseDay, baseNight := p.segments, p.baseDayDB, p.baseNightDB

	// Ascending in lengthM, so the first rung that breaks the bound ends the
	// walk. A receiver so close that even rung zero exceeds the bound keeps
	// rung zero: the mode is a licence to coarsen up to what the user asked
	// for, never an instruction to split finer than they did.
	for i := range p.coarser {
		if p.coarser[i].lengthM > admissibleM {
			break
		}

		segments = p.coarser[i].segments
		baseDay = p.coarser[i].baseDayDB
		baseNight = p.coarser[i].baseNightDB
	}

	return segments, baseDay, baseNight
}
