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
	Format           string  `json:"format"`
	ProjectTitle     string  `json:"project_title"`
	ProjectVersion   int     `json:"project_version"`
	ProjectV64       bool    `json:"project_v64"`
	SourcePath       string  `json:"source_path"`
	ProjectCRS       string  `json:"project_crs"`
	AssumedImportCRS string  `json:"assumed_import_crs"`
	GridResolutionM  float64 `json:"grid_resolution_m,omitempty"`
	// GridMapHeightM is the receiver height the SoundPLAN project's grid maps
	// were computed at (`RLKHEIGHT`). The raster comparison synthesizes its
	// receivers at this height, because a grid map's own metadata does not
	// record one and the project's receivers are at facade heights that have
	// nothing to do with it.
	GridMapHeightM   float64                            `json:"grid_map_height_m,omitempty"`
	RunCount         int                                `json:"run_count"`
	CountsByKind     map[string]int                     `json:"counts_by_kind"`
	CalcArea         *soundPlanImportCalcArea           `json:"calc_area,omitempty"`
	CalcAreaBounds   *soundPlanBounds                   `json:"calc_area_bounds,omitempty"`
	TerrainSource    string                             `json:"terrain_source,omitempty"`
	GridMaps         []soundplanimport.GridMapMetadata  `json:"grid_maps,omitempty"`
	StandardMappings []soundplanimport.StandardMapping  `json:"standard_mappings"`
	Warnings         []string                           `json:"warnings,omitempty"`
	Decisions        []string                           `json:"decisions,omitempty"`
	Assessment       []soundplanimport.AssessmentPeriod `json:"assessment_periods,omitempty"`
	ResultRuns       []soundPlanImportRunSummary        `json:"result_runs,omitempty"`
}

type soundPlanImportCalcArea struct {
	Points   []soundPlanPoint `json:"points"`
	IsClosed bool             `json:"is_closed"`
}

type soundPlanPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// soundPlanBounds is the envelope of the calculation area an import read out of
// CalcArea.geo, reported as soundPlanImportReport.CalcAreaBounds.
//
// It describes the *imported bundle*, not the project's current extent. The
// model's calc-area feature is what a run computes over and what the raster
// comparison synthesizes receivers across, and a user may have moved it since
// the import. Nothing outside the import's own test reads this.
type soundPlanBounds struct {
	MinX float64 `json:"min_x"`
	MinY float64 `json:"min_y"`
	MaxX float64 `json:"max_x"`
	MaxY float64 `json:"max_y"`
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

	model, importReport := buildSoundPlanModelAndReport(bundle, proj.CRS, relInput)

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
		outputFieldWarnings, len(importReport.Warnings),
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

func buildSoundPlanModelAndReport(bundle *soundplanimport.ProjectBundle, projectCRS string, sourcePath string) (modelgeojson.Model, soundPlanImportReport) {
	counts := map[string]int{
		modelgeojson.FeatureKindSource:   0,
		modelgeojson.FeatureKindBuilding: 0,
		modelgeojson.FeatureKindBarrier:  0,
		modelgeojson.FeatureKindReceiver: 0,
		modelgeojson.FeatureKindCalcArea: 0,
	}

	warnings := append([]string(nil), bundle.Warnings...)
	decisions := make([]string, 0, 4)
	decisions = append(
		decisions,
		"SoundPLAN coordinates are imported without reprojection and assumed to already match the project CRS.",
		"Rail defaults are exported explicitly so the normalized model can enter the existing Schall 03 path before full train-operation mapping exists.",
	)

	buildingHeightM := derivedBuildingHeight(bundle.Project)
	receiverHeightM := derivedReceiverHeight(bundle.Project)
	terrainSource := soundPlanTerrainSource(bundle.Terrain)

	warnings = append(warnings, soundPlanBundleWarnings(bundle)...)

	features := make([]modelgeojson.Feature, 0, 512)

	railFeatures, railWarnings := buildSoundPlanRailFeatures(bundle)
	features = append(features, railFeatures...)
	counts[modelgeojson.FeatureKindSource] += len(railFeatures)

	warnings = append(warnings, railWarnings...)

	geoObjects := buildSoundPlanGeoObjectFeatures(bundle.GeoObjects, buildingHeightM, receiverHeightM)
	features = append(features, geoObjects.buildings...)
	features = append(features, geoObjects.receivers...)
	counts[modelgeojson.FeatureKindBuilding] += len(geoObjects.buildings)
	counts[modelgeojson.FeatureKindReceiver] += len(geoObjects.receivers)
	warnings = append(warnings, geoObjects.warnings...)
	decisions = append(decisions, geoObjects.decisions...)

	barrierFeatures, barrierWarnings := buildSoundPlanBarrierFeatures(bundle.Barriers)
	features = append(features, barrierFeatures...)
	counts[modelgeojson.FeatureKindBarrier] += len(barrierFeatures)

	warnings = append(warnings, barrierWarnings...)

	calcAreaFeature, calcAreaOK, calcAreaWarnings := buildSoundPlanCalcAreaFeature(bundle.CalcArea)
	if calcAreaOK {
		features = append(features, calcAreaFeature)
		counts[modelgeojson.FeatureKindCalcArea]++
	}

	warnings = append(warnings, calcAreaWarnings...)

	reportRuns := soundPlanRunSummaries(bundle.Runs)

	slices.Sort(warnings)

	var (
		calcAreaBounds *soundPlanBounds
		calcAreaMeta   *soundPlanImportCalcArea
	)

	if bundle.CalcArea != nil {
		calcAreaBounds = calcAreaEnvelope(bundle.CalcArea)
		calcAreaMeta = calcAreaMetadata(bundle.CalcArea)
	}

	report := soundPlanImportReport{
		Format:           "soundplan",
		ProjectTitle:     bundle.Project.Title,
		ProjectVersion:   bundle.Project.Version,
		ProjectV64:       bundle.Project.V64,
		SourcePath:       sourcePath,
		ProjectCRS:       projectCRS,
		AssumedImportCRS: projectCRS,
		GridResolutionM:  bundle.Project.Settings.GridMapDistance,
		GridMapHeightM:   bundle.Project.Settings.GridMapHeight,
		RunCount:         len(bundle.Runs),
		CountsByKind:     counts,
		CalcArea:         calcAreaMeta,
		CalcAreaBounds:   calcAreaBounds,
		TerrainSource:    terrainSource,
		GridMaps:         append([]soundplanimport.GridMapMetadata(nil), bundle.GridMaps...),
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

	return model, report
}

// soundPlanRunSummaries condenses the bundle's SoundPLAN runs into the shape
// the import report carries.
func soundPlanRunSummaries(runs []*soundplanimport.RunResult) []soundPlanImportRunSummary {
	summaries := make([]soundPlanImportRunSummary, 0, len(runs))
	for _, run := range runs {
		summaries = append(summaries, soundPlanImportRunSummary{
			RunType:         run.RunType,
			ResultSubFolder: run.ResultSubFolder,
			Warnings:        append([]string(nil), run.Warnings...),
		})
	}

	return summaries
}

// soundPlanTerrainSource names the file the terrain data was recovered from.
func soundPlanTerrainSource(terrain *soundplanimport.TerrainData) string {
	if terrain == nil {
		return ""
	}

	switch {
	case len(terrain.ContourLines) > 0:
		return "GeoTmp.geo"
	case len(terrain.ElevationPoints) > 0:
		return "Höhen.txt"
	case len(terrain.DGMFiles) > 0:
		return ".dgm"
	}

	return ""
}

// soundPlanBundleWarnings reports the modelling simplifications that apply to
// the bundle as a whole, independent of individual features.
func soundPlanBundleWarnings(bundle *soundplanimport.ProjectBundle) []string {
	warnings := make([]string, 0, 4)

	if len(bundle.Barriers) > 0 {
		warnings = append(warnings, "barrier height_m is collapsed to one scalar per barrier using the maximum point height; per-vertex height variation is preserved only in SoundPLAN-specific properties")
	}

	if len(bundle.RailTracks) > 0 {
		if len(bundle.RailOps) == 0 {
			warnings = append(warnings, "rail traffic, train class, and traction type could not be derived from SoundPLAN RRAD/RRAI tables; importer fell back to explicit placeholders")
		}

		warnings = append(warnings, "rail track form and roughness still use explicit placeholders until deeper SoundPLAN parameter mapping is implemented")
	}

	if len(bundle.GridMaps) > 0 {
		warnings = append(warnings, "SoundPLAN grid-map GM payloads are decoded into row spans and layer values, but spatial origin/alignment for direct raster delta maps is still unresolved")
	}

	return warnings
}

// buildSoundPlanRailFeatures converts SoundPLAN rail tracks into line sources.
func buildSoundPlanRailFeatures(bundle *soundplanimport.ProjectBundle) ([]modelgeojson.Feature, []string) {
	railOpsByName := make(map[string][]soundplanimport.RailOperationSummary)
	for _, summary := range bundle.RailOps {
		railOpsByName[strings.TrimSpace(summary.Railname)] = append(railOpsByName[strings.TrimSpace(summary.Railname)], summary)
	}

	features := make([]modelgeojson.Feature, 0, len(bundle.RailTracks))
	warnings := make([]string, 0, 4)

	for trackIndex, track := range bundle.RailTracks {
		opsForTrack := railOpsByName[strings.TrimSpace(track.Name)]
		trackSummary := aggregateRailSummaries(opsForTrack)

		for segmentIndex, segment := range track.Segments {
			if len(segment.Points) < 2 {
				warnings = append(warnings, fmt.Sprintf("rail track %q segment %d skipped because it has fewer than 2 points", track.Name, segmentIndex+1))
				continue
			}

			features = append(features, buildSoundPlanRailSegmentFeature(
				trackIndex, segmentIndex, track, segment, trackSummary, len(opsForTrack) == 0,
			))
		}
	}

	return features, warnings
}

// buildSoundPlanRailSegmentFeature builds the line-source feature for one rail segment.
func buildSoundPlanRailSegmentFeature(
	trackIndex int,
	segmentIndex int,
	track soundplanimport.RailTrack,
	segment soundplanimport.RailSegment,
	trackSummary soundplanimport.RailOperationSummary,
	placeholderMapping bool,
) modelgeojson.Feature {
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
		"soundplan_track_name":                    strings.TrimSpace(track.Name),
		"soundplan_segment_index":                 segmentIndex + 1,
		"soundplan_bridge_correction_db":          segment.Params.BridgeCorrection,
		"soundplan_track_height_m":                segment.Params.TrackHeight,
		"elevation_m":                             segment.Points[0].ZTrack,
		runParams.RailTrainClass.key():            coalesceString(trackSummary.TrainClass, schall03.TrainClassMixed),
		runParams.RailTractionType.key():          coalesceString(trackSummary.TractionType, schall03.TractionMixed),
		runParams.RailTrackType.key():             schall03.TrackTypeBallasted,
		runParams.RailTrackForm.key():             schall03.TrackFormMainline,
		runParams.RailTrackRoughnessClass.key():   schall03.RoughnessStandard,
		runParams.RailAverageTrainSpeedKPH.key():  speedKPH,
		runParams.RailCurveRadiusM.key():          defaultSoundPlanCurveRadiusM,
		runParams.RailOnBridge.key():              trackSummary.OnBridge || segment.Params.BridgeCorrection > -999.0,
		runParams.TrafficDayTrainsPerHour.key():   coalescePositive(trackSummary.TrafficDayPH, defaultSoundPlanTrafficDayPH),
		runParams.TrafficNightTrainsPerHour.key(): coalescePositive(trackSummary.TrafficNightPH, defaultSoundPlanTrafficNightPH),
		"soundplan_placeholder_mapping":           placeholderMapping,
		"soundplan_dominant_train_name":           trackSummary.DominantTrainName,
		"soundplan_train_names":                   trackSummary.TrainNames,
		"soundplan_day_train_count":               trackSummary.DayTrainCount,
		"soundplan_night_train_count":             trackSummary.NightTrainCount,
		"soundplan_track_vmax_kph":                trackSummary.TrackVMaxKPH,
		"soundplan_assessment_day_hours":          trackSummary.AssessmentDayHours,
		"soundplan_assessment_night_h":            trackSummary.AssessmentNightHrs,
	}

	return modelgeojson.Feature{
		ID:           id,
		Kind:         modelgeojson.FeatureKindSource,
		SourceType:   modelgeojson.SourceTypeLine,
		Properties:   properties,
		GeometryType: modelgeojson.GeometryTypeLineString,
		Coordinates:  coords,
	}
}

// soundPlanGeoObjectFeatures groups the features derived from GeoObjs.geo.
type soundPlanGeoObjectFeatures struct {
	buildings []modelgeojson.Feature
	receivers []modelgeojson.Feature
	warnings  []string
	decisions []string
}

// buildSoundPlanGeoObjectFeatures converts GeoObjs.geo buildings and immission
// points.
func buildSoundPlanGeoObjectFeatures(
	objects *soundplanimport.GeoObjects,
	buildingHeightM float64,
	receiverHeightM float64,
) soundPlanGeoObjectFeatures {
	if objects == nil {
		return soundPlanGeoObjectFeatures{}
	}

	out := soundPlanGeoObjectFeatures{
		buildings: make([]modelgeojson.Feature, 0, len(objects.Buildings)),
		receivers: make([]modelgeojson.Feature, 0, len(objects.ImmissionPoints)),
		warnings:  make([]string, 0, 4),
		decisions: make([]string, 0, 2),
	}

	missingBuildingHeights := 0

	for buildingIndex, building := range objects.Buildings {
		if len(building.Footprint) < 4 {
			out.warnings = append(out.warnings, fmt.Sprintf("building %d skipped because footprint has fewer than 4 points", buildingIndex+1))
			continue
		}

		heightM := buildingHeightM

		properties := map[string]any{
			"soundplan_base_elevation_m": building.Footprint[0].Z,
		}
		if building.HeightM > 0 {
			heightM = building.HeightM
		} else {
			missingBuildingHeights++
			properties["soundplan_placeholder_height"] = true
		}

		switch len(building.Addresses) {
		case 1:
			properties["soundplan_address"] = building.Addresses[0]
		case 0:
		default:
			properties["soundplan_addresses"] = append([]string(nil), building.Addresses...)
		}

		out.buildings = append(out.buildings, modelgeojson.Feature{
			ID:           fmt.Sprintf("soundplan-building-%04d", buildingIndex+1),
			Kind:         modelgeojson.FeatureKindBuilding,
			HeightM:      float64Ptr(heightM),
			Properties:   properties,
			GeometryType: modelgeojson.GeometryTypePolygon,
			Coordinates:  []any{points3DToRing(building.Footprint)},
		})
	}

	if missingBuildingHeights > 0 {
		out.warnings = append(out.warnings, fmt.Sprintf("building heights were missing for %d GeoObjs buildings; imported those features with derived default height %.2f m", missingBuildingHeights, buildingHeightM))
	}

	for pointIndex, point := range objects.ImmissionPoints {
		features, warnings := buildSoundPlanReceiverColumn(point, pointIndex, receiverHeightM)
		out.receivers = append(out.receivers, features...)
		out.warnings = append(out.warnings, warnings...)
	}

	if len(objects.ImmissionPoints) > 0 {
		out.decisions = append(out.decisions, fmt.Sprintf(
			"SoundPLAN evaluates an Immissionsort once per floor, so each of the %d immission points in GeoObjs.geo is imported as a column of receivers — %d in total — keyed by (soundplan_obj_id, soundplan_floor), which is the key RREC*.abs joins on.",
			len(objects.ImmissionPoints), len(out.receivers),
		))
	}

	if len(objects.MapLabels) > 0 {
		out.decisions = append(out.decisions, fmt.Sprintf(
			"GeoObjs.geo object type 0x0028 is a map text caption (house numbers, bridge and dimension labels), not an Immissionsort; the %d such objects are no longer imported as receivers.",
			len(objects.MapLabels),
		))
	}

	return out
}

// buildSoundPlanReceiverColumn expands one Immissionsort into the receivers
// SoundPLAN evaluates for it: one per floor, at that floor's height above
// ground.
//
// A point whose floor attributes could not be decoded, or whose geometry puts
// a floor at or below ground level, is not expanded. It becomes a single
// receiver at the project's default height, carrying floor 0, and the caller
// warns naming the point — an incomplete column would silently drop reference
// rows, and a guessed one would invent receivers SoundPLAN never computed.
func buildSoundPlanReceiverColumn(
	point soundplanimport.ImmissionPoint,
	pointIndex int,
	defaultHeightM float64,
) ([]modelgeojson.Feature, []string) {
	if !point.HasFloorAttrs || point.FloorCount <= 0 {
		return []modelgeojson.Feature{
				soundPlanReceiverFeature(point, pointIndex, 0, defaultHeightM),
			}, []string{fmt.Sprintf(
				"SoundPLAN immission point %s carries no decodable floor attributes; imported as one receiver at the project default height %.2f m instead of one per floor",
				describeSoundPlanImmissionPoint(point, pointIndex), defaultHeightM,
			)}
	}

	features := make([]modelgeojson.Feature, 0, point.FloorCount)

	for floor := 1; floor <= point.FloorCount; floor++ {
		heightM := point.FloorHeightM(floor)
		if heightM <= 0 {
			return []modelgeojson.Feature{
					soundPlanReceiverFeature(point, pointIndex, 0, defaultHeightM),
				}, []string{fmt.Sprintf(
					"SoundPLAN immission point %s puts floor %d at %.3f m above ground; imported as one receiver at the project default height %.2f m instead of one per floor",
					describeSoundPlanImmissionPoint(point, pointIndex), floor, heightM, defaultHeightM,
				)}
		}

		features = append(features, soundPlanReceiverFeature(point, pointIndex, floor, heightM))
	}

	return features, nil
}

// soundPlanReceiverFeature builds one receiver of an Immissionsort's column.
// Floor 0 means the column could not be expanded; see
// buildSoundPlanReceiverColumn.
func soundPlanReceiverFeature(
	point soundplanimport.ImmissionPoint,
	pointIndex int,
	floor int,
	heightM float64,
) modelgeojson.Feature {
	zM := point.FloorRefZ
	if floor > 0 {
		zM = point.FloorZ(floor)
	}

	properties := map[string]any{
		"soundplan_obj_id":          point.ObjID,
		"soundplan_receiver_name":   point.Name,
		"soundplan_floor":           floor,
		"soundplan_floor_count":     point.FloorCount,
		"soundplan_z_m":             zM,
		"soundplan_ground_height_m": point.GroundHeightM,
		"soundplan_limit_day_db":    point.LimitDayDB,
		"soundplan_limit_night_db":  point.LimitNightDB,
	}

	if floor == 0 {
		properties["soundplan_floor_attributes_missing"] = true
	}

	return modelgeojson.Feature{
		ID:           soundPlanReceiverFeatureID(point, pointIndex, floor),
		Kind:         modelgeojson.FeatureKindReceiver,
		HeightM:      float64Ptr(heightM),
		Properties:   properties,
		GeometryType: modelgeojson.GeometryTypePoint,
		Coordinates:  []any{point.X, point.Y},
	}
}

// soundPlanReceiverFeatureID names a receiver after the SoundPLAN object it
// came from, falling back to the point's position in the file when the object
// id could not be decoded — an id of 0 is not an id and must not collide.
func soundPlanReceiverFeatureID(point soundplanimport.ImmissionPoint, pointIndex int, floor int) string {
	if point.ObjID > 0 {
		return fmt.Sprintf("soundplan-receiver-%d-f%d", point.ObjID, floor)
	}

	return fmt.Sprintf("soundplan-receiver-p%04d-f%d", pointIndex+1, floor)
}

// describeSoundPlanImmissionPoint names a point in a warning a user has to act
// on, so it says which object in their project is affected.
func describeSoundPlanImmissionPoint(point soundplanimport.ImmissionPoint, pointIndex int) string {
	name := strings.TrimSpace(point.Name)
	if name == "" {
		name = fmt.Sprintf("#%d in file order", pointIndex+1)
	}

	if point.ObjID > 0 {
		return fmt.Sprintf("%q (obj %d)", name, point.ObjID)
	}

	return fmt.Sprintf("%q", name)
}

// buildSoundPlanBarrierFeatures converts SoundPLAN noise barriers into barrier lines.
func buildSoundPlanBarrierFeatures(barriers []soundplanimport.NoiseBarrier) ([]modelgeojson.Feature, []string) {
	features := make([]modelgeojson.Feature, 0, len(barriers))
	warnings := make([]string, 0, 4)

	for barrierIndex, barrier := range barriers {
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

		properties := map[string]any{
			"soundplan_height_profile_m": topHeights,
			"soundplan_variable_height":  true,
		}
		if barrier.HasAcousticProperties {
			properties["soundplan_barrier_absorption_a_db"] = barrier.AbsorptionSideADB

			properties["soundplan_barrier_absorption_b_db"] = barrier.AbsorptionSideBDB
			if barrier.MaterialCode >= 0 {
				properties["soundplan_barrier_material_code"] = barrier.MaterialCode
			} else {
				properties["soundplan_barrier_material_unset"] = true
			}
		}

		features = append(features, modelgeojson.Feature{
			ID:           fmt.Sprintf("soundplan-barrier-%03d", barrierIndex+1),
			Kind:         modelgeojson.FeatureKindBarrier,
			HeightM:      float64Ptr(maxHeight),
			Properties:   properties,
			GeometryType: modelgeojson.GeometryTypeLineString,
			Coordinates:  coords,
		})
	}

	return features, warnings
}

// buildSoundPlanCalcAreaFeature converts the bundle's CalcArea.geo polygon into
// the single calc-area feature the model schema permits. A SoundPLAN bundle
// already says where its author wanted results computed, so emitting it makes
// an imported project's automatic receiver grid cover that extent instead of
// the extent the imported sources happen to span.
//
// A bundle without a calculation area is the normal case, not an error:
// CalcArea.geo is optional, and such an import is left exactly as it was.
//
// Z is dropped, following every other SoundPLAN appender: rail segments,
// building footprints (points3DToRing) and receivers all emit 2D coordinates
// and keep the third dimension as a SoundPLAN-specific property. Here the
// elevation carries no acoustic meaning at all — the validator deliberately
// applies no height_m rule to a calculation area, because it is a footprint on
// the ground bounding the receiver grid rather than an object sound travels
// around — so the first vertex's elevation is preserved as metadata only, named
// as it is on buildings, which are the other footprint polygons in this model.
func buildSoundPlanCalcAreaFeature(area *soundplanimport.CalcArea) (modelgeojson.Feature, bool, []string) {
	if area == nil || len(area.Points) == 0 {
		return modelgeojson.Feature{}, false, nil
	}

	ring := points3DToRing(area.Points)

	// Closure is decided in 2D because the emitted ring is 2D. The import
	// report's IsClosed compares Z as well, so a bundle whose first and last
	// vertex differ only in elevation reports "not closed" while its footprint
	// already is; appending the first vertex again there would add a
	// zero-length segment to an otherwise valid ring.
	if !soundPlanRingIsClosed(area.Points) {
		ring = append(ring, ring[0])
	}

	// A polygon ring needs four coordinates, so fewer than three distinct
	// vertices cannot describe an area. Refusing to emit it keeps the import
	// working: an invalid feature would fail validation and reject the whole
	// bundle over a degenerate extent that is worth nothing anyway.
	if len(ring) < 4 {
		return modelgeojson.Feature{}, false, []string{fmt.Sprintf(
			"CalcArea.geo carries only %d point(s) and cannot form a polygon ring; imported without a calc-area feature, so the receiver grid falls back to the source extent",
			len(area.Points),
		)}
	}

	return modelgeojson.Feature{
		ID:           "soundplan-calc-area",
		Kind:         modelgeojson.FeatureKindCalcArea,
		Properties:   map[string]any{"soundplan_base_elevation_m": area.Points[0].Z},
		GeometryType: modelgeojson.GeometryTypePolygon,
		Coordinates:  []any{ring},
	}, true, nil
}

// soundPlanRingIsClosed reports whether the footprint's first and last vertex
// coincide in plan, which is the closure the emitted 2D ring is judged by.
func soundPlanRingIsClosed(points []soundplanimport.Point3D) bool {
	if len(points) < 2 {
		return false
	}

	first := points[0]
	last := points[len(points)-1]

	return first.X == last.X && first.Y == last.Y
}

func calcAreaMetadata(area *soundplanimport.CalcArea) *soundPlanImportCalcArea {
	if area == nil || len(area.Points) == 0 {
		return nil
	}

	points := make([]soundPlanPoint, 0, len(area.Points))
	for _, point := range area.Points {
		points = append(points, soundPlanPoint{
			X: point.X,
			Y: point.Y,
			Z: point.Z,
		})
	}

	isClosed := false

	if len(points) >= 2 {
		first := points[0]
		last := points[len(points)-1]
		isClosed = first.X == last.X && first.Y == last.Y && first.Z == last.Z
	}

	return &soundPlanImportCalcArea{
		Points:   points,
		IsClosed: isClosed,
	}
}

// railSummaryAccumulator collects the cross-record state needed to fold several
// SoundPLAN rail operation records into one summary.
type railSummaryAccumulator struct {
	classSeen      map[string]struct{}
	tractionSeen   map[string]struct{}
	nameSeen       map[string]struct{}
	dominantWeight float64
	speedWeight    float64
}

// add folds one rail operation record into out and updates the accumulator state.
func (a *railSummaryAccumulator) add(out *soundplanimport.RailOperationSummary, item soundplanimport.RailOperationSummary) {
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
		a.speedWeight += weight
	}

	if item.TrainClass != "" {
		a.classSeen[item.TrainClass] = struct{}{}
	}

	if item.TractionType != "" {
		a.tractionSeen[item.TractionType] = struct{}{}
	}

	for _, name := range item.TrainNames {
		if _, ok := a.nameSeen[name]; ok || strings.TrimSpace(name) == "" {
			continue
		}

		a.nameSeen[name] = struct{}{}
		out.TrainNames = append(out.TrainNames, name)
	}

	if weight > a.dominantWeight && strings.TrimSpace(item.DominantTrainName) != "" {
		a.dominantWeight = weight
		out.DominantTrainName = item.DominantTrainName
	}
}

// collapseRailCategory reduces a set of observed category values to a single one:
// empty when nothing was observed, the sole value when unambiguous, otherwise mixed.
func collapseRailCategory(seen map[string]struct{}, mixed string) string {
	switch len(seen) {
	case 0:
		return ""
	case 1:
		for value := range seen {
			return value
		}

		return ""
	default:
		return mixed
	}
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

	acc := railSummaryAccumulator{
		classSeen:      make(map[string]struct{}),
		tractionSeen:   make(map[string]struct{}),
		nameSeen:       make(map[string]struct{}),
		dominantWeight: -1.0,
	}

	for _, item := range items {
		acc.add(&out, item)
	}

	if acc.speedWeight > 0 {
		out.AverageSpeedKPH /= acc.speedWeight
	}

	out.TrainClass = collapseRailCategory(acc.classSeen, schall03.TrainClassMixed)
	out.TractionType = collapseRailCategory(acc.tractionSeen, schall03.TractionMixed)

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
		{ID: project.ArtifactIDModelNormalized, Kind: project.ArtifactKindModelNormalizedGeoJSON, Path: relativePath(store.Root(), normalizedPath), CreatedAt: now},
		{ID: project.ArtifactIDModelDump, Kind: project.ArtifactKindModelDumpJSON, Path: relativePath(store.Root(), dumpPath), CreatedAt: now},
		{ID: project.ArtifactIDModelValidation, Kind: project.ArtifactKindModelValidationReport, Path: relativePath(store.Root(), reportPath), CreatedAt: now},
		{ID: "artifact-soundplan-import-report", Kind: "model.soundplan_import_report", Path: relativePath(store.Root(), importReportPath), CreatedAt: now},
	} {
		proj.Artifacts = upsertArtifact(proj.Artifacts, ref)
	}

	if err := store.Save(*proj); err != nil {
		return fmt.Errorf("save project manifest: %w", err)
	}

	return nil
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

func calcAreaEnvelope(area *soundplanimport.CalcArea) *soundPlanBounds {
	if area == nil || len(area.Points) == 0 {
		return nil
	}

	bounds := &soundPlanBounds{
		MinX: area.Points[0].X,
		MinY: area.Points[0].Y,
		MaxX: area.Points[0].X,
		MaxY: area.Points[0].Y,
	}

	for _, point := range area.Points[1:] {
		if point.X < bounds.MinX {
			bounds.MinX = point.X
		}

		if point.Y < bounds.MinY {
			bounds.MinY = point.Y
		}

		if point.X > bounds.MaxX {
			bounds.MaxX = point.X
		}

		if point.Y > bounds.MaxY {
			bounds.MaxY = point.Y
		}
	}

	return bounds
}
