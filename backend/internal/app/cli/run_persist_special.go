package cli

import (
	"path/filepath"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/results"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	"github.com/aconiq/backend/internal/standards/framework"
	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// persistRLS19RoadRunOutputs writes an RLS-19 run. RLS-19 has no model
// version of its own: the data pack it evaluates is the only thing that
// versions its results.
func persistRLS19RoadRunOutputs(
	runDir string,
	outputs []rls19road.ReceiverOutput,
	layout results.GridLayout,
	sourceCount int,
	sourceOverrideCount int,
	parkingSourceCount int,
	receiverMode string,
	tier framework.EvidenceTier,
	projection computeProjection,
) (persistedRunOutputs, string, time.Time, error) {
	const scope = "cli.persistRLS19RoadRunOutputs"

	// Before the plan literal: `export` closes over layout, so a stamp
	// applied later would reach the run summary and miss the raster sidecar.
	layout = withComputeCRS(layout, projection)

	plan := receiverPersistPlan[rls19road.ReceiverOutput]{
		scope:          scope,
		hashLabel:      "RLS-19 road receiver outputs",
		hashErrMessage: "hash RLS-19 road outputs",
		modelVersion:   rls19road.BuiltinDataPackVersion,
		indicatorOrder: []string{rls19road.IndicatorLrDay, rls19road.IndicatorLrNight},
		decorateSummary: func(summary map[string]any) {
			summary["sources_with_feature_acoustics_overrides"] = sourceOverrideCount
			// Reported separately because source_count counts line sources: a
			// Parkplatz-only run would otherwise show source_count 0 on success.
			summary["parking_source_count"] = parkingSourceCount
			summary["reporting_precision_db"] = rls19road.ReportingPrecisionDB
		},
		receiver:   func(output rls19road.ReceiverOutput) geo.PointReceiver { return output.Receiver },
		indicators: func(output rls19road.ReceiverOutput) any { return output.Indicators },
		values: func(output rls19road.ReceiverOutput) map[string]float64 {
			return map[string]float64{
				rls19road.IndicatorLrDay:   output.Indicators.LrDay,
				rls19road.IndicatorLrNight: output.Indicators.LrNight,
			}
		},
		export: exportBundle(scope, "export RLS-19 road results", rls19road.ExportResultBundle, outputs, layout),
	}

	return persistReceiverRunOutputs(plan, runDir, outputs, layout, sourceCount, receiverMode, tier, projection)
}

func persistSchall03RunOutputs(
	runDir string,
	outputs []schall03.ReceiverOutput,
	layout results.GridLayout,
	sourceCount int,
	receiverMode string,
	engine string,
	tier framework.EvidenceTier,
	projection computeProjection,
) (persistedRunOutputs, string, time.Time, error) {
	const scope = "cli.persistSchall03RunOutputs"

	// Before the plan literal: `export` closes over layout, so a stamp
	// applied later would reach the run summary and miss the raster sidecar.
	layout = withComputeCRS(layout, projection)

	plan := receiverPersistPlan[schall03.ReceiverOutput]{
		scope:          scope,
		hashLabel:      "Schall 03 receiver outputs",
		hashErrMessage: "hash Schall 03 outputs",
		modelVersion:   schall03.ModelVersionForEngine(engine),
		indicatorOrder: []string{schall03.IndicatorLrDay, schall03.IndicatorLrNight},
		decorateSummary: func(summary map[string]any) {
			summary["compliance_boundary"] = schall03.ComplianceBoundaryForEngine(engine)
			summary[schall03.ParamEngine] = engine
			summary["reporting_precision_db"] = schall03.ReportingPrecisionDB
			summary["band_model"] = "octave-63Hz-8000Hz"

			// The data pack exists only on the preview path; the normative chain
			// reads Beiblatt 1/2 and the Anlage-2 tables directly.
			if engine != schall03.EngineNormative {
				summary["data_pack_version"] = schall03.BuiltinDataPackVersion
			}
		},
		receiver:   func(output schall03.ReceiverOutput) geo.PointReceiver { return output.Receiver },
		indicators: func(output schall03.ReceiverOutput) any { return output.Indicators },
		values: func(output schall03.ReceiverOutput) map[string]float64 {
			return map[string]float64{
				schall03.IndicatorLrDay:   output.Indicators.LrDay,
				schall03.IndicatorLrNight: output.Indicators.LrNight,
			}
		},
		export: exportBundle(scope, "export Schall 03 results", schall03.ExportResultBundle, outputs, layout),
	}

	return persistReceiverRunOutputs(plan, runDir, outputs, layout, sourceCount, receiverMode, tier, projection)
}

func persistISO9613RunOutputs(
	runDir string,
	outputs []iso9613.ReceiverOutput,
	layout results.GridLayout,
	sourceCount int,
	receiverMode string,
	tier framework.EvidenceTier,
	projection computeProjection,
) (persistedRunOutputs, string, time.Time, error) {
	const scope = "cli.persistISO9613RunOutputs"

	// Before the plan literal: `export` closes over layout, so a stamp
	// applied later would reach the run summary and miss the raster sidecar.
	layout = withComputeCRS(layout, projection)

	plan := receiverPersistPlan[iso9613.ReceiverOutput]{
		scope:           scope,
		hashLabel:       "ISO 9613 receiver outputs",
		hashErrMessage:  "hash iso9613 outputs",
		modelVersion:    iso9613.BuiltinModelVersion,
		indicatorOrder:  []string{iso9613.IndicatorLpAeqDW, iso9613.IndicatorLpAeqLT},
		decorateSummary: func(summary map[string]any) { summary["indicator"] = iso9613.IndicatorLpAeqDW },
		receiver:        func(output iso9613.ReceiverOutput) geo.PointReceiver { return output.Receiver },
		indicators:      func(output iso9613.ReceiverOutput) any { return output.Indicators },
		values: func(output iso9613.ReceiverOutput) map[string]float64 {
			return map[string]float64{
				iso9613.IndicatorLpAeqDW: output.Indicators.LpAeqDW,
				iso9613.IndicatorLpAeqLT: output.Indicators.LpAeqLT,
			}
		},
		export: exportBundle(scope, "export iso9613 results", iso9613.ExportResultBundle, outputs, layout),
	}

	return persistReceiverRunOutputs(plan, runDir, outputs, layout, sourceCount, receiverMode, tier, projection)
}

func persistBEBExposureRunOutputs(
	runDir string,
	outputs []bebexposure.BuildingExposureOutput,
	summary bebexposure.Summary,
	sourceCount int,
	tier framework.EvidenceTier,
	projection computeProjection,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	exported, err := bebexposure.ExportResultBundle(resultsDir, outputs, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistBEBExposureRunOutputs", "export BEB exposure results", err)
	}

	outputHash, err := hashBEBExposureOutputs(outputs, summary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistBEBExposureRunOutputs", "hash BEB exposure outputs", err)
	}

	// BEB counts buildings rather than receivers and has no receiver mode, so it
	// does not share the newRunSummary shape.
	runSummary := map[string]any{
		"run_id":                    filepath.Base(runDir),
		"status":                    project.RunStatusCompleted,
		"output_hash":               outputHash,
		evidenceTierKey:             string(tier),
		"source_count":              sourceCount,
		"building_count":            len(outputs),
		"estimated_dwellings":       summary.EstimatedDwellings,
		"estimated_persons":         summary.EstimatedPersons,
		"affected_dwellings_lden":   summary.AffectedDwellingsLden,
		"affected_persons_lden":     summary.AffectedPersonsLden,
		"affected_dwellings_lnight": summary.AffectedDwellingsLnight,
		"affected_persons_lnight":   summary.AffectedPersonsLnight,
		"model_version":             bebexposure.BuiltinModelVersion,
		"reporting_precision_db":    bebexposure.ReportingPrecisionCount,
		"occupancy_mode":            summary.OccupancyMode,
		"facade_evaluation_mode":    summary.FacadeEvaluationMode,
		"upstream_mapping_standard": summary.UpstreamMappingStandard,
		"lden_bands":                summary.LdenBands,
		"lnight_bands":              summary.LnightBands,
	}

	addComputeCRS(runSummary, projection)

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, runSummary)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	return persistedRunOutputs{
		ReceiverJSONPath:   exported.ReceiverJSONPath,
		ReceiverCSVPath:    exported.ReceiverCSVPath,
		RasterMetadataPath: exported.RasterMetaPath,
		RasterDataPath:     exported.RasterDataPath,
		SummaryPath:        summaryPath,
	}, outputHash, nowUTC(), nil
}

// hashBEBExposureOutputs differs from the others in shape, not only in type:
// the records are keyed by building and the run-level summary is inside what
// the hash covers.
func hashBEBExposureOutputs(outputs []bebexposure.BuildingExposureOutput, summary bebexposure.Summary) (string, error) {
	records := make([]hashedBuildingRecord[bebexposure.BuildingIndicators], 0, len(outputs))
	for _, output := range outputs {
		records = append(records, hashedBuildingRecord[bebexposure.BuildingIndicators]{
			BuildingID: output.Building.ID,
			Indicators: output.Indicators,
		})
	}

	return hashJSONPayload("BEB exposure outputs", struct {
		Buildings []hashedBuildingRecord[bebexposure.BuildingIndicators] `json:"buildings"`
		Summary   bebexposure.Summary                                    `json:"summary"`
	}{
		Buildings: records,
		Summary:   summary,
	})
}
