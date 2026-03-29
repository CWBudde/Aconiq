package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/io/soundplanimport"
	"github.com/aconiq/backend/internal/standards/schall03"
	"github.com/spf13/cobra"
)

const (
	defaultSoundPlanBuildingHeightM = 8.0
	defaultSoundPlanCurveRadiusM    = 500.0
	defaultSoundPlanTrafficDayPH    = 8.0
	defaultSoundPlanTrafficNightPH  = 4.0
)

type soundPlanImportReport struct {
	Format           string                             `json:"format"`
	ProjectTitle     string                             `json:"project_title"`
	ProjectVersion   int                                `json:"project_version"`
	ProjectV64       bool                               `json:"project_v64"`
	SourcePath       string                             `json:"source_path"`
	ProjectCRS       string                             `json:"project_crs"`
	AssumedImportCRS string                             `json:"assumed_import_crs"`
	RunCount         int                                `json:"run_count"`
	CountsByKind     map[string]int                     `json:"counts_by_kind"`
	TerrainSource    string                             `json:"terrain_source,omitempty"`
	StandardMappings []soundplanimport.StandardMapping  `json:"standard_mappings"`
	Warnings         []string                           `json:"warnings,omitempty"`
	Decisions        []string                           `json:"decisions,omitempty"`
	Assessment       []soundplanimport.AssessmentPeriod `json:"assessment_periods,omitempty"`
	ResultRuns       []soundPlanImportRunSummary        `json:"result_runs,omitempty"`
}

type soundPlanImportRunSummary struct {
	RunType         string   `json:"run_type,omitempty"`
	ResultSubFolder string   `json:"result_subfolder,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
}

func runSoundPlanImport(
	cmd *cobra.Command,
	state commandState,
	store projectfs.Store,
	proj *project.Project,
	soundPlanPath string,
	normalizedPath string,
	dumpPath string,
	reportPath string,
	importReportPath string,
) error {
	absoluteInput := resolvePath(store.Root(), soundPlanPath)
	relInput := relativePath(store.Root(), absoluteInput)

	bundle, err := soundplanimport.LoadProjectBundle(absoluteInput)
	if err != nil {
		return fmt.Errorf("load soundplan project: %w", err)
	}

	model, importReport, err := buildSoundPlanModelAndReport(bundle, proj.CRS, relInput)
	if err != nil {
		return err
	}

	report := modelgeojson.Validate(model)
	if report.ErrorCount() > 0 {
		messages := make([]string, 0, len(report.Errors))
		for _, issue := range report.Errors {
			messages = append(messages, issue.Code+": "+issue.Message)
		}

		return fmt.Errorf("soundplan import produced invalid model: %s", summarizeValidationErrors(messages, 5))
	}

	err = persistSoundPlanArtifacts(store, proj, model, report, importReport, normalizedPath, dumpPath, reportPath, importReportPath)
	if err != nil {
		return err
	}

	state.Logger.Info(
		"soundplan import completed",
		"input", relInput,
		"feature_count", len(model.Features),
		"warnings", len(importReport.Warnings),
		"normalized", relativePath(store.Root(), normalizedPath),
	)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Imported %d features from SoundPLAN project %s\n", len(model.Features), relInput)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Normalized GeoJSON: %s\n", relativePath(store.Root(), normalizedPath))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Model dump: %s\n", relativePath(store.Root(), dumpPath))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Validation report: %s\n", relativePath(store.Root(), reportPath))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "SoundPLAN import report: %s\n", relativePath(store.Root(), importReportPath))

	if len(importReport.Warnings) > 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Import warnings: %d\n", len(importReport.Warnings))
	}

	return nil
}

func buildSoundPlanModelAndReport(bundle *soundplanimport.ProjectBundle, projectCRS string, sourcePath string) (modelgeojson.Model, soundPlanImportReport, error) {
	features := make([]modelgeojson.Feature, 0, 512)
	counts := map[string]int{
		"source":   0,
		"building": 0,
		"barrier":  0,
		"receiver": 0,
	}

	warnings := append([]string(nil), bundle.Warnings...)
	decisions := []string{
		"SoundPLAN coordinates are imported without reprojection and assumed to already match the project CRS.",
		"Rail defaults are exported explicitly so the normalized model can enter the existing Schall 03 path before full train-operation mapping exists.",
	}

	buildingHeightM := derivedBuildingHeight(bundle.Project)
	receiverHeightM := derivedReceiverHeight(bundle.Project)
	terrainSource := ""

	if bundle.Terrain != nil {
		switch {
		case len(bundle.Terrain.ContourLines) > 0:
			terrainSource = "GeoTmp.geo"
		case len(bundle.Terrain.ElevationPoints) > 0:
			terrainSource = "Höhen.txt"
		}
	}

	if len(bundle.Barriers) > 0 {
		warnings = append(warnings, "barrier height_m is collapsed to one scalar per barrier using the maximum point height; per-vertex height variation is preserved only in SoundPLAN-specific properties")
	}

	if len(bundle.RailTracks) > 0 {
		if len(bundle.RailOps) == 0 {
			warnings = append(warnings, "rail traffic and train class could not be derived from SoundPLAN RRAD/RRAI tables; importer fell back to explicit placeholders")
		}

		warnings = append(warnings, "rail traction, track form, and roughness still use explicit placeholders until deeper SoundPLAN parameter mapping is implemented")
	}

	railOpsByName := make(map[string][]soundplanimport.RailOperationSummary)
	for _, summary := range bundle.RailOps {
		railOpsByName[strings.TrimSpace(summary.Railname)] = append(railOpsByName[strings.TrimSpace(summary.Railname)], summary)
	}

	for trackIndex, track := range bundle.RailTracks {
		opsForTrack := railOpsByName[strings.TrimSpace(track.Name)]
		trackSummary := aggregateRailSummaries(opsForTrack)

		for segmentIndex, segment := range track.Segments {
			if len(segment.Points) < 2 {
				warnings = append(warnings, fmt.Sprintf("rail track %q segment %d skipped because it has fewer than 2 points", track.Name, segmentIndex+1))
				continue
			}

			id := fmt.Sprintf("soundplan-rail-%02d-%02d", trackIndex+1, segmentIndex+1)
			speedKPH := trackSummary.AverageSpeedKPH
			if speedKPH <= 0 {
				speedKPH = segment.Params.Speed
			}
			if speedKPH <= 0 {
				speedKPH = 100
			}

			coords := make([]any, 0, len(segment.Points))
			for _, point := range segment.Points {
				coords = append(coords, []any{point.X, point.Y})
			}

			properties := map[string]any{
				"soundplan_track_name":           strings.TrimSpace(track.Name),
				"soundplan_segment_index":        segmentIndex + 1,
				"soundplan_bridge_correction_db": segment.Params.BridgeCorrection,
				"soundplan_track_height_m":       segment.Params.TrackHeight,
				"elevation_m":                    segment.Points[0].ZTrack,
				"rail_train_class":               coalesceString(trackSummary.TrainClass, schall03.TrainClassMixed),
				"rail_traction_type":             schall03.TractionElectric,
				"rail_track_type":                schall03.TrackTypeBallasted,
				"rail_track_form":                schall03.TrackFormMainline,
				"rail_track_roughness_class":     schall03.RoughnessStandard,
				"rail_average_train_speed_kph":   speedKPH,
				"rail_curve_radius_m":            defaultSoundPlanCurveRadiusM,
				"rail_on_bridge":                 trackSummary.OnBridge || segment.Params.BridgeCorrection > -999.0,
				"traffic_day_trains_per_hour":    coalescePositive(trackSummary.TrafficDayPH, defaultSoundPlanTrafficDayPH),
				"traffic_night_trains_per_hour":  coalescePositive(trackSummary.TrafficNightPH, defaultSoundPlanTrafficNightPH),
				"soundplan_placeholder_mapping":  len(opsForTrack) == 0,
				"soundplan_dominant_train_name":  trackSummary.DominantTrainName,
				"soundplan_train_names":          trackSummary.TrainNames,
				"soundplan_day_train_count":      trackSummary.DayTrainCount,
				"soundplan_night_train_count":    trackSummary.NightTrainCount,
				"soundplan_track_vmax_kph":       trackSummary.TrackVMaxKPH,
				"soundplan_assessment_day_hours": trackSummary.AssessmentDayHours,
				"soundplan_assessment_night_h":   trackSummary.AssessmentNightHrs,
			}

			features = append(features, modelgeojson.Feature{
				ID:           id,
				Kind:         "source",
				SourceType:   "line",
				Properties:   properties,
				GeometryType: "LineString",
				Coordinates:  coords,
			})
			counts["source"]++
		}
	}

	if bundle.GeoObjects != nil {
		if len(bundle.GeoObjects.Buildings) > 0 {
			warnings = append(warnings, fmt.Sprintf("building heights are not yet available from GeoObjs.geo attributes; imported %d buildings with derived default height %.2f m", len(bundle.GeoObjects.Buildings), buildingHeightM))
		}

		if len(bundle.GeoObjects.Receivers) > 0 {
			warnings = append(warnings, fmt.Sprintf("receiver heights are not encoded per receiver in the current parser; imported %d receivers with project default height %.2f m", len(bundle.GeoObjects.Receivers), receiverHeightM))
		}

		for buildingIndex, building := range bundle.GeoObjects.Buildings {
			if len(building.Footprint) < 4 {
				warnings = append(warnings, fmt.Sprintf("building %d skipped because footprint has fewer than 4 points", buildingIndex+1))
				continue
			}

			features = append(features, modelgeojson.Feature{
				ID:           fmt.Sprintf("soundplan-building-%04d", buildingIndex+1),
				Kind:         "building",
				HeightM:      float64Ptr(buildingHeightM),
				Properties:   map[string]any{"soundplan_placeholder_height": true, "soundplan_base_elevation_m": building.Footprint[0].Z},
				GeometryType: "Polygon",
				Coordinates:  []any{points3DToRing(building.Footprint)},
			})
			counts["building"]++
		}

		for receiverIndex, receiver := range bundle.GeoObjects.Receivers {
			features = append(features, modelgeojson.Feature{
				ID:           fmt.Sprintf("soundplan-receiver-%04d", receiverIndex+1),
				Kind:         "receiver",
				HeightM:      float64Ptr(receiverHeightM),
				Properties:   map[string]any{"soundplan_z_m": receiver.Z},
				GeometryType: "Point",
				Coordinates:  []any{receiver.X, receiver.Y},
			})
			counts["receiver"]++
		}
	}

	for barrierIndex, barrier := range bundle.Barriers {
		if len(barrier.Points) < 2 {
			warnings = append(warnings, fmt.Sprintf("barrier %d skipped because it has fewer than 2 points", barrierIndex+1))
			continue
		}

		coords := make([]any, 0, len(barrier.Points))
		maxHeight := barrier.Points[0].Height
		topHeights := make([]float64, 0, len(barrier.Points))

		for _, point := range barrier.Points {
			coords = append(coords, []any{point.X, point.Y})
			topHeights = append(topHeights, point.ZTop)
			if point.Height > maxHeight {
				maxHeight = point.Height
			}
		}

		features = append(features, modelgeojson.Feature{
			ID:      fmt.Sprintf("soundplan-barrier-%03d", barrierIndex+1),
			Kind:    "barrier",
			HeightM: float64Ptr(maxHeight),
			Properties: map[string]any{
				"soundplan_height_profile_m": topHeights,
				"soundplan_variable_height":  true,
			},
			GeometryType: "LineString",
			Coordinates:  coords,
		})
		counts["barrier"]++
	}

	reportRuns := make([]soundPlanImportRunSummary, 0, len(bundle.Runs))
	for _, run := range bundle.Runs {
		reportRuns = append(reportRuns, soundPlanImportRunSummary{
			RunType:         run.RunType,
			ResultSubFolder: run.ResultSubFolder,
			Warnings:        append([]string(nil), run.Warnings...),
		})
	}

	slices.Sort(warnings)

	report := soundPlanImportReport{
		Format:           "soundplan",
		ProjectTitle:     bundle.Project.Title,
		ProjectVersion:   bundle.Project.Version,
		ProjectV64:       bundle.Project.V64,
		SourcePath:       sourcePath,
		ProjectCRS:       projectCRS,
		AssumedImportCRS: projectCRS,
		RunCount:         len(bundle.Runs),
		CountsByKind:     counts,
		TerrainSource:    terrainSource,
		StandardMappings: append([]soundplanimport.StandardMapping(nil), bundle.Standards...),
		Warnings:         warnings,
		Decisions:        decisions,
		Assessment:       append([]soundplanimport.AssessmentPeriod(nil), bundle.Project.AssessmentPeriods...),
		ResultRuns:       reportRuns,
	}

	model := modelgeojson.Model{
		SchemaVersion: 1,
		ProjectCRS:    projectCRS,
		ImportedAt:    nowUTC(),
		SourcePath:    sourcePath,
		Features:      features,
	}

	return model, report, nil
}

func aggregateRailSummaries(items []soundplanimport.RailOperationSummary) soundplanimport.RailOperationSummary {
	if len(items) == 0 {
		return soundplanimport.RailOperationSummary{}
	}

	out := soundplanimport.RailOperationSummary{
		Railname:           items[0].Railname,
		AssessmentDayHours: items[0].AssessmentDayHours,
		AssessmentNightHrs: items[0].AssessmentNightHrs,
	}

	classSeen := make(map[string]struct{})
	nameSeen := make(map[string]struct{})
	dominantWeight := -1.0
	speedWeight := 0.0

	for _, item := range items {
		out.DayTrainCount += item.DayTrainCount
		out.NightTrainCount += item.NightTrainCount
		out.TrafficDayPH += item.TrafficDayPH
		out.TrafficNightPH += item.TrafficNightPH
		out.OnBridge = out.OnBridge || item.OnBridge
		if item.TrackVMaxKPH > out.TrackVMaxKPH {
			out.TrackVMaxKPH = item.TrackVMaxKPH
		}

		weight := item.DayTrainCount + item.NightTrainCount
		if weight > 0 && item.AverageSpeedKPH > 0 {
			out.AverageSpeedKPH += item.AverageSpeedKPH * weight
			speedWeight += weight
		}

		if item.TrainClass != "" {
			classSeen[item.TrainClass] = struct{}{}
		}

		for _, name := range item.TrainNames {
			if _, ok := nameSeen[name]; ok || strings.TrimSpace(name) == "" {
				continue
			}

			nameSeen[name] = struct{}{}
			out.TrainNames = append(out.TrainNames, name)
		}

		if weight > dominantWeight && strings.TrimSpace(item.DominantTrainName) != "" {
			dominantWeight = weight
			out.DominantTrainName = item.DominantTrainName
		}
	}

	if speedWeight > 0 {
		out.AverageSpeedKPH /= speedWeight
	}

	switch len(classSeen) {
	case 0:
		out.TrainClass = ""
	case 1:
		for class := range classSeen {
			out.TrainClass = class
		}
	default:
		out.TrainClass = schall03.TrainClassMixed
	}

	slices.Sort(out.TrainNames)

	return out
}

func coalescePositive(value float64, fallback float64) float64 {
	if value > 0 {
		return value
	}

	return fallback
}

func coalesceString(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}

	return fallback
}

func persistSoundPlanArtifacts(
	store projectfs.Store,
	proj *project.Project,
	model modelgeojson.Model,
	report modelgeojson.ValidationReport,
	importReport soundPlanImportReport,
	normalizedPath string,
	dumpPath string,
	reportPath string,
	importReportPath string,
) error {
	if err := writeJSONFile(normalizedPath, model.ToFeatureCollection()); err != nil {
		return err
	}

	if err := writeJSONFile(dumpPath, model.ToDump()); err != nil {
		return err
	}

	if err := writeJSONFile(reportPath, report); err != nil {
		return err
	}

	if err := writeJSONFile(importReportPath, importReport); err != nil {
		return err
	}

	now := nowUTC()
	for _, ref := range []project.ArtifactRef{
		{ID: "artifact-model-normalized", Kind: "model.normalized_geojson", Path: relativePath(store.Root(), normalizedPath), CreatedAt: now},
		{ID: "artifact-model-dump", Kind: "model.dump_json", Path: relativePath(store.Root(), dumpPath), CreatedAt: now},
		{ID: "artifact-model-validation", Kind: "model.validation_report", Path: relativePath(store.Root(), reportPath), CreatedAt: now},
		{ID: "artifact-soundplan-import-report", Kind: "model.soundplan_import_report", Path: relativePath(store.Root(), importReportPath), CreatedAt: now},
	} {
		proj.Artifacts = upsertArtifact(proj.Artifacts, ref)
	}

	return store.Save(*proj)
}

func derivedBuildingHeight(proj *soundplanimport.Project) float64 {
	if proj == nil {
		return defaultSoundPlanBuildingHeightM
	}

	if proj.Settings.FloorCount > 0 && proj.Settings.FloorHeight > 0 {
		return float64(proj.Settings.FloorCount) * proj.Settings.FloorHeight
	}

	if proj.GeoDB.FloorHeight > 0 && proj.Settings.FloorCount > 0 {
		return float64(proj.Settings.FloorCount) * proj.GeoDB.FloorHeight
	}

	return defaultSoundPlanBuildingHeightM
}

func derivedReceiverHeight(proj *soundplanimport.Project) float64 {
	if proj == nil {
		return 4.0
	}

	if proj.Settings.ReceiverHeightAboveGround > 0 {
		return proj.Settings.ReceiverHeightAboveGround
	}

	if proj.GeoDB.RelHeightEFH > 0 {
		return proj.GeoDB.RelHeightEFH
	}

	return 4.0
}

func points3DToRing(points []soundplanimport.Point3D) []any {
	coords := make([]any, 0, len(points))
	for _, point := range points {
		coords = append(coords, []any{point.X, point.Y})
	}

	return coords
}

func float64Ptr(v float64) *float64 {
	out := v
	return &out
}
