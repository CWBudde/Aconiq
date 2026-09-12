package schall03

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/aconiq/backend/internal/geo"
)

// speedSubstitutionExtentM is the reach of the Nr. 5.3.2 substitute speed on
// either side of a Weiche, Kreuzung or Haltestelle an Strecken.
const speedSubstitutionExtentM = 25.0

// trackFeatureMaxOffsetM bounds how far a TrackFeature may sit from the
// centerline it is attached to.  A feature further from the track than the
// reach of its own substitution zone is not on that track, and projecting it
// onto the nearest point regardless would invent a zone where Nr. 5.3.2 puts
// none.
const trackFeatureMaxOffsetM = speedSubstitutionExtentM

// TrackFeatureKind names the track features Nr. 5.3.2 scopes the 50 km/h
// substitute speed to.
type TrackFeatureKind string

const (
	// TrackFeatureWeiche is a Weiche.
	TrackFeatureWeiche TrackFeatureKind = "weiche"

	// TrackFeatureKreuzung is a Kreuzung.
	TrackFeatureKreuzung TrackFeatureKind = "kreuzung"

	// TrackFeatureHaltestelle is a Haltestelle an Strecken.
	TrackFeatureHaltestelle TrackFeatureKind = "haltestelle"
)

// TrackFeature locates one Nr. 5.3.2 track feature on a segment's centerline.
// The point is projected onto the centerline to find the chainage its
// substitution zone is centred on.
type TrackFeature struct {
	Kind  TrackFeatureKind `json:"kind"`
	Point geo.Point2D      `json:"point"`
}

// Validate checks one track feature.
func (f TrackFeature) Validate() error {
	switch f.Kind {
	case TrackFeatureWeiche, TrackFeatureKreuzung, TrackFeatureHaltestelle:
	default:
		return fmt.Errorf(
			"track feature kind must be one of %q, %q, %q, got %q",
			TrackFeatureWeiche, TrackFeatureKreuzung, TrackFeatureHaltestelle, f.Kind,
		)
	}

	if !f.Point.IsFinite() {
		return fmt.Errorf("track feature %q point is not finite", f.Kind)
	}

	return nil
}

// speedZonePart is one homogeneous stretch of a TrackSegment after the
// Nr. 5.3.2 zone split: a piece over which a single effective speed applies.
type speedZonePart struct {
	segment TrackSegment

	// substitutionSuppressed marks a part lying outside every substitution
	// zone, where the real track speed applies instead of the 50 km/h
	// substitute.  It is only ever set for a segment that declares features.
	substitutionSuppressed bool
}

// chainageSpan is a stretch of centerline in metres from its start.
type chainageSpan struct {
	from float64
	to   float64
}

// splitSegmentsForSpeedZones expands every segment into the homogeneous parts
// the Nr. 5.3.2 substitution needs.
//
// A segment that declares no TrackFeatures is passed through untouched.  That
// keeps the whole-segment reading for models that carry no Weichen- or
// Haltestellen-geometry, and — because the part is the original value, not a
// re-sliced copy — keeps its integration subdivision, and so its result,
// bit-for-bit what it was before speed zones existed.
func splitSegmentsForSpeedZones(segments []TrackSegment) ([]speedZonePart, error) {
	parts := make([]speedZonePart, 0, len(segments))

	for i := range segments {
		segParts, err := splitForSpeedZones(segments[i])
		if err != nil {
			return nil, fmt.Errorf("segment[%d]: %w", i, err)
		}

		parts = append(parts, segParts...)
	}

	return parts, nil
}

// splitForSpeedZones cuts one segment at the boundaries of its Nr. 5.3.2
// substitution zones.
func splitForSpeedZones(seg TrackSegment) ([]speedZonePart, error) {
	if len(seg.Features) == 0 {
		return []speedZonePart{{segment: seg}}, nil
	}

	total := geo.LineStringLength(seg.TrackCenterline)
	if !isFiniteLength(total) {
		return nil, fmt.Errorf("segment %q has no usable centerline length", seg.ID)
	}

	zones, err := substitutionZones(seg, total)
	if err != nil {
		return nil, err
	}

	return cutAtZones(seg, zones, total)
}

// substitutionZones projects every feature onto the centerline and returns the
// merged, ordered spans the substitute speed applies to.
//
// A feature lying just off the end of the line projects onto that end, so its
// zone is centred on the endpoint rather than on the feature itself.  That
// over-states the zone by the overshoot — bounded by trackFeatureMaxOffsetM,
// and in the conservative direction, since it substitutes the higher speed over
// slightly more track.
func substitutionZones(seg TrackSegment, total float64) ([]chainageSpan, error) {
	spans := make([]chainageSpan, 0, len(seg.Features))

	for _, feature := range seg.Features {
		err := feature.Validate()
		if err != nil {
			return nil, fmt.Errorf("segment %q: %w", seg.ID, err)
		}

		chainage, offset, ok := geo.ProjectPointOntoLineString(feature.Point, seg.TrackCenterline)
		if !ok {
			return nil, fmt.Errorf(
				"segment %q: cannot project track feature %q onto the centerline",
				seg.ID, feature.Kind,
			)
		}

		if offset > trackFeatureMaxOffsetM {
			return nil, fmt.Errorf(
				"segment %q: track feature %q lies %.1f m from the centerline, beyond the %.0f m bound",
				seg.ID, feature.Kind, offset, trackFeatureMaxOffsetM,
			)
		}

		spans = append(spans, chainageSpan{
			from: max(chainage-speedSubstitutionExtentM, 0),
			to:   min(chainage+speedSubstitutionExtentM, total),
		})
	}

	return mergeSpans(spans), nil
}

// mergeSpans orders spans by start and collapses every overlap or touch, so two
// features less than 50 m apart yield one zone rather than two abutting ones
// that would be integrated separately.
func mergeSpans(spans []chainageSpan) []chainageSpan {
	if len(spans) == 0 {
		return nil
	}

	sorted := slices.Clone(spans)
	slices.SortFunc(sorted, func(a, b chainageSpan) int {
		return cmp.Compare(a.from, b.from)
	})

	merged := []chainageSpan{sorted[0]}

	for _, span := range sorted[1:] {
		last := &merged[len(merged)-1]
		if span.from <= last.to {
			last.to = max(last.to, span.to)

			continue
		}

		merged = append(merged, span)
	}

	return merged
}

// cutAtZones walks the centerline once and emits alternating out-of-zone and
// in-zone parts covering it exactly.
func cutAtZones(seg TrackSegment, zones []chainageSpan, total float64) ([]speedZonePart, error) {
	parts := make([]speedZonePart, 0, 2*len(zones)+1)
	cursor := 0.0

	for _, zone := range zones {
		parts = appendPart(parts, seg, chainageSpan{from: cursor, to: zone.from}, true)
		parts = appendPart(parts, seg, zone, false)
		cursor = zone.to
	}

	parts = appendPart(parts, seg, chainageSpan{from: cursor, to: total}, true)

	if len(parts) == 0 {
		return nil, fmt.Errorf("segment %q: speed zone split produced no parts", seg.ID)
	}

	return parts, nil
}

// appendPart adds the part covering span, unless the span is empty — which is
// how the zero-length stretches before a zone starting at 0, or after one
// ending at the far end, are dropped.
func appendPart(parts []speedZonePart, seg TrackSegment, span chainageSpan, suppressed bool) []speedZonePart {
	centerline, ok := geo.SliceLineString(seg.TrackCenterline, span.from, span.to)
	if !ok {
		return parts
	}

	part := seg
	part.TrackCenterline = centerline
	part.ID = fmt.Sprintf("%s#%.0f-%.0f", seg.ID, span.from, span.to)

	return append(parts, speedZonePart{segment: part, substitutionSuppressed: suppressed})
}

// isFiniteLength reports whether a computed centerline length can be split on.
func isFiniteLength(total float64) bool {
	return total > 0 && !math.IsNaN(total) && !math.IsInf(total, 0)
}
