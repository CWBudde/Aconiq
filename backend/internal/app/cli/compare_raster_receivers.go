package cli

import (
	"fmt"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/soundplanimport"
)

// synthesizeRasterReceivers places one Aconiq receiver per decoded SoundPLAN
// grid cell, preferring the GM origin metadata and falling back to CalcArea
// scanlines. It records the chosen alignment and any warnings on report, and
// reports false when no receivers could be synthesized.
func synthesizeRasterReceivers(
	report *soundPlanRasterCompareReport,
	importReport soundPlanImportReport,
	meta soundplanimport.GridMapMetadata,
	calcArea *soundplanimport.CalcArea,
	receiverHeightM float64,
	layoutRows [][]soundplanimport.GridMapCell,
) ([]heuristicRasterReceiver, []string, bool) {
	syntheticReceivers, ids, synthWarnings := buildMetadataAlignedRasterReceivers(meta, calcArea, receiverHeightM, layoutRows)
	if len(syntheticReceivers) > 0 {
		report.Alignment = soundPlanRasterMetadataAlignment
		report.CalcAreaRole = calcAreaRoleRowDirectionOnly

		// Say so where the source field would otherwise overclaim. The decoded
		// values live at the GM grid's cells and nowhere else, so placing the
		// receivers anywhere but there would compare an Aconiq level at one
		// point against a SoundPLAN level at another — but a reader who sees
		// only calc_area_source: model would expect moving the area to move
		// them, and it does not. It can still flip the row direction.
		if report.CalcAreaSource == calcAreaSourceModel {
			report.Warnings = append(report.Warnings,
				"GM origin metadata placed the raster receivers on the SoundPLAN grid; the model's calculation area "+
					"only chose the row direction, so editing it does not move them")
		}
	}

	if len(syntheticReceivers) == 0 {
		gridResolutionM := importReport.GridResolutionM

		if calcArea == nil || len(calcArea.Points) < 4 {
			report.Warnings = append(report.Warnings, "CalcArea.geo is missing or incomplete, and GM metadata was insufficient for raster synthesis")
			return nil, nil, false
		}

		if gridResolutionM <= 0 {
			report.Warnings = append(report.Warnings, "grid resolution is unavailable, so raster scanline receivers could not be synthesized")
			return nil, nil, false
		}

		syntheticReceivers, ids, synthWarnings = buildHeuristicRasterReceivers(calcArea, gridResolutionM, receiverHeightM, layoutRows)
		report.Alignment = "calcarea_scanlines_centered"
		report.CalcAreaRole = calcAreaRoleReceiverPlacement
	}

	report.Warnings = append(report.Warnings, synthWarnings...)
	if len(syntheticReceivers) == 0 {
		report.Warnings = append(report.Warnings, "heuristic raster receiver synthesis produced no receivers")
		return nil, nil, false
	}

	return syntheticReceivers, ids, true
}

// syntheticRasterReceiverHeight returns the height the synthetic raster
// receivers are placed at.
//
// It comes from the SoundPLAN project's own grid-map setting (`RLKHEIGHT`,
// carried as soundPlanImportReport.GridMapHeightM), because that is the height
// the reference grid maps were computed at and the one the comparison has to
// reproduce. A grid map's GM metadata records no height, so nothing in the
// raster files themselves can supply it.
//
// This used to read the first receiver in the model instead, which made every
// raster delta a hostage to the receiver import: it happened to return 2.0 m
// only because the SoundPLAN importer put every receiver at the project's
// default facade height, which for this project equals RLKHEIGHT by
// coincidence. Facade receivers now sit at their own per-floor heights, and
// the first one is no longer anything a grid map has to do with.
func syntheticRasterReceiverHeight(importReport soundPlanImportReport) float64 {
	if importReport.GridMapHeightM > 0 {
		return importReport.GridMapHeightM
	}

	return defaultGridMapReceiverHeightM
}

// selectedRunReceiverHeight returns the height the synthetic raster receivers
// are placed at for the one grid map that was selected, plus any warning the
// choice owes the reader.
//
// The height has to come from the run being compared, not from the project.
// RLKHEIGHT is the project's *current* grid-map setting, and the selection can
// legitimately land on a run computed at another: --soundplan-grid-run names a
// run outright, and the geometry signal alone can settle the choice before the
// height is ever consulted. Placing the Aconiq receivers at RLKHEIGHT then
// compares Aconiq levels at one height against SoundPLAN cells at another while
// the report states the run was chosen on purpose — the very mismatch selecting
// a single run exists to remove.
//
// The project value stays the fallback for a run that declared no layout, which
// is every import report written before GridMapMetadata.RunLayout existed. A
// declared height of zero is treated the same way: nothing places receivers at
// ground level, so it is a parse artefact rather than a grid.
func selectedRunReceiverHeight(
	meta soundplanimport.GridMapMetadata,
	importReport soundPlanImportReport,
) (float64, []string) {
	layout := meta.RunLayout
	if layout == nil || layout.HeightM <= 0 {
		return syntheticRasterReceiverHeight(importReport), nil
	}

	// Only a recorded RLKHEIGHT can disagree. When the bundle recorded none,
	// syntheticRasterReceiverHeight would have answered with an assumed default,
	// and warning that the run contradicts an assumption would be noise.
	projectHeightM := importReport.GridMapHeightM
	if projectHeightM <= 0 || math.Abs(layout.HeightM-projectHeightM) <= gridMapHeightToleranceM {
		return layout.HeightM, nil
	}

	return layout.HeightM, []string{fmt.Sprintf(
		"SoundPLAN grid map %s was computed at %g m, but the project records RLKHEIGHT %g m; "+
			"the synthetic raster receivers are placed at %g m so both sides of the comparison sit at the same height",
		meta.ResultSubFolder, layout.HeightM, projectHeightM, layout.HeightM,
	)}
}

type heuristicRasterReceiver struct {
	ID      string
	X       float64
	Y       float64
	HeightM float64
	Row     int
	Col     int
}

func buildHeuristicRasterReceivers(
	area *soundplanimport.CalcArea,
	gridResolutionM float64,
	receiverHeightM float64,
	rows [][]soundplanimport.GridMapCell,
) ([]heuristicRasterReceiver, []string, []string) {
	if area == nil || len(area.Points) < 4 || len(rows) == 0 || gridResolutionM <= 0 {
		return nil, nil, nil
	}

	bounds := calcAreaBounds(area)
	yPositions := heuristicRasterRowCenters(bounds.MinY, bounds.MaxY, gridResolutionM, len(rows))

	receivers := make([]heuristicRasterReceiver, 0, countGridMapCells(rows))
	ids := make([]string, 0, countGridMapCells(rows))
	warnings := make([]string, 0, 4)

	for rowIndex, row := range rows {
		if len(row) == 0 {
			continue
		}

		y := yPositions[rowIndex]

		left, right, ok := calcAreaHorizontalSpan(area, y)
		if !ok || right <= left {
			left = bounds.MinX
			right = bounds.MaxX

			warnings = append(warnings, fmt.Sprintf("row %d could not be intersected with CalcArea; fell back to bounding box span", rowIndex))
		}

		xs := heuristicRowXPositions(left, right, gridResolutionM, len(row))
		for colIndex, x := range xs {
			id := fmt.Sprintf("%sr%03d-c%03d", soundPlanRasterReceiverPrefix, rowIndex+1, colIndex+1)
			receivers = append(receivers, heuristicRasterReceiver{
				ID:      id,
				X:       x,
				Y:       y,
				HeightM: receiverHeightM,
				Row:     rowIndex,
				Col:     colIndex,
			})
			ids = append(ids, id)
		}
	}

	return receivers, ids, uniqueStrings(warnings)
}

// anyGridMapRowHasCells reports whether at least one decoded row carries cells.
func anyGridMapRowHasCells(rows [][]soundplanimport.GridMapCell) bool {
	for _, row := range rows {
		if len(row) > 0 {
			return true
		}
	}

	return false
}

// gridMapMetadataFinite reports whether origin and spacing are usable numbers.
func gridMapMetadataFinite(meta soundplanimport.GridMapMetadata) bool {
	return !(math.IsNaN(meta.OriginX) || math.IsInf(meta.OriginX, 0) ||
		math.IsNaN(meta.OriginY) || math.IsInf(meta.OriginY, 0) ||
		math.IsNaN(meta.SpacingX) || math.IsInf(meta.SpacingX, 0) ||
		math.IsNaN(meta.SpacingY) || math.IsInf(meta.SpacingY, 0))
}

// gridMapAlignmentUsable reports whether the GM metadata can place the decoded
// rows on the project grid. When it cannot, it returns the warning explaining why
// (nil when the decoded payload simply carries no cells).
func gridMapAlignmentUsable(meta soundplanimport.GridMapMetadata, rows [][]soundplanimport.GridMapCell) ([]string, bool) {
	if len(rows) == 0 || !anyGridMapRowHasCells(rows) {
		return nil, false
	}

	if meta.OriginX == 0 && meta.OriginY == 0 {
		return []string{"skip GM metadata receiver synthesis: missing origin"}, false
	}

	if meta.SpacingX <= 0 || meta.SpacingY <= 0 {
		return []string{"skip GM metadata receiver synthesis: missing or non-positive spacing"}, false
	}

	if !gridMapMetadataFinite(meta) {
		return []string{"skip GM metadata receiver synthesis: invalid origin/spacing metadata"}, false
	}

	if meta.DeclaredRowCount > 0 && meta.DeclaredRowCount != len(rows) {
		return []string{fmt.Sprintf("skip GM metadata receiver synthesis: row count mismatch (metadata=%d, decoded=%d)", meta.DeclaredRowCount, len(rows))}, false
	}

	return nil, true
}

func buildMetadataAlignedRasterReceivers(
	meta soundplanimport.GridMapMetadata,
	area *soundplanimport.CalcArea,
	receiverHeightM float64,
	rows [][]soundplanimport.GridMapCell,
) ([]heuristicRasterReceiver, []string, []string) {
	warnings, usable := gridMapAlignmentUsable(meta, rows)
	if !usable {
		return nil, nil, warnings
	}

	yPositions := metadataAlignedRowCenters(meta, len(rows), area)
	receivers := make([]heuristicRasterReceiver, 0, countGridMapCells(rows))
	ids := make([]string, 0, countGridMapCells(rows))

	for rowIndex, row := range rows {
		if len(row) == 0 {
			continue
		}

		for colIndex := range row {
			id := fmt.Sprintf("%sr%03d-c%03d", soundPlanRasterReceiverPrefix, rowIndex+1, colIndex+1)
			receivers = append(receivers, heuristicRasterReceiver{
				ID:      id,
				X:       meta.OriginX + float64(colIndex)*meta.SpacingX,
				Y:       yPositions[rowIndex],
				HeightM: receiverHeightM,
				Row:     rowIndex,
				Col:     colIndex,
			})
			ids = append(ids, id)
		}
	}

	return receivers, ids, nil
}

func metadataAlignedRowCenters(meta soundplanimport.GridMapMetadata, rowCount int, area *soundplanimport.CalcArea) []float64 {
	if rowCount <= 0 {
		return nil
	}

	yFromOrigin := make([]float64, 0, rowCount)

	yFromReverse := make([]float64, 0, rowCount)
	for i := range rowCount {
		yFromOrigin = append(yFromOrigin, meta.OriginY+float64(i)*meta.SpacingY)
		yFromReverse = append(yFromReverse, meta.OriginY-float64(i)*meta.SpacingY)
	}

	if area == nil || len(area.Points) == 0 {
		return yFromOrigin
	}

	if len(area.Points) < 4 {
		return yFromOrigin
	}

	minY := minYFromArea(area)
	maxY := maxYFromArea(area)

	scoreFromOrigin := rasterSpanOverlapScore(yFromOrigin, minY, maxY)

	scoreFromReverse := rasterSpanOverlapScore(yFromReverse, minY, maxY)
	if scoreFromOrigin >= scoreFromReverse {
		return yFromOrigin
	}

	return yFromReverse
}

func rasterSpanOverlapScore(values []float64, minY, maxY float64) float64 {
	if len(values) < 2 {
		return 0
	}

	return spanOverlap(minY, maxY, values[0], values[len(values)-1])
}

func spanOverlap(a0, a1, b0, b1 float64) float64 {
	low := math.Max(math.Min(a0, a1), math.Min(b0, b1))

	high := math.Min(math.Max(a0, a1), math.Max(b0, b1))
	if high <= low {
		return 0
	}

	return high - low
}

func appendSyntheticRasterReceivers(model modelgeojson.Model, receivers []heuristicRasterReceiver) modelgeojson.Model {
	out := model
	out.Features = make([]modelgeojson.Feature, 0, len(model.Features)+len(receivers))

	for _, feature := range model.Features {
		if feature.Kind == modelgeojson.FeatureKindReceiver && strings.HasPrefix(feature.ID, soundPlanRasterReceiverPrefix) {
			continue
		}

		out.Features = append(out.Features, feature)
	}

	for _, receiver := range receivers {
		out.Features = append(out.Features, modelgeojson.Feature{
			ID:      receiver.ID,
			Kind:    modelgeojson.FeatureKindReceiver,
			HeightM: float64Ptr(receiver.HeightM),
			Properties: map[string]any{
				"soundplan_raster_compare": true,
				"soundplan_raster_row":     receiver.Row,
				"soundplan_raster_col":     receiver.Col,
			},
			GeometryType: modelgeojson.GeometryTypePoint,
			Coordinates:  []any{receiver.X, receiver.Y},
		})
	}

	return out
}

func heuristicRasterRowCenters(minY, maxY, resolutionM float64, rowCount int) []float64 {
	if rowCount <= 0 {
		return nil
	}

	centers := make([]float64, rowCount)
	if rowCount == 1 {
		centers[0] = (minY + maxY) / 2
		return centers
	}

	totalSpan := float64(rowCount-1) * resolutionM

	topCenter := maxY - ((maxY-minY)-totalSpan)/2
	for i := range centers {
		centers[i] = topCenter - float64(i)*resolutionM
	}

	return centers
}

func heuristicRowXPositions(left, right, resolutionM float64, count int) []float64 {
	xs := make([]float64, 0, count)
	if count <= 0 {
		return xs
	}

	if count == 1 {
		return append(xs, (left+right)/2)
	}

	totalSpan := float64(count-1) * resolutionM

	start := (left + right - totalSpan) / 2
	for i := range count {
		xs = append(xs, start+float64(i)*resolutionM)
	}

	return xs
}
