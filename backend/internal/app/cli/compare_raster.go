package cli

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/soundplanimport"
	"github.com/aconiq/backend/internal/report/results"
	"github.com/aconiq/backend/internal/standards/schall03"
)

const (
	defaultRasterCompareArtifactPath = ".noise/artifacts/soundplan-raster-compare.json"
	soundPlanRasterReceiverPrefix    = "soundplan-raster-"
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
	Status                 string                            `json:"status"`
	Alignment              string                            `json:"alignment,omitempty"`
	GridResolutionM        float64                           `json:"grid_resolution_m,omitempty"`
	ReceiverHeightM        float64                           `json:"receiver_height_m,omitempty"`
	SyntheticReceiverCount int                               `json:"synthetic_receiver_count,omitempty"`
	SoundPlanRuns          []soundplanimport.GridMapMetadata `json:"soundplan_runs,omitempty"`
	Runs                   []soundPlanRasterRunCompareDetail `json:"runs,omitempty"`
	Warnings               []string                          `json:"warnings,omitempty"`
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

func prepareSoundPlanRasterCompare(projectRoot string, importReport soundPlanImportReport, modelPath string) (*rasterComparePreparation, error) {
	if len(importReport.GridMaps) == 0 {
		return nil, nil
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
	bundle, err := soundplanimport.LoadProjectBundle(soundPlanRoot)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("raster compare preparation failed while loading SoundPLAN bundle: %v", err))
		return &rasterComparePreparation{report: report}, nil
	}

	if bundle.CalcArea == nil || len(bundle.CalcArea.Points) < 4 {
		report.Warnings = append(report.Warnings, "CalcArea.geo is missing or incomplete, so raster scanline receivers could not be synthesized")
		return &rasterComparePreparation{report: report}, nil
	}

	gridResolutionM := importReport.GridResolutionM
	if gridResolutionM <= 0 && bundle.Project != nil {
		gridResolutionM = bundle.Project.Settings.GridMapDistance
	}
	if gridResolutionM <= 0 {
		report.Warnings = append(report.Warnings, "grid resolution is unavailable, so raster scanline receivers could not be synthesized")
		return &rasterComparePreparation{report: report}, nil
	}

	decodedRuns := make([]decodedGridMapRun, 0, len(bundle.GridMaps))
	for _, gm := range bundle.GridMaps {
		gmPath := filepath.Join(soundPlanRoot, gm.ResultSubFolder, gm.GMFile)
		decoded, decodeErr := soundplanimport.ParseDecodedGridMap(gmPath, gm.PointsTotal)
		if decodeErr != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s: %v", gm.ResultSubFolder, decodeErr))
			continue
		}
		if decoded.ValueCellCount == 0 || len(decoded.Rows) == 0 {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s: decoded GM has no value cells", gm.ResultSubFolder))
			continue
		}

		decodedRuns = append(decodedRuns, decodedGridMapRun{
			metadata: gm,
			decoded:  decoded,
		})
	}

	if len(decodedRuns) == 0 {
		report.Warnings = append(report.Warnings, "no decodable GM payload was available for raster comparison")
		return &rasterComparePreparation{report: report}, nil
	}

	layoutRows := decodedRuns[0].decoded.Rows
	syntheticReceivers, ids, synthWarnings := buildHeuristicRasterReceivers(bundle.CalcArea, gridResolutionM, derivedReceiverHeight(bundle.Project), layoutRows)
	report.Warnings = append(report.Warnings, synthWarnings...)
	if len(syntheticReceivers) == 0 {
		report.Warnings = append(report.Warnings, "heuristic raster receiver synthesis produced no receivers")
		return &rasterComparePreparation{report: report}, nil
	}

	baseModelPath := resolvePath(projectRoot, modelPath)
	baseModel, err := loadValidatedModel(baseModelPath, importReport.ProjectCRS, relativePath(projectRoot, baseModelPath))
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("load normalized model for raster compare: %v", err))
		return &rasterComparePreparation{report: report}, nil
	}

	tempModel, err := appendSyntheticRasterReceivers(baseModel, syntheticReceivers)
	if err != nil {
		return nil, err
	}

	tempModelPath := filepath.Join(projectRoot, ".noise", "tmp", "soundplan-raster-compare-model.geojson")
	if err := writeJSONFile(tempModelPath, tempModel.ToFeatureCollection()); err != nil {
		return nil, err
	}

	report.Status = "heuristic_scanline_compare"
	report.Alignment = "calcarea_scanlines_centered"
	report.ReceiverHeightM = syntheticReceivers[0].HeightM
	report.SyntheticReceiverCount = len(ids)

	return &rasterComparePreparation{
		report:               report,
		tempModelPath:        tempModelPath,
		syntheticReceiverIDs: ids,
		soundPlanRuns:        append([]soundplanimport.GridMapMetadata(nil), bundle.GridMaps...),
		decodedRuns:          decodedRuns,
	}, nil
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
		Status:                 prep.report.Status,
		Alignment:              prep.report.Alignment,
		GridResolutionM:        prep.report.GridResolutionM,
		ReceiverHeightM:        prep.report.ReceiverHeightM,
		SyntheticReceiverCount: len(prep.syntheticReceiverIDs),
		SoundPlanRuns:          append([]soundplanimport.GridMapMetadata(nil), prep.soundPlanRuns...),
		Warnings:               append([]string(nil), prep.report.Warnings...),
	}

	prep.report.Runs = make([]soundPlanRasterRunCompareSummary, 0, len(prep.decodedRuns))

	for _, run := range prep.decodedRuns {
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
				if cellIndex >= len(prep.syntheticReceiverIDs) {
					detail.Status = "partial_compare"
					detail.Warnings = append(detail.Warnings, "Aconiq raster receiver sequence is shorter than decoded SoundPLAN cells")
					break
				}

				receiverID := prep.syntheticReceiverIDs[cellIndex]
				record, ok := recordByID[receiverID]
				if !ok {
					detail.Status = "partial_compare"
					detail.Warnings = append(detail.Warnings, fmt.Sprintf("missing raster receiver output for %s", receiverID))
					cellIndex++
					continue
				}

				dayValue, ok := record.Values[schall03.IndicatorLrDay]
				if !ok {
					return prep.report, nil, fmt.Errorf("raster receiver table missing %s", schall03.IndicatorLrDay)
				}

				nightValue, ok := record.Values[schall03.IndicatorLrNight]
				if !ok {
					return prep.report, nil, fmt.Errorf("raster receiver table missing %s", schall03.IndicatorLrNight)
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

	minX, minY, maxX, maxY := calcAreaBounds(area)
	yPositions := heuristicRasterRowCenters(minY, maxY, gridResolutionM, len(rows))

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
			left = minX
			right = maxX
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

func appendSyntheticRasterReceivers(model modelgeojson.Model, receivers []heuristicRasterReceiver) (modelgeojson.Model, error) {
	out := model
	out.Features = make([]modelgeojson.Feature, 0, len(model.Features)+len(receivers))

	for _, feature := range model.Features {
		if feature.Kind == "receiver" && strings.HasPrefix(feature.ID, soundPlanRasterReceiverPrefix) {
			continue
		}

		out.Features = append(out.Features, feature)
	}

	for _, receiver := range receivers {
		out.Features = append(out.Features, modelgeojson.Feature{
			ID:      receiver.ID,
			Kind:    "receiver",
			HeightM: float64Ptr(receiver.HeightM),
			Properties: map[string]any{
				"soundplan_raster_compare": true,
				"soundplan_raster_row":     receiver.Row,
				"soundplan_raster_col":     receiver.Col,
			},
			GeometryType: "Point",
			Coordinates:  []any{receiver.X, receiver.Y},
		})
	}

	return out, nil
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
	for i := 0; i < count; i++ {
		xs = append(xs, start+float64(i)*resolutionM)
	}

	return xs
}

func calcAreaBounds(area *soundplanimport.CalcArea) (float64, float64, float64, float64) {
	minX := area.Points[0].X
	minY := area.Points[0].Y
	maxX := area.Points[0].X
	maxY := area.Points[0].Y

	for _, point := range area.Points[1:] {
		minX = math.Min(minX, point.X)
		minY = math.Min(minY, point.Y)
		maxX = math.Max(maxX, point.X)
		maxY = math.Max(maxY, point.Y)
	}

	return minX, minY, maxX, maxY
}

func calcAreaHorizontalSpan(area *soundplanimport.CalcArea, y float64) (float64, float64, bool) {
	if area == nil || len(area.Points) < 4 {
		return 0, 0, false
	}

	intersections := make([]float64, 0, len(area.Points))
	for i := 0; i < len(area.Points)-1; i++ {
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
