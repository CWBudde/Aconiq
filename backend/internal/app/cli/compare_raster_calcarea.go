package cli

import (
	"fmt"
	"math"
	"slices"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/soundplanimport"
)

// Where the raster comparison's calculation area came from, as recorded in the
// report and the artifact. An absent value means the comparison returned before
// it resolved one at all, which is not the same as calcAreaSourceNone.
const (
	calcAreaSourceModel        = "model"
	calcAreaSourceImportReport = "import_report"
	calcAreaSourceNone         = "none"
)

// What the resolved calculation area actually did for the synthesis, as
// recorded in the report and the artifact. The scanline path derives every
// receiver position from the area; the GM-metadata path derives positions from
// the grid's own origin and spacing and consults the area only to decide which
// way the decoded rows run. Without this, calc_area_source: model reads as
// "the model's area placed these receivers" on a path where it did not.
const (
	calcAreaRoleReceiverPlacement = "receiver_placement"
	calcAreaRoleRowDirectionOnly  = "row_direction_only"
)

// The unit calcAreaBoundsDelta is measured in — the project CRS's horizontal
// axis unit, because the delta is a plain subtraction of project-CRS
// coordinates.
const (
	calcAreaDeltaUnitMetre   = "m"
	calcAreaDeltaUnitDegree  = "degree"
	calcAreaDeltaUnitUnknown = "unknown"
)

// calcAreaFromImportReport reads the fallback calculation area out of the
// SoundPLAN import report, closing its ring.
//
// The point list is verbatim ParseCalcAreaFile output and nothing upstream
// enforces closure, while calcAreaHorizontalSpan's edge loop runs to
// len(Points)-1 and so never emits the closing edge. Left open, the fallback
// therefore drops one edge of the outline: a rectangle finds a single crossing
// per row, reports no span and falls back to the bounding box, and an outline
// with a notch keeps an even crossing count but pairs the crossings wrongly and
// can return the notch *gap* as the row span — placing receivers in exactly the
// region the drawing excludes. Closing it here gives the fallback the same
// guarantee the model path gets from validation.
//
// Closure is decided in 2D, as it is for the import's building appender: the
// scanline is 2D, and CalcArea.geo's z varies along the outline, so comparing z
// would leave a ring that closes in plan open.
func calcAreaFromImportReport(area *soundPlanImportCalcArea) *soundplanimport.CalcArea {
	if area == nil || len(area.Points) == 0 {
		return nil
	}

	out := &soundplanimport.CalcArea{Points: make([]soundplanimport.Point3D, 0, len(area.Points)+1)}
	for _, point := range area.Points {
		out.Points = append(out.Points, soundplanimport.Point3D{X: point.X, Y: point.Y, Z: point.Z})
	}

	first := out.Points[0]

	last := out.Points[len(out.Points)-1]
	if first.X != last.X || first.Y != last.Y {
		out.Points = append(out.Points, first)
	}

	return out
}

// calcAreaFromModel reads the calculation area out of the project model. It
// returns the area, the feature id for warning text, and any warnings; a nil
// area means "this model has none", and the caller falls back to the import
// report.
//
// The model's ring is the better-formed input of the two. Validation guarantees
// it is closed and carries at least four coordinates
// (modelgeojson/validate.go's parsePolygon), which is exactly what
// calcAreaHorizontalSpan's edge loop needs: it runs to len(Points)-1 and so
// never emits the closing edge, which is correct and complete for a closed ring
// and drops an edge for an open one. Nothing upstream of the import report's
// point list enforces closure, so calcAreaFromImportReport closes it.
func calcAreaFromModel(model modelgeojson.Model) (*soundplanimport.CalcArea, string, []string) {
	for _, feature := range model.Features {
		if feature.Kind != modelgeojson.FeatureKindCalcArea {
			continue
		}

		// At most one exists — calcAreaTracker in modelgeojson/validate.go
		// refuses a second — so the first match is the only match.
		rings, err := parsePolygonCoordinates(feature.Coordinates)
		if err != nil {
			// Falling back beats failing: the comparison still has an answer
			// from the import report, and a model whose geometry cannot be
			// parsed here is one built in code rather than read from disk.
			return nil, feature.ID, []string{fmt.Sprintf(
				"calculation area %q could not be read from the model (%v); using the SoundPLAN import report instead",
				feature.ID, err,
			)}
		}

		var warnings []string

		// soundplanimport.CalcArea is a flat point list with no way to carry a
		// hole, and calcAreaHorizontalSpan picks the widest gap between
		// crossings — handed an interior ring it would silently choose a
		// sub-span across the hole. Outer ring only, and say so.
		if len(rings) > 1 {
			warnings = append(warnings, fmt.Sprintf(
				"calculation area %q carries %d interior ring(s), which a flat point list cannot represent; the raster comparison uses the outer ring only",
				feature.ID, len(rings)-1,
			))
		}

		// Nothing reads this z today: the only read of .Z off a CalcArea in this
		// file is the field copy in calcAreaFromImportReport, and every consumer
		// — calcAreaBounds, calcAreaHorizontalSpan, metadataAlignedRowCenters —
		// is purely 2D. It is carried so the two areas have the same shape.
		//
		// Its absence is never "no area" — it says only that no base elevation
		// was recorded, and 0 is the right reading for that. It does not say
		// where the area came from: an ordinary GeoJSON import carries the
		// property only if the file did, and a SoundPLAN model saved from the
		// map before calcAreaToGeoJSON's properties passthrough lost it. What
		// is no longer true is the reverse claim this comment used to make,
		// that a save from the map drops it in every case; the passthrough
		// carries the area's own properties through now.
		baseElevationM, _, err := featurePropertyFloat(feature, "soundplan_base_elevation_m")
		if err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"calculation area %q: %v; base elevation read as 0", feature.ID, err,
			))

			baseElevationM = 0
		}

		points := make([]soundplanimport.Point3D, 0, len(rings[0]))
		for _, point := range rings[0] {
			points = append(points, soundplanimport.Point3D{X: point.X, Y: point.Y, Z: baseElevationM})
		}

		return &soundplanimport.CalcArea{Points: points}, feature.ID, warnings
	}

	return nil, "", nil
}

// rasterCalcArea is the calculation area the raster comparison synthesizes
// receivers over, together with the evidence for how it was chosen.
type rasterCalcArea struct {
	area   *soundplanimport.CalcArea
	source string
	// boundsDelta is set only when both sources carried an area, so nil means
	// "there was nothing to compare against", not "they agreed". boundsDeltaUnit
	// names the unit it is in, which is the project CRS's axis unit and not
	// necessarily metres.
	boundsDelta     *float64
	boundsDeltaUnit string
	warnings        []string
}

// resolveRasterCalcArea picks the calculation area the raster comparison uses.
//
// The model's calc-area feature wins. It is the extent a run actually computes
// over — calcAreaExtent (run_input.go) feeds it to resolveGridReceivers — so
// synthesizing receivers over anything else compares a run against a grid it did
// not use. The import report's CalcArea is the fallback, for a project carrying
// no calc-area feature: one imported before `aconiq import --soundplan` emitted
// one, or one whose CalcArea.geo was too degenerate to form a ring.
//
// **There is still no agreement tolerance, and now it is a choice rather than a
// blocked one.** A threshold here has to sit above the noise floor of an
// edit-free save and below a real edit. No such number existed while the
// EPSG:25832 → 4326 → 25832 round trip the map save path runs lost 0.49 m in a
// typical German project and 8.5 m at the edges of UTM zone 32, accumulating
// with every save: anything below the fixture's 5 m grid resolution fired on a
// save that changed nothing, anything above it hid a real edit. PLAN.md
// Priority 1.6 closed that — the save path now moves a vertex by nanometres —
// so a threshold is available and simply has not been chosen yet; Priority 13
// owns picking one from measured fixture data. Until then the two signals are:
//
//   - the vertex count, compared after normalising closure, which warns. That
//     comparison is exact and CRS-independent, so no transform error can reach
//     it, and a notch that leaves the envelope untouched still changes every
//     row span.
//   - the bounds delta, recorded unconditionally rather than judged, together
//     with the unit it is in. Priority 13 then reads a measured number instead
//     of a threshold verdict.
//
// Comparing the two areas vertex by vertex would be wrong: the lists legitimately
// differ in start vertex, in winding, and in whether closure is spelled out.
func resolveRasterCalcArea(model modelgeojson.Model, importReport soundPlanImportReport) rasterCalcArea {
	fromReport := calcAreaFromImportReport(importReport.CalcArea)

	fromModel, featureID, warnings := calcAreaFromModel(model)
	if fromModel == nil {
		if fromReport == nil {
			return rasterCalcArea{source: calcAreaSourceNone, warnings: warnings}
		}

		return rasterCalcArea{area: fromReport, source: calcAreaSourceImportReport, warnings: warnings}
	}

	chosen := rasterCalcArea{area: fromModel, source: calcAreaSourceModel, warnings: warnings}
	if fromReport == nil {
		return chosen
	}

	delta := calcAreaBoundsDelta(fromModel, fromReport)
	chosen.boundsDelta = &delta
	chosen.boundsDeltaUnit = calcAreaBoundsDeltaUnit(importReport.ProjectCRS)

	modelVertices := openVertexCount(fromModel)

	reportVertices := openVertexCount(fromReport)
	if modelVertices != reportVertices {
		chosen.warnings = append(chosen.warnings, fmt.Sprintf(
			"the model's calculation area %q has %d vertices and the SoundPLAN import report's has %d "+
				"(envelopes differ by %.6g %s); the model wins, because it is the extent a run computes over",
			featureID, modelVertices, reportVertices, delta, chosen.boundsDeltaUnit,
		))
	}

	return chosen
}

// calcAreaBoundsDeltaUnit names the unit calcAreaBoundsDelta reports in. The
// delta subtracts project-CRS coordinates, so it is metres only when the
// project CRS is projected; under the CLI's default EPSG:4326 it is degrees,
// where 0.001 is roughly 111 m of latitude. Reprojecting to force metres was
// ruled out while the 25832 <-> 4326 round trip lost up to 8.5 m — more than
// anything this delta would report. PLAN.md Priority 1.6 removed that
// objection; what remains is that a delta reported in a unit the caller did not
// ask for is worse than one reported with its unit named, which is what the
// return value does.
func calcAreaBoundsDeltaUnit(projectCRS string) string {
	parsed, err := geo.ParseCRS(projectCRS)
	if err != nil {
		return calcAreaDeltaUnitUnknown
	}

	switch parsed.Kind {
	case geo.CRSKindProjected:
		return calcAreaDeltaUnitMetre
	case geo.CRSKindGeographic:
		return calcAreaDeltaUnitDegree
	case geo.CRSKindUnknown:
		return calcAreaDeltaUnitUnknown
	}

	return calcAreaDeltaUnitUnknown
}

// openVertexCount counts an area's distinct vertices, ignoring whether it
// carries a repeated closing vertex. The two sources spell closure differently —
// the model's ring always repeats its first coordinate, the import report's
// point list is verbatim ParseCalcAreaFile output and may or may not — so a raw
// len() would report a difference that is notation rather than shape.
func openVertexCount(area *soundplanimport.CalcArea) int {
	points := area.Points
	if len(points) < 2 {
		return len(points)
	}

	first := points[0]

	last := points[len(points)-1]
	if first.X == last.X && first.Y == last.Y {
		return len(points) - 1
	}

	return len(points)
}

// calcAreaBoundsDelta is the largest absolute difference between two areas'
// axis-aligned envelopes, in the project CRS's axis unit — see
// calcAreaBoundsDeltaUnit. That envelope is what the synthesis actually
// consumes: calcAreaBounds drives the row centres and the bounding-box fallback
// span, and a bounds shift of δ moves every synthesized receiver by about δ/2
// while the comparison is per-cell.
func calcAreaBoundsDelta(first, second *soundplanimport.CalcArea) float64 {
	a := calcAreaBounds(first)
	b := calcAreaBounds(second)

	return max(
		math.Abs(a.MinX-b.MinX),
		math.Abs(a.MinY-b.MinY),
		math.Abs(a.MaxX-b.MaxX),
		math.Abs(a.MaxY-b.MaxY),
	)
}

func minYFromArea(area *soundplanimport.CalcArea) float64 {
	return calcAreaBounds(area).MinY
}

func maxYFromArea(area *soundplanimport.CalcArea) float64 {
	return calcAreaBounds(area).MaxY
}

// areaBounds is the axis-aligned envelope of a calculation area. Named fields
// rather than four positional float64 results, so a caller that needs one bound
// names it instead of counting blanks.
type areaBounds struct {
	MinX float64
	MinY float64
	MaxX float64
	MaxY float64
}

func calcAreaBounds(area *soundplanimport.CalcArea) areaBounds {
	bounds := areaBounds{
		MinX: area.Points[0].X,
		MinY: area.Points[0].Y,
		MaxX: area.Points[0].X,
		MaxY: area.Points[0].Y,
	}

	for _, point := range area.Points[1:] {
		bounds.MinX = math.Min(bounds.MinX, point.X)
		bounds.MinY = math.Min(bounds.MinY, point.Y)
		bounds.MaxX = math.Max(bounds.MaxX, point.X)
		bounds.MaxY = math.Max(bounds.MaxY, point.Y)
	}

	return bounds
}

func calcAreaHorizontalSpan(area *soundplanimport.CalcArea, y float64) (float64, float64, bool) {
	if area == nil || len(area.Points) < 4 {
		return 0, 0, false
	}

	intersections := make([]float64, 0, len(area.Points))
	for i := range len(area.Points) - 1 {
		a := area.Points[i]

		b := area.Points[i+1]
		if a.Y == b.Y {
			continue
		}

		minY := math.Min(a.Y, b.Y)

		maxY := math.Max(a.Y, b.Y)
		if y < minY || y >= maxY {
			continue
		}

		t := (y - a.Y) / (b.Y - a.Y)
		intersections = append(intersections, a.X+t*(b.X-a.X))
	}

	if len(intersections) < 2 {
		return 0, 0, false
	}

	slices.Sort(intersections)
	bestLeft := intersections[0]
	bestRight := intersections[1]
	bestWidth := bestRight - bestLeft

	for i := 2; i+1 < len(intersections); i += 2 {
		width := intersections[i+1] - intersections[i]
		if width > bestWidth {
			bestLeft = intersections[i]
			bestRight = intersections[i+1]
			bestWidth = width
		}
	}

	return bestLeft, bestRight, bestWidth > 0
}
