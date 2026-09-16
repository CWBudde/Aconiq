package cli

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/soundplanimport"
	"github.com/aconiq/backend/internal/report/results"
	"github.com/aconiq/backend/internal/standards/schall03"
)

const (
	defaultRasterCompareArtifactPath = ".noise/artifacts/soundplan-raster-compare.json"
	soundPlanRasterReceiverPrefix    = "soundplan-raster-"
	soundPlanRasterMetadataAlignment = "gm_origin_grid"

	// defaultGridMapReceiverHeightM is the height used when the imported
	// bundle did not record a grid-map height. It is SoundPLAN Essential's own
	// Rasterlärmkarte default, and it is a stated assumption rather than a
	// measurement: a bundle that carries RLKHEIGHT overrides it.
	defaultGridMapReceiverHeightM = 4.0
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

type soundPlanRasterCellComparisonRecord struct {
	ReceiverID     string  `json:"receiver_id"`
	Row            int     `json:"row"`
	Col            int     `json:"col"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	AconiqLrDay    float64 `json:"aconiq_lr_day"`
	SoundPlanDayDB float64 `json:"soundplan_day_db"`
	DeltaDayDB     float64 `json:"delta_day_db"`
	AconiqLrNight  float64 `json:"aconiq_lr_night"`
	SoundPlanNight float64 `json:"soundplan_night_db"`
	DeltaNightDB   float64 `json:"delta_night_db"`
}

type soundPlanRasterRunCompareSummary struct {
	ResultSubFolder   string                           `json:"result_subfolder"`
	Status            string                           `json:"status"`
	ComparedCellCount int                              `json:"compared_cell_count"`
	RowCount          int                              `json:"row_count"`
	MarkerCellCount   int                              `json:"marker_cell_count,omitempty"`
	ValueCellCount    int                              `json:"value_cell_count,omitempty"`
	Stats             map[string]compareIndicatorStats `json:"stats,omitempty"`
	Warnings          []string                         `json:"warnings,omitempty"`
}

type soundPlanRasterCompareArtifact struct {
	Status                  string   `json:"status"`
	Alignment               string   `json:"alignment,omitempty"`
	CalcAreaSource          string   `json:"calc_area_source,omitempty"`
	CalcAreaRole            string   `json:"calc_area_role,omitempty"`
	CalcAreaBoundsDelta     *float64 `json:"calc_area_bounds_delta,omitempty"`
	CalcAreaBoundsDeltaUnit string   `json:"calc_area_bounds_delta_unit,omitempty"`
	GridResolutionM         float64  `json:"grid_resolution_m,omitempty"`
	ReceiverHeightM         float64  `json:"receiver_height_m,omitempty"`
	SyntheticReceiverCount  int      `json:"synthetic_receiver_count,omitempty"`
	// The same three fields the report carries: which grid map the per-cell
	// deltas below belong to, out of which candidates, and on what grounds.
	// SoundPlanRuns stays every grid map the bundle holds.
	SoundPlanRasterRun           string                            `json:"soundplan_raster_run,omitempty"`
	SoundPlanRasterRunCandidates []string                          `json:"soundplan_raster_run_candidates,omitempty"`
	SoundPlanRasterRunSelection  string                            `json:"soundplan_raster_run_selection,omitempty"`
	SoundPlanRuns                []soundplanimport.GridMapMetadata `json:"soundplan_runs,omitempty"`
	Runs                         []soundPlanRasterRunCompareDetail `json:"runs,omitempty"`
	Warnings                     []string                          `json:"warnings,omitempty"`
}

type soundPlanRasterRunCompareDetail struct {
	ResultSubFolder   string                                `json:"result_subfolder"`
	Status            string                                `json:"status"`
	ComparedCellCount int                                   `json:"compared_cell_count"`
	RowCount          int                                   `json:"row_count"`
	MarkerCellCount   int                                   `json:"marker_cell_count,omitempty"`
	ValueCellCount    int                                   `json:"value_cell_count,omitempty"`
	Stats             map[string]compareIndicatorStats      `json:"stats,omitempty"`
	Records           []soundPlanRasterCellComparisonRecord `json:"records,omitempty"`
	Warnings          []string                              `json:"warnings,omitempty"`
}

type rasterComparePreparation struct {
	report               *soundPlanRasterCompareReport
	tempModelPath        string
	syntheticReceiverIDs []string
	soundPlanRuns        []soundplanimport.GridMapMetadata
	decodedRuns          []decodedGridMapRun
}

type decodedGridMapRun struct {
	metadata soundplanimport.GridMapMetadata
	decoded  soundplanimport.DecodedGridMap
}

// decodeGridMapRuns decodes every usable RRLK*.GM payload referenced by the
// import report. It returns the decoded runs plus one warning per skipped record.
func decodeGridMapRuns(soundPlanRoot string, gridMaps []soundplanimport.GridMapMetadata) ([]decodedGridMapRun, []string) {
	decodedRuns := make([]decodedGridMapRun, 0, len(gridMaps))
	warnings := make([]string, 0, len(gridMaps))

	for _, gm := range gridMaps {
		if strings.TrimSpace(gm.ResultSubFolder) == "" || strings.TrimSpace(gm.GMFile) == "" {
			warnings = append(warnings, "skipping incomplete grid-map metadata record")
			continue
		}

		gmPath := filepath.Join(soundPlanRoot, gm.ResultSubFolder, gm.GMFile)

		decoded, decodeErr := soundplanimport.ParseDecodedGridMap(gmPath, gm.PointsTotal)
		if decodeErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", gm.ResultSubFolder, decodeErr))
			continue
		}

		if decoded.ValueCellCount == 0 || len(decoded.Rows) == 0 {
			warnings = append(warnings, gm.ResultSubFolder+": decoded GM has no value cells")
			continue
		}

		decodedRuns = append(decodedRuns, decodedGridMapRun{
			metadata: gm,
			decoded:  decoded,
		})
	}

	return decodedRuns, warnings
}

// recordSelectedGridMapRun chooses the grid map the raster comparison is run
// against and writes the decision onto the report. It returns the selected
// result subfolder, or "" when no grid map named one at all — which is reported
// rather than left to look like a selection nobody made.
func recordSelectedGridMapRun(
	report *soundPlanRasterCompareReport,
	importReport soundPlanImportReport,
	baseModel modelgeojson.Model,
	explicitGridRun string,
) (string, error) {
	selection, err := selectSoundPlanGridMapRun(
		importReport.GridMaps, explicitGridRun, modelHasBarriers(baseModel), importReport.GridMapHeightM,
	)
	if err != nil {
		return "", err
	}

	report.SoundPlanRasterRun = selection.Dir
	report.SoundPlanRasterRunCandidates = selection.Candidates
	report.SoundPlanRasterRunSelection = selection.Selection
	report.Warnings = append(report.Warnings, selection.Warnings...)

	if selection.Dir == "" {
		report.Warnings = append(report.Warnings,
			"no SoundPLAN grid map names a result subfolder, so none could be selected for raster comparison")
	}

	return selection.Dir, nil
}

// selectedGridMaps narrows the discovered grid maps to the one run the
// comparison chose. It returns a slice so that decodeGridMapRuns keeps
// reporting a skipped record the same way it always has.
func selectedGridMaps(gridMaps []soundplanimport.GridMapMetadata, resultSubFolder string) []soundplanimport.GridMapMetadata {
	for _, gridMap := range gridMaps {
		if strings.TrimSpace(gridMap.ResultSubFolder) == resultSubFolder {
			return []soundplanimport.GridMapMetadata{gridMap}
		}
	}

	return nil
}

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

// prepareSoundPlanRasterCompare synthesizes the raster receivers for the one
// SoundPLAN grid map this comparison is run against.
//
// The model is loaded before the grid map is decoded, not after, because the
// selection depends on it: which grid map describes the scenario the model does
// is decided by whether the model carries barriers. Decoding then happens once,
// for the selected run only — which also retires the assumption that the first
// decoded run's grid layout could stand for all of them.
func prepareSoundPlanRasterCompare(
	projectRoot string,
	importReport soundPlanImportReport,
	modelPath string,
	explicitGridRun string,
) (*rasterComparePreparation, bool, error) {
	if len(importReport.GridMaps) == 0 {
		return nil, false, nil
	}

	report := &soundPlanRasterCompareReport{
		Status:          "parsed_values_unaligned",
		GridResolutionM: importReport.GridResolutionM,
		SoundPlanRuns:   append([]soundplanimport.GridMapMetadata(nil), importReport.GridMaps...),
		Warnings: []string{
			"RRLK*.GM values are decoded, but raster comparison is using a heuristic scanline alignment until SoundPLAN origin metadata is fully decoded.",
		},
	}

	soundPlanRoot := resolvePath(projectRoot, importReport.SourcePath)
	baseModelPath := resolvePath(projectRoot, modelPath)

	baseModel, err := loadValidatedModel(baseModelPath, importReport.ProjectCRS, relativePath(projectRoot, baseModelPath))
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("load normalized model for raster compare: %v", err))
		return &rasterComparePreparation{report: report}, true, nil
	}

	selectedRun, err := recordSelectedGridMapRun(report, importReport, baseModel, explicitGridRun)
	if err != nil {
		return nil, false, err
	}

	if selectedRun == "" {
		return &rasterComparePreparation{report: report}, true, nil
	}

	decodedRuns, decodeWarnings := decodeGridMapRuns(soundPlanRoot, selectedGridMaps(importReport.GridMaps, selectedRun))
	report.Warnings = append(report.Warnings, decodeWarnings...)

	if len(decodedRuns) == 0 {
		report.Warnings = append(report.Warnings, "no decodable GM payload was available for raster comparison")
		return &rasterComparePreparation{report: report}, true, nil
	}

	// One run by construction, and it is the selected one — so the grid layout
	// below belongs to the grid map actually being compared.
	layoutRows := decodedRuns[0].decoded.Rows

	calcArea := resolveRasterCalcArea(baseModel, importReport)
	report.CalcAreaSource = calcArea.source
	report.CalcAreaBoundsDelta = calcArea.boundsDelta
	report.CalcAreaBoundsDeltaUnit = calcArea.boundsDeltaUnit
	report.Warnings = append(report.Warnings, calcArea.warnings...)

	syntheticReceiverHeight := syntheticRasterReceiverHeight(importReport)

	syntheticReceivers, ids, synthesized := synthesizeRasterReceivers(
		report, importReport, decodedRuns[0].metadata, calcArea.area, syntheticReceiverHeight, layoutRows,
	)
	if !synthesized {
		return &rasterComparePreparation{report: report}, true, nil
	}

	tempModel := appendSyntheticRasterReceivers(baseModel, syntheticReceivers)

	tempModelPath := filepath.Join(projectRoot, ".noise", "tmp", "soundplan-raster-compare-model.geojson")
	if err := writeJSONFile(tempModelPath, tempModel.ToFeatureCollection()); err != nil {
		return nil, false, err
	}

	report.Status = "heuristic_scanline_compare"
	if report.Alignment == "" {
		report.Alignment = "calcarea_scanlines_centered"
	}

	report.ReceiverHeightM = syntheticReceivers[0].HeightM
	report.SyntheticReceiverCount = len(ids)

	return &rasterComparePreparation{
		report:               report,
		tempModelPath:        tempModelPath,
		syntheticReceiverIDs: ids,
		soundPlanRuns:        append([]soundplanimport.GridMapMetadata(nil), importReport.GridMaps...),
		decodedRuns:          decodedRuns,
	}, true, nil
}

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
		// Its absence is never "no area". The property survives only until the
		// first save from the map: frontend/src/model/to-geojson.ts emits
		// properties: { kind } and nothing else.
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

// compareDecodedGridMapRun compares one decoded SoundPLAN grid map against the
// synthetic raster receivers computed by the Aconiq run, in scanline order.
func compareDecodedGridMapRun(
	run decodedGridMapRun,
	syntheticReceiverIDs []string,
	recordByID map[string]results.ReceiverRecord,
	toleranceDB float64,
) (soundPlanRasterRunCompareDetail, error) {
	detail := soundPlanRasterRunCompareDetail{
		ResultSubFolder:   run.metadata.ResultSubFolder,
		Status:            "compared",
		RowCount:          len(run.decoded.Rows),
		MarkerCellCount:   run.decoded.MarkerCellCount,
		ValueCellCount:    run.decoded.ValueCellCount,
		ComparedCellCount: 0,
		Stats:             map[string]compareIndicatorStats{},
	}

	dayAbs := make([]float64, 0, run.decoded.ValueCellCount)
	nightAbs := make([]float64, 0, run.decoded.ValueCellCount)
	cellIndex := 0

	for rowIndex, row := range run.decoded.Rows {
		for colIndex, cell := range row {
			if cellIndex >= len(syntheticReceiverIDs) {
				detail.Status = "partial_compare"
				detail.Warnings = append(detail.Warnings, "Aconiq raster receiver sequence is shorter than decoded SoundPLAN cells")

				break
			}

			receiverID := syntheticReceiverIDs[cellIndex]

			record, ok := recordByID[receiverID]
			if !ok {
				detail.Status = "partial_compare"
				detail.Warnings = append(detail.Warnings, "missing raster receiver output for "+receiverID)
				cellIndex++

				continue
			}

			dayValue, ok := record.Values[schall03.IndicatorLrDay]
			if !ok {
				return detail, fmt.Errorf("raster receiver table missing %s", schall03.IndicatorLrDay)
			}

			nightValue, ok := record.Values[schall03.IndicatorLrNight]
			if !ok {
				return detail, fmt.Errorf("raster receiver table missing %s", schall03.IndicatorLrNight)
			}

			dayDelta := dayValue - cell.DayDB
			nightDelta := nightValue - cell.NightDB
			detail.Records = append(detail.Records, soundPlanRasterCellComparisonRecord{
				ReceiverID:     receiverID,
				Row:            rowIndex,
				Col:            colIndex,
				X:              record.X,
				Y:              record.Y,
				AconiqLrDay:    dayValue,
				SoundPlanDayDB: cell.DayDB,
				DeltaDayDB:     dayDelta,
				AconiqLrNight:  nightValue,
				SoundPlanNight: cell.NightDB,
				DeltaNightDB:   nightDelta,
			})

			dayAbs = append(dayAbs, math.Abs(dayDelta))
			nightAbs = append(nightAbs, math.Abs(nightDelta))
			detail.ComparedCellCount++
			cellIndex++
		}
	}

	if detail.ComparedCellCount == 0 {
		detail.Status = "decoded_but_unmatched"
		detail.Warnings = append(detail.Warnings, "no raster cells could be matched to Aconiq receiver outputs")
	}

	detail.Stats[schall03.IndicatorLrDay] = buildCompareIndicatorStats(dayAbs, toleranceDB)
	detail.Stats[schall03.IndicatorLrNight] = buildCompareIndicatorStats(nightAbs, toleranceDB)

	return detail, nil
}

func finalizeSoundPlanRasterCompare(
	projectRoot string,
	prep *rasterComparePreparation,
	table results.ReceiverTable,
	toleranceDB float64,
) (*soundPlanRasterCompareReport, *soundPlanRasterCompareArtifact, error) {
	if prep == nil || prep.report == nil {
		return nil, nil, nil
	}

	if prep.tempModelPath == "" || len(prep.syntheticReceiverIDs) == 0 || len(prep.decodedRuns) == 0 {
		return prep.report, nil, nil
	}

	recordByID := make(map[string]results.ReceiverRecord, len(table.Records))
	for _, record := range table.Records {
		if strings.HasPrefix(record.ID, soundPlanRasterReceiverPrefix) {
			recordByID[record.ID] = record
		}
	}

	artifact := &soundPlanRasterCompareArtifact{
		Status:                  prep.report.Status,
		Alignment:               prep.report.Alignment,
		CalcAreaSource:          prep.report.CalcAreaSource,
		CalcAreaRole:            prep.report.CalcAreaRole,
		CalcAreaBoundsDelta:     prep.report.CalcAreaBoundsDelta,
		CalcAreaBoundsDeltaUnit: prep.report.CalcAreaBoundsDeltaUnit,
		GridResolutionM:         prep.report.GridResolutionM,
		ReceiverHeightM:         prep.report.ReceiverHeightM,
		SyntheticReceiverCount:  len(prep.syntheticReceiverIDs),

		SoundPlanRasterRun:           prep.report.SoundPlanRasterRun,
		SoundPlanRasterRunCandidates: append([]string(nil), prep.report.SoundPlanRasterRunCandidates...),
		SoundPlanRasterRunSelection:  prep.report.SoundPlanRasterRunSelection,
		SoundPlanRuns:                append([]soundplanimport.GridMapMetadata(nil), prep.soundPlanRuns...),
		Warnings:                     append([]string(nil), prep.report.Warnings...),
	}

	prep.report.Runs = make([]soundPlanRasterRunCompareSummary, 0, len(prep.decodedRuns))

	for _, run := range prep.decodedRuns {
		detail, err := compareDecodedGridMapRun(run, prep.syntheticReceiverIDs, recordByID, toleranceDB)
		if err != nil {
			return prep.report, nil, err
		}

		artifact.Runs = append(artifact.Runs, detail)
		prep.report.Runs = append(prep.report.Runs, soundPlanRasterRunCompareSummary{
			ResultSubFolder:   detail.ResultSubFolder,
			Status:            detail.Status,
			ComparedCellCount: detail.ComparedCellCount,
			RowCount:          detail.RowCount,
			MarkerCellCount:   detail.MarkerCellCount,
			ValueCellCount:    detail.ValueCellCount,
			Stats:             detail.Stats,
			Warnings:          append([]string(nil), detail.Warnings...),
		})
	}

	if err := writeJSONFile(filepath.Join(projectRoot, filepath.FromSlash(defaultRasterCompareArtifactPath)), artifact); err != nil {
		return prep.report, nil, err
	}

	prep.report.ArtifactPath = defaultRasterCompareArtifactPath

	return prep.report, artifact, nil
}

func cleanupRasterComparePreparation(prep *rasterComparePreparation) {
	if prep == nil || strings.TrimSpace(prep.tempModelPath) == "" {
		return
	}

	_ = os.Remove(prep.tempModelPath)
}

func filterOutSyntheticRasterReceivers(table results.ReceiverTable) results.ReceiverTable {
	filtered := results.ReceiverTable{
		IndicatorOrder: append([]string(nil), table.IndicatorOrder...),
		Unit:           table.Unit,
		Records:        make([]results.ReceiverRecord, 0, len(table.Records)),
	}

	for _, record := range table.Records {
		if strings.HasPrefix(record.ID, soundPlanRasterReceiverPrefix) {
			continue
		}

		filtered.Records = append(filtered.Records, record)
	}

	return filtered
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

func minYFromArea(area *soundplanimport.CalcArea) float64 {
	return calcAreaBounds(area).MinY
}

func maxYFromArea(area *soundplanimport.CalcArea) float64 {
	return calcAreaBounds(area).MaxY
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

func countGridMapCells(rows [][]soundplanimport.GridMapCell) int {
	total := 0
	for _, row := range rows {
		total += len(row)
	}

	return total
}

func uniqueStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(items))

	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		if _, ok := seen[item]; ok {
			continue
		}

		seen[item] = struct{}{}
		out = append(out, item)
	}

	return out
}
