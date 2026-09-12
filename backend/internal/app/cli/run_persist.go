package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aconiq/backend/internal/acoustics"
	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/engine"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/report/results"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubindustry "github.com/aconiq/backend/internal/standards/bub/industry"
	bubrail "github.com/aconiq/backend/internal/standards/bub/rail"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/framework"
	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// newRunSummary builds the keys every standard-backed run summary carries.
// Callers add only what is specific to their standard on top of it.
//
// modelVersion may be empty for a module that versions nothing of its own; the
// key is then left out rather than written as an empty string.
func newRunSummary(
	runDir string,
	outputHash string,
	modelVersion string,
	receiverMode string,
	tier framework.EvidenceTier,
	sourceCount int,
	receiverCount int,
) map[string]any {
	summary := map[string]any{
		"run_id":         filepath.Base(runDir),
		"status":         project.RunStatusCompleted,
		"output_hash":    outputHash,
		"source_count":   sourceCount,
		"receiver_count": receiverCount,
		"receiver_mode":  receiverMode,
		evidenceTierKey:  string(tier),
	}

	if modelVersion != "" {
		summary["model_version"] = modelVersion
	}

	return summary
}

// writeGridRunSummary stamps the raster grid dimensions onto a run summary and
// writes it next to the exported result bundle.
func writeGridRunSummary(resultsDir string, summary map[string]any, gridWidth int, gridHeight int) (string, error) {
	summary["grid_width"] = gridWidth
	summary["grid_height"] = gridHeight

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err := writeJSONFile(summaryPath, summary)
	if err != nil {
		return "", err
	}

	return summaryPath, nil
}

func persistReceiverTableOnly(
	resultsDir string,
	table results.ReceiverTable,
	summary map[string]any,
) (persistedRunOutputs, error) {
	err := os.MkdirAll(resultsDir, 0o750)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistReceiverTableOnly", "create results directory "+resultsDir, err)
	}

	receiverJSONPath := filepath.Join(resultsDir, "receivers.json")
	receiverCSVPath := filepath.Join(resultsDir, "receivers.csv")

	err = results.SaveReceiverTableJSON(receiverJSONPath, table)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistReceiverTableOnly", "save receiver table json", err)
	}

	err = results.SaveReceiverTableCSV(receiverCSVPath, table)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistReceiverTableOnly", "save receiver table csv", err)
	}

	summaryPath := filepath.Join(resultsDir, "run-summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return persistedRunOutputs{}, err
	}

	return persistedRunOutputs{
		ReceiverJSONPath: receiverJSONPath,
		ReceiverCSVPath:  receiverCSVPath,
		SummaryPath:      summaryPath,
	}, nil
}

// buildDummyReceiverTable maps engine results onto the receiver set. Every
// receiver must have a result; a missing one is an internal inconsistency.
func buildDummyReceiverTable(receivers []geo.PointReceiver, levelByReceiver map[string]float64, indicator string) (results.ReceiverTable, error) {
	table := results.ReceiverTable{
		IndicatorOrder: []string{indicator},
		Unit:           dummyResultUnit,
		Records:        make([]results.ReceiverRecord, 0, len(receivers)),
	}

	for _, receiver := range receivers {
		level, ok := levelByReceiver[receiver.ID]
		if !ok {
			return results.ReceiverTable{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "missing result for receiver "+receiver.ID, nil)
		}

		table.Records = append(table.Records, results.ReceiverRecord{
			ID:      receiver.ID,
			X:       receiver.Point.X,
			Y:       receiver.Point.Y,
			HeightM: receiver.HeightM,
			Values: map[string]float64{
				indicator: level,
			},
		})
	}

	return table, nil
}

// persistDummyRaster writes the grid raster. Receivers are laid out row-major
// in the order the grid produced them.
func persistDummyRaster(
	resultsDir string,
	receivers []geo.PointReceiver,
	levelByReceiver map[string]float64,
	gridWidth int,
	gridHeight int,
	indicator string,
) (results.RasterPersistence, error) {
	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     gridWidth,
		Height:    gridHeight,
		Bands:     1,
		NoData:    -9999,
		Unit:      dummyResultUnit,
		BandNames: []string{indicator},
	})
	if err != nil {
		return results.RasterPersistence{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "build raster", err)
	}

	for receiverIndex, receiver := range receivers {
		level := levelByReceiver[receiver.ID]
		x := receiverIndex % gridWidth
		y := receiverIndex / gridWidth

		err := raster.Set(x, y, 0, level)
		if err != nil {
			return results.RasterPersistence{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "set raster value", err)
		}
	}

	rasterBasePath := filepath.Join(resultsDir, strings.ToLower(indicator))

	rasterPersistence, err := results.SaveRaster(rasterBasePath, raster)
	if err != nil {
		return results.RasterPersistence{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "save raster", err)
	}

	return rasterPersistence, nil
}

func persistDummyRunOutputs(
	runDir string,
	runOutput engine.RunOutput,
	receivers []geo.PointReceiver,
	gridWidth int,
	gridHeight int,
	indicator string,
	tier framework.EvidenceTier,
) (persistedRunOutputs, error) {
	resultsDir := filepath.Join(runDir, "results")

	err := os.MkdirAll(resultsDir, 0o750)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "create results directory "+resultsDir, err)
	}

	levelByReceiver := make(map[string]float64, len(runOutput.Results))
	for _, receiverResult := range runOutput.Results {
		levelByReceiver[receiverResult.ReceiverID] = receiverResult.LevelDB
	}

	table, err := buildDummyReceiverTable(receivers, levelByReceiver, indicator)
	if err != nil {
		return persistedRunOutputs{}, err
	}

	receiverJSONPath := filepath.Join(resultsDir, "receivers.json")
	receiverCSVPath := filepath.Join(resultsDir, "receivers.csv")

	err = results.SaveReceiverTableJSON(receiverJSONPath, table)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "save receiver table json", err)
	}

	err = results.SaveReceiverTableCSV(receiverCSVPath, table)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "save receiver table csv", err)
	}

	// The engine reports the source count it actually chunked; the test fixture
	// versions no model of its own, so it contributes no model version.
	sourceCount, _ := runOutput.Metadata["source_count"].(int)

	summary := newRunSummary(runDir, runOutput.OutputHash, "", receiverModeAutoGrid, tier, sourceCount, len(receivers))
	summary["total_chunks"] = runOutput.TotalChunks
	summary["used_cached_chunks"] = runOutput.UsedCachedChunks

	if gridWidth <= 0 || gridHeight <= 0 {
		summary["receiver_mode"] = receiverModeCustom

		summaryPath := filepath.Join(resultsDir, "run-summary.json")

		err := writeJSONFile(summaryPath, summary)
		if err != nil {
			return persistedRunOutputs{}, err
		}

		return persistedRunOutputs{
			ReceiverJSONPath: receiverJSONPath,
			ReceiverCSVPath:  receiverCSVPath,
			SummaryPath:      summaryPath,
		}, nil
	}

	rasterPersistence, err := persistDummyRaster(resultsDir, receivers, levelByReceiver, gridWidth, gridHeight, indicator)
	if err != nil {
		return persistedRunOutputs{}, err
	}

	summaryPath, err := writeGridRunSummary(resultsDir, summary, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, err
	}

	return persistedRunOutputs{
		ReceiverJSONPath:   receiverJSONPath,
		ReceiverCSVPath:    receiverCSVPath,
		RasterMetadataPath: rasterPersistence.MetadataPath,
		RasterDataPath:     rasterPersistence.DataPath,
		SummaryPath:        summaryPath,
	}, nil
}

// exportOutputs is the shape every standards module's ExportResultBundle
// returns. The per-module structs are structurally identical but nominally
// distinct, so this constraint is what lets one adapter convert any of them.
type exportOutputs interface {
	~struct {
		ReceiverJSONPath string
		ReceiverCSVPath  string
		RasterMetaPath   string
		RasterDataPath   string
	}
}

// exportBundle adapts a module's ExportResultBundle to the export hook a
// persist plan carries: it binds the outputs and the grid to write and wraps a
// failure in the caller's scope.
func exportBundle[Output any, Bundle exportOutputs](
	scope string,
	message string,
	export func(string, []Output, int, int) (Bundle, error),
	outputs []Output,
	gridWidth int,
	gridHeight int,
) func(resultsDir string) (exportedBundle, error) {
	return func(resultsDir string) (exportedBundle, error) {
		exported, err := export(resultsDir, outputs, gridWidth, gridHeight)
		if err != nil {
			return exportedBundle{}, domainerrors.New(domainerrors.KindInternal, scope, message, err)
		}

		return exportedBundle(exported), nil
	}
}

// receiverPersistPlan carries the whole of what one standard's receiver-output
// persist path does not share with the others: the identity its errors and its
// run summary carry, the indicator vocabulary its receiver table publishes, and
// the result bundle it writes to disk. Everything around those — the output
// hash, the run summary, the custom-receiver short circuit and the grid bundle —
// is the same for every standard that produces receiver outputs, and lives once
// in persistReceiverRunOutputs.
//
// indicators returns any rather than a second type parameter because the value
// is only ever marshalled: the hash payload is byte-identical either way.
type receiverPersistPlan[Output any] struct {
	scope           string
	hashLabel       string
	hashErrMessage  string
	modelVersion    string
	indicatorOrder  []string
	decorateSummary func(summary map[string]any)
	receiver        func(Output) geo.PointReceiver
	indicators      func(Output) any
	values          func(Output) map[string]float64
	export          func(resultsDir string) (exportedBundle, error)
}

// persistReceiverRunOutputs writes one standard's receiver outputs under runDir
// and returns the paths, the output hash and the finish time the run manifest
// records. In custom-receiver mode it writes a receiver table only; otherwise it
// writes the standard's full grid bundle beside a grid run summary.
func persistReceiverRunOutputs[Output any](
	plan receiverPersistPlan[Output],
	runDir string,
	outputs []Output,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
	tier framework.EvidenceTier,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")

	outputHash, err := hashReceiverOutputs(
		plan.hashLabel,
		outputs,
		func(output Output) string { return plan.receiver(output).ID },
		plan.indicators,
	)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, plan.scope, plan.hashErrMessage, err)
	}

	summary := newRunSummary(runDir, outputHash, plan.modelVersion, receiverMode, tier, sourceCount, len(outputs))
	if plan.decorateSummary != nil {
		plan.decorateSummary(summary)
	}

	if receiverMode == receiverModeCustom {
		table := results.ReceiverTable{
			IndicatorOrder: plan.indicatorOrder,
			Unit:           "dB",
			Records:        make([]results.ReceiverRecord, 0, len(outputs)),
		}

		for _, output := range outputs {
			receiver := plan.receiver(output)
			table.Records = append(table.Records, results.ReceiverRecord{
				ID:      receiver.ID,
				X:       receiver.Point.X,
				Y:       receiver.Point.Y,
				HeightM: receiver.HeightM,
				Values:  plan.values(output),
			})
		}

		persisted, err := persistReceiverTableOnly(resultsDir, table, summary)

		return persisted, outputHash, nowUTC(), err
	}

	exported, err := plan.export(resultsDir)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, err
	}

	summaryPath, err := writeGridRunSummary(resultsDir, summary, gridWidth, gridHeight)
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

// endPersistSpec is what one END strategic-mapping module contributes to the
// shared persist path. The receiver outputs are acoustics.ReceiverOutput for
// every module in the family, so everything else — the hash payload, the
// receiver table, the run summary — is identical by construction, and only
// these two values differ.
type endPersistSpec struct {
	// modelVersion is the version the run summary names. An alias module names
	// the module whose numbers it publishes, not itself.
	modelVersion string

	// reportingPrecisionDB is stamped into the run summary when non-zero.
	// cnossos-industry, bub-industry and buf-aircraft leave it out, which is an
	// inconsistency this collapse preserved rather than fixed: the digest
	// goldens pin the run summary, so changing it here would be a behaviour
	// change smuggled inside a refactor. Tracked in PLAN.md Priority 7.
	reportingPrecisionDB float64

	// export is the module's own bundle writer. The layout is shared, but the
	// raster files are named after the standard that produced them, so an alias
	// module still writes its own.
	export func(baseDir string, outputs []acoustics.ReceiverOutput, gridWidth int, gridHeight int) (acoustics.ExportOutputs, error)
}

// endPersistSpecs holds one entry per module reporting the END day/evening/night
// indicator set. A standard missing from this map is a standard the END persist
// path cannot write, and says so rather than writing something plausible.
var endPersistSpecs = map[string]endPersistSpec{
	cnossosroad.StandardID: {
		modelVersion:         cnossosroad.BuiltinModelVersion,
		reportingPrecisionDB: cnossosroad.ReportingPrecisionDB,
		export:               cnossosroad.ExportResultBundle,
	},
	bubroad.StandardID: {
		modelVersion:         bubroad.BuiltinModelVersion,
		reportingPrecisionDB: bubroad.ReportingPrecisionDB,
		export:               bubroad.ExportResultBundle,
	},
	cnossosrail.StandardID: {
		modelVersion:         cnossosrail.BuiltinModelVersion,
		reportingPrecisionDB: cnossosrail.ReportingPrecisionDB,
		export:               cnossosrail.ExportResultBundle,
	},
	bubrail.StandardID: {
		modelVersion:         cnossosrail.BuiltinModelVersion,
		reportingPrecisionDB: cnossosrail.ReportingPrecisionDB,
		export:               bubrail.ExportResultBundle,
	},
	cnossosindustry.StandardID: {
		modelVersion: cnossosindustry.BuiltinModelVersion,
		export:       cnossosindustry.ExportResultBundle,
	},
	bubindustry.StandardID: {
		modelVersion: cnossosindustry.BuiltinModelVersion,
		export:       bubindustry.ExportResultBundle,
	},
	cnossosaircraft.StandardID: {
		modelVersion:         cnossosaircraft.BuiltinModelVersion,
		reportingPrecisionDB: cnossosaircraft.ReportingPrecisionDB,
		export:               cnossosaircraft.ExportResultBundle,
	},
	bufaircraft.StandardID: {
		modelVersion: bufaircraft.BuiltinModelVersion,
		export:       bufaircraft.ExportResultBundle,
	},
}

// persistENDRunOutputs writes one END strategic-mapping run: cnossos-road,
// -rail, -industry and -aircraft, their bub/buf counterparts, and anything else
// that reports Lden/Lnight/Lday/Levening.
//
// This replaces eight functions that were clones of one another — three of them
// close enough that dupl fired on them and carried a suppression. What made
// them clones was not the persist path but the indicator model: each module
// declared its own copy of the END types, so the same code had to be written
// once per type. They share acoustics.ReceiverOutput now, and one function
// serves them all.
func persistENDRunOutputs(
	standardID string,
	runDir string,
	outputs []acoustics.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
	tier framework.EvidenceTier,
) (persistedRunOutputs, string, time.Time, error) {
	const scope = "cli.persistENDRunOutputs"

	spec, ok := endPersistSpecs[standardID]
	if !ok {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, scope, "no END persist spec for standard "+standardID, nil)
	}

	plan := receiverPersistPlan[acoustics.ReceiverOutput]{
		scope:          scope,
		hashLabel:      standardID + " receiver outputs",
		hashErrMessage: "hash " + standardID + " outputs",
		modelVersion:   spec.modelVersion,
		indicatorOrder: acoustics.IndicatorOrder(),
		receiver:       func(output acoustics.ReceiverOutput) geo.PointReceiver { return output.Receiver },
		indicators:     func(output acoustics.ReceiverOutput) any { return output.Indicators },
		values:         func(output acoustics.ReceiverOutput) map[string]float64 { return output.Indicators.Values() },
		export:         exportBundle(scope, "export "+standardID+" results", spec.export, outputs, gridWidth, gridHeight),
	}

	if spec.reportingPrecisionDB != 0 {
		plan.decorateSummary = func(summary map[string]any) {
			summary["reporting_precision_db"] = spec.reportingPrecisionDB
		}
	}

	return persistReceiverRunOutputs(plan, runDir, outputs, gridWidth, gridHeight, sourceCount, receiverMode, tier)
}

// persistRLS19RoadRunOutputs writes an RLS-19 run. RLS-19 has no model
// version of its own: the data pack it evaluates is the only thing that
// versions its results.
func persistRLS19RoadRunOutputs(
	runDir string,
	outputs []rls19road.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	sourceOverrideCount int,
	receiverMode string,
	tier framework.EvidenceTier,
) (persistedRunOutputs, string, time.Time, error) {
	const scope = "cli.persistRLS19RoadRunOutputs"

	plan := receiverPersistPlan[rls19road.ReceiverOutput]{
		scope:          scope,
		hashLabel:      "RLS-19 road receiver outputs",
		hashErrMessage: "hash RLS-19 road outputs",
		modelVersion:   rls19road.BuiltinDataPackVersion,
		indicatorOrder: []string{rls19road.IndicatorLrDay, rls19road.IndicatorLrNight},
		decorateSummary: func(summary map[string]any) {
			summary["sources_with_feature_acoustics_overrides"] = sourceOverrideCount
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
		export: exportBundle(scope, "export RLS-19 road results", rls19road.ExportResultBundle, outputs, gridWidth, gridHeight),
	}

	return persistReceiverRunOutputs(plan, runDir, outputs, gridWidth, gridHeight, sourceCount, receiverMode, tier)
}

func persistSchall03RunOutputs(
	runDir string,
	outputs []schall03.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
	engine string,
	tier framework.EvidenceTier,
) (persistedRunOutputs, string, time.Time, error) {
	const scope = "cli.persistSchall03RunOutputs"

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
		export: exportBundle(scope, "export Schall 03 results", schall03.ExportResultBundle, outputs, gridWidth, gridHeight),
	}

	return persistReceiverRunOutputs(plan, runDir, outputs, gridWidth, gridHeight, sourceCount, receiverMode, tier)
}

func persistISO9613RunOutputs(
	runDir string,
	outputs []iso9613.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
	receiverMode string,
	tier framework.EvidenceTier,
) (persistedRunOutputs, string, time.Time, error) {
	const scope = "cli.persistISO9613RunOutputs"

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
		export: exportBundle(scope, "export iso9613 results", iso9613.ExportResultBundle, outputs, gridWidth, gridHeight),
	}

	return persistReceiverRunOutputs(plan, runDir, outputs, gridWidth, gridHeight, sourceCount, receiverMode, tier)
}

// exportedBundle is the shape every standards module's ExportResultBundle
// returns. The per-module structs are structurally identical but nominally
// distinct, so a persist path shared by an alias pair needs one type to hand
// back.
type exportedBundle struct {
	ReceiverJSONPath string
	ReceiverCSVPath  string
	RasterMetaPath   string
	RasterDataPath   string
}

func persistBEBExposureRunOutputs(
	runDir string,
	outputs []bebexposure.BuildingExposureOutput,
	summary bebexposure.Summary,
	sourceCount int,
	tier framework.EvidenceTier,
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

// hashedReceiverRecord is the unit of the run output-hash contract: one
// receiver's ID beside its indicator block. The JSON tags are part of that
// contract — changing them changes every output hash this repo has ever stored.
type hashedReceiverRecord[Indicators any] struct {
	ReceiverID string     `json:"receiver_id"`
	Indicators Indicators `json:"indicators"`
}

// hashedBuildingRecord is the BEB equivalent. Exposure is aggregated per
// building, so the key is a building ID rather than a receiver ID.
type hashedBuildingRecord[Indicators any] struct {
	BuildingID string     `json:"building_id"`
	Indicators Indicators `json:"indicators"`
}

// hashReceiverOutputs hashes one standard's receiver outputs. Every standard
// but BEB hashes the same shape over a different indicator type, so the only
// per-standard inputs are the two accessors and label, which names the payload
// in the error a marshal failure produces.
func hashReceiverOutputs[Output, Indicators any](
	label string,
	outputs []Output,
	receiverID func(Output) string,
	indicators func(Output) Indicators,
) (string, error) {
	records := make([]hashedReceiverRecord[Indicators], 0, len(outputs))
	for _, output := range outputs {
		records = append(records, hashedReceiverRecord[Indicators]{
			ReceiverID: receiverID(output),
			Indicators: indicators(output),
		})
	}

	return hashJSONPayload(label, records)
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

func hashJSONPayload(label string, payload any) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal %s: %w", label, err)
	}

	sum := sha256.Sum256(encoded)

	return hex.EncodeToString(sum[:]), nil
}

func buildRunArtifacts(projectRoot string, runID string, persisted persistedRunOutputs) []project.ArtifactRef {
	now := nowUTC()

	artifacts := make([]project.ArtifactRef, 0, 5)
	if persisted.ReceiverJSONPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-receivers-json", runID), RunID: runID, Kind: project.ArtifactKindRunResultReceiverTableJSON, Path: relativePath(projectRoot, persisted.ReceiverJSONPath), CreatedAt: now})
	}

	if persisted.ReceiverCSVPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-receivers-csv", runID), RunID: runID, Kind: project.ArtifactKindRunResultReceiverTableCSV, Path: relativePath(projectRoot, persisted.ReceiverCSVPath), CreatedAt: now})
	}

	if persisted.RasterMetadataPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-raster-meta", runID), RunID: runID, Kind: project.ArtifactKindRunResultRasterMetadata, Path: relativePath(projectRoot, persisted.RasterMetadataPath), CreatedAt: now})
	}

	if persisted.RasterDataPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-raster-data", runID), RunID: runID, Kind: project.ArtifactKindRunResultRasterBinary, Path: relativePath(projectRoot, persisted.RasterDataPath), CreatedAt: now})
	}

	if persisted.SummaryPath != "" {
		artifacts = append(artifacts, project.ArtifactRef{ID: fmt.Sprintf("artifact-run-%s-summary", runID), RunID: runID, Kind: project.ArtifactKindRunResultSummary, Path: relativePath(projectRoot, persisted.SummaryPath), CreatedAt: now})
	}

	return artifacts
}

func finalizeRunFailure(store projectfs.Store, run project.Run, logLines []string, runErr error) error {
	finishedAt := nowUTC()

	logLines = append(logLines, finishedAt.Format(time.RFC3339)+" run failed")

	err := finalizeRun(store, run, project.RunStatusFailed, finishedAt, logLines, nil)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.finalizeRunFailure", "finalize failed run", errors.Join(runErr, err))
	}

	return runErr
}

func finalizeRun(
	store projectfs.Store,
	run project.Run,
	status string,
	finishedAt time.Time,
	logLines []string,
	artifacts []project.ArtifactRef,
) error {
	if finishedAt.IsZero() {
		finishedAt = nowUTC()
	}

	proj, err := store.Load()
	if err != nil {
		return fmt.Errorf("load project manifest: %w", err)
	}

	foundRun := false

	for i := range proj.Runs {
		if proj.Runs[i].ID != run.ID {
			continue
		}

		proj.Runs[i].Status = status
		proj.Runs[i].FinishedAt = finishedAt
		foundRun = true

		break
	}

	if !foundRun {
		return domainerrors.New(domainerrors.KindInternal, "cli.finalizeRun", fmt.Sprintf("run %s not found in project manifest", run.ID), nil)
	}

	for _, artifact := range artifacts {
		proj.Artifacts = upsertArtifact(proj.Artifacts, artifact)
	}

	err = store.Save(proj)
	if err != nil {
		return fmt.Errorf("save project manifest: %w", err)
	}

	if len(logLines) == 0 {
		logLines = []string{fmt.Sprintf("%s run finalized with status=%s", finishedAt.Format(time.RFC3339), status)}
	}

	logContent := strings.Join(logLines, "\n") + "\n"

	logPath := filepath.Join(store.Root(), filepath.FromSlash(run.LogPath))

	err = os.WriteFile(logPath, []byte(logContent), 0o600)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.finalizeRun", "write run log "+logPath, err)
	}

	return nil
}

// receiverGridCenter computes the centroid of a set of receivers.
func receiverGridCenter(receivers []geo.PointReceiver) (float64, float64) {
	if len(receivers) == 0 {
		return 0, 0
	}

	var sumX, sumY float64

	for _, r := range receivers {
		sumX += r.Point.X
		sumY += r.Point.Y
	}

	n := float64(len(receivers))

	return sumX / n, sumY / n
}

// findArtifactPath returns the path for the artifact with the given ID, or empty string if not found.
func findArtifactPath(proj project.Project, id string) string {
	for _, a := range proj.Artifacts {
		if a.ID == id {
			return a.Path
		}
	}

	return ""
}

// terrainElevationAt queries the terrain model for elevation at (x, y).
// Returns 0 if terrain is nil or the point is outside bounds.
func terrainElevationAt(tm terrain.Model, x, y float64) float64 {
	if tm == nil {
		return 0
	}

	elev, ok := tm.ElevationAt(x, y)
	if !ok {
		return 0
	}

	return elev
}
