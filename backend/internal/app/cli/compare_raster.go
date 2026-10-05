package cli

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/soundplanimport"
	"github.com/aconiq/backend/internal/results"
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

	syntheticReceiverHeight, heightWarnings := selectedRunReceiverHeight(decodedRuns[0].metadata, importReport)
	report.Warnings = append(report.Warnings, heightWarnings...)

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
	// Records are dropped here, never indicators, so the units come across
	// whole. Copied rather than shared, for the same reason IndicatorOrder is.
	filtered := results.ReceiverTable{
		IndicatorOrder: append([]string(nil), table.IndicatorOrder...),
		Units:          results.CopyUnits(table.Units),
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
