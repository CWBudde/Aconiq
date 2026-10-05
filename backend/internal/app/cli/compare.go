package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/io/soundplanimport"
	"github.com/aconiq/backend/internal/results"
	"github.com/aconiq/backend/internal/standards/schall03"
	"github.com/spf13/cobra"
)

const (
	// commandNameCompare is the cobra command name; it is also written to the
	// `command` field of the JSON output and of the compare report.
	commandNameCompare = "compare"

	defaultCompareReportPath = ".noise/artifacts/soundplan-receiver-compare.json"
	defaultReceiverMatchTolM = 0.5
)

type compareIndicatorStats struct {
	MeanAbsDeltaDB     float64 `json:"mean_abs_delta_db"`
	MaxAbsDeltaDB      float64 `json:"max_abs_delta_db"`
	P95AbsDeltaDB      float64 `json:"p95_abs_delta_db"`
	ToleranceExceeding int     `json:"tolerance_exceeding"`
	Count              int     `json:"count"`
}

type soundPlanReceiverComparisonRecord struct {
	AconiqID       string  `json:"aconiq_id"`
	SoundPlanRecNo int32   `json:"soundplan_rec_no"`
	SoundPlanObjID int64   `json:"soundplan_obj_id"`
	SoundPlanFloor int32   `json:"soundplan_floor"`
	SoundPlanName  string  `json:"soundplan_name,omitempty"`
	MatchStrategy  string  `json:"match_strategy"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	DistanceM      float64 `json:"distance_m"`
	AconiqLrDay    float64 `json:"aconiq_lr_day"`
	SoundPlanZB1   float64 `json:"soundplan_zb1"`
	DeltaDayDB     float64 `json:"delta_day_db"`
	AconiqLrNight  float64 `json:"aconiq_lr_night"`
	SoundPlanZB2   float64 `json:"soundplan_zb2"`
	DeltaNightDB   float64 `json:"delta_night_db"`
}

type soundPlanRasterCompareReport struct {
	Status    string `json:"status"`
	Alignment string `json:"alignment,omitempty"`
	// CalcAreaSource names which of the project's two calculation areas the
	// synthesis used and CalcAreaRole what it did there, because the
	// GM-metadata path consults the area only for the row direction.
	// CalcAreaBoundsDelta measures how far apart the two envelopes are, in
	// CalcAreaBoundsDeltaUnit, and is nil when there was no second area to
	// compare against. A warning fires only when the vertex counts disagree, so
	// without these fields the artifact would not record which area produced it.
	CalcAreaSource          string   `json:"calc_area_source,omitempty"`
	CalcAreaRole            string   `json:"calc_area_role,omitempty"`
	CalcAreaBoundsDelta     *float64 `json:"calc_area_bounds_delta,omitempty"`
	CalcAreaBoundsDeltaUnit string   `json:"calc_area_bounds_delta_unit,omitempty"`
	GridResolutionM         float64  `json:"grid_resolution_m,omitempty"`
	ReceiverHeightM         float64  `json:"receiver_height_m,omitempty"`
	SyntheticReceiverCount  int      `json:"synthetic_receiver_count,omitempty"`
	ArtifactPath            string   `json:"artifact_path,omitempty"`
	// SoundPlanRasterRun names the one grid map the raster deltas were computed
	// against, out of SoundPlanRasterRunCandidates and on the grounds
	// SoundPlanRasterRunSelection records. SoundPlanRuns stays the full list of
	// grid maps the bundle holds, which is what it always meant — the number of
	// runs discovered, not the number compared.
	SoundPlanRasterRun           string                             `json:"soundplan_raster_run,omitempty"`
	SoundPlanRasterRunCandidates []string                           `json:"soundplan_raster_run_candidates,omitempty"`
	SoundPlanRasterRunSelection  string                             `json:"soundplan_raster_run_selection,omitempty"`
	SoundPlanRuns                []soundplanimport.GridMapMetadata  `json:"soundplan_runs,omitempty"`
	Runs                         []soundPlanRasterRunCompareSummary `json:"runs,omitempty"`
	Warnings                     []string                           `json:"warnings,omitempty"`
}

type soundPlanCompareReport struct {
	Command         string `json:"command"`
	StandardID      string `json:"standard_id"`
	StandardVersion string `json:"standard_version,omitempty"`
	StandardProfile string `json:"standard_profile,omitempty"`
	RunID           string `json:"run_id"`
	SoundPlanSource string `json:"soundplan_source"`
	// SoundPlanResultRun names exactly one SoundPLAN result directory. The
	// candidates it was chosen from and the grounds for the choice are
	// recorded alongside it, because "which scenario was this compared
	// against" is not answerable from the levels.
	SoundPlanResultRun           string   `json:"soundplan_result_run"`
	SoundPlanResultRunCandidates []string `json:"soundplan_result_run_candidates,omitempty"`
	SoundPlanResultRunSelection  string   `json:"soundplan_result_run_selection"`
	// ReceiverMatchTolM is an assertion threshold for keyed matches and a
	// search bound for the coordinate fallback; see matchSoundPlanReceivers.
	ReceiverMatchTolM    float64        `json:"receiver_match_tolerance_m"`
	ToleranceDB          float64        `json:"tolerance_db"`
	MatchedReceiverCount int            `json:"matched_receiver_count"`
	MatchStrategyCounts  map[string]int `json:"match_strategy_counts"`
	MaxMatchDistanceM    float64        `json:"max_match_distance_m"`
	UnmatchedAconiqCount int            `json:"unmatched_aconiq_count"`
	UnmatchedSPCount     int            `json:"unmatched_soundplan_count"`
	// UnmatchedAconiq and UnmatchedSoundPlan name every receiver that was not
	// paired. Counts alone let an unmatched side be read as a rounding
	// difference rather than as the missing half of the evidence.
	UnmatchedAconiq    []string `json:"unmatched_aconiq,omitempty"`
	UnmatchedSoundPlan []string `json:"unmatched_soundplan,omitempty"`
	// StatsScope states what Stats aggregates over, so a mean cannot be read
	// as covering receivers that were never compared.
	StatsScope string                              `json:"stats_scope"`
	Warnings   []string                            `json:"warnings,omitempty"`
	Raster     *soundPlanRasterCompareReport       `json:"raster,omitempty"`
	Stats      map[string]compareIndicatorStats    `json:"stats"`
	Records    []soundPlanReceiverComparisonRecord `json:"records"`
}

// statsScopeMatchedOnly is the only scope the aggregates have ever had; naming
// it makes that visible in the artifact.
const statsScopeMatchedOnly = "matched_receivers_only"

func newCompareCommand() *cobra.Command {
	var (
		standardID       string
		standardVersion  string
		standardProfile  string
		modelPath        string
		scenarioID       string
		toleranceDB      float64
		soundPlanRun     string
		soundPlanGridRun string
		rawParams        []string
	)

	cmd := &cobra.Command{
		Use:   commandNameCompare,
		Short: "Run a comparison against imported SoundPLAN receiver results",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCompare(cmd, compareRequest{
				standardID:       standardID,
				standardVersion:  standardVersion,
				standardProfile:  standardProfile,
				modelPath:        modelPath,
				scenarioID:       scenarioID,
				toleranceDB:      toleranceDB,
				soundPlanRun:     soundPlanRun,
				soundPlanGridRun: soundPlanGridRun,
				rawParams:        rawParams,
			})
		},
	}

	cmd.Flags().StringVar(&scenarioID, "scenario", "default", "Scenario ID")
	cmd.Flags().StringVar(&standardID, "standard", schall03.StandardID, "Standard identifier")
	cmd.Flags().StringVar(&standardVersion, "standard-version", "", "Standard version (defaults to standard default)")
	cmd.Flags().StringVar(&standardProfile, "standard-profile", "", "Standard profile (defaults to version profile default)")
	cmd.Flags().StringVar(&modelPath, "model", defaultModelPath, "Path to normalized GeoJSON model")
	cmd.Flags().Float64Var(&toleranceDB, "tolerance-db", 0.5, "Absolute delta threshold for tolerance exceedance counting")
	cmd.Flags().StringVar(&soundPlanRun, "soundplan-run", "", "SoundPLAN receiver result run directory to compare against (default: inferred from the geometry each run used)")
	// A separate flag rather than an overload of --soundplan-run: the receiver
	// results and the grid maps live in different result directories, and one
	// value could never name both.
	cmd.Flags().StringVar(&soundPlanGridRun, "soundplan-grid-run", "", "SoundPLAN grid-map run directory to compare the raster against (default: inferred from the geometry and grid height each run used)")
	cmd.Flags().StringArrayVar(&rawParams, "param", nil, "Run parameter as key=value, repeatable; forwarded to the underlying run")

	return cmd
}

// compareRequest carries the compare command's flags.
type compareRequest struct {
	standardID       string
	standardVersion  string
	standardProfile  string
	modelPath        string
	scenarioID       string
	toleranceDB      float64
	soundPlanRun     string
	soundPlanGridRun string
	rawParams        []string
}

// validateCompareFlags rejects compare flag values the command cannot honour.
func validateCompareFlags(standardID string, toleranceDB float64) error {
	if strings.TrimSpace(standardID) != schall03.StandardID {
		return domainerrors.New(domainerrors.KindUserInput, "cli.compare", "compare currently supports only schall03", nil)
	}

	if toleranceDB < 0 || math.IsNaN(toleranceDB) || math.IsInf(toleranceDB, 0) {
		return domainerrors.New(domainerrors.KindUserInput, "cli.compare", "--tolerance-db must be finite and >= 0", nil)
	}

	return nil
}

// compareInputs bundles the SoundPLAN reference data a comparison run needs.
type compareInputs struct {
	importReport soundPlanImportReport
	resultRun    soundPlanResultRunSelection
	receivers    []soundplanimport.ReceiverResult
}

// loadCompareInputs reads the receiver results of the one result run the
// comparison was resolved to.
//
// The barrier count comes from model, not from importReport: the report
// records what the import produced, and the comparison computes whatever
// --model points at. Those are the same file by default and need not be — a
// model edited since the import, or a different one named on the flag, would
// otherwise pick the reference scenario on a stale count and compare against
// the wrong side of a noise barrier worth up to 8 dB.
func loadCompareInputs(
	root string,
	explicitRun string,
	importReport soundPlanImportReport,
	model modelgeojson.Model,
) (compareInputs, error) {
	soundPlanRoot := resolvePath(root, importReport.SourcePath)

	resultRun, err := selectSoundPlanReceiverResultDir(soundPlanRoot, explicitRun, modelHasBarriers(model))
	if err != nil {
		return compareInputs{}, err
	}

	receivers, err := loadSoundPlanReceiverResults(soundPlanRoot, resultRun.Dir)
	if err != nil {
		return compareInputs{}, err
	}

	return compareInputs{
		importReport: importReport,
		resultRun:    resultRun,
		receivers:    receivers,
	}, nil
}

// compareRunModelPath selects the model the comparison run should evaluate: the
// temporary model carrying synthetic raster receivers when one was prepared.
func compareRunModelPath(root string, modelPath string, prep *rasterComparePreparation, hasPrep bool) string {
	if hasPrep && strings.TrimSpace(prep.tempModelPath) != "" {
		return relativePath(root, prep.tempModelPath)
	}

	return modelPath
}

// modelHasBarriers reports whether the model carries any barrier feature,
// which is the geometry the reference run is selected on.
func modelHasBarriers(model modelgeojson.Model) bool {
	return slices.ContainsFunc(model.Features, func(feature modelgeojson.Feature) bool {
		return feature.Kind == modelgeojson.FeatureKindBarrier
	})
}

// buildCompareReport reads the finished run's receiver table, compares it
// against the SoundPLAN reference run, and attaches the raster section.
func buildCompareReport(
	root string,
	req compareRequest,
	inputs compareInputs,
	receiverKeys map[string]soundPlanReceiverKey,
	rasterPrep *rasterComparePreparation,
	runID string,
) (soundPlanCompareReport, error) {
	receiverTable, err := results.LoadReceiverTableJSON(filepath.Join(root, ".noise", "runs", runID, "results", "receivers.json"))
	if err != nil {
		return soundPlanCompareReport{}, domainerrors.New(domainerrors.KindInternal, "cli.compare", "load run receiver outputs", err)
	}

	report, err := compareSoundPlanReceiverTables(compareReceiverTablesInput{
		table:           filterOutSyntheticRasterReceivers(receiverTable),
		keys:            receiverKeys,
		soundPlan:       inputs.receivers,
		toleranceDB:     req.toleranceDB,
		soundPlanSource: inputs.importReport.SourcePath,
		resultRun:       inputs.resultRun,
		runID:           runID,
		standardID:      req.standardID,
		standardVersion: req.standardVersion,
		standardProfile: req.standardProfile,
	})
	if err != nil {
		return soundPlanCompareReport{}, err
	}

	report.Raster, _, err = finalizeSoundPlanRasterCompare(root, rasterPrep, receiverTable, req.toleranceDB)
	if err != nil {
		return soundPlanCompareReport{}, err
	}

	if report.Raster == nil {
		report.Raster = buildSoundPlanRasterCompareReport(inputs.importReport)
	}

	return report, nil
}

func runCompare(cmd *cobra.Command, req compareRequest) error {
	if err := validateCompareFlags(req.standardID, req.toleranceDB); err != nil {
		return err
	}

	state, ok := stateFromCommand(cmd)
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, "cli.compare", "command state unavailable", nil)
	}

	store, err := projectfs.New(state.Config.ProjectPath)
	if err != nil {
		return fmt.Errorf("open project %s: %w", state.Config.ProjectPath, err)
	}

	importReport, err := loadSoundPlanImportReport(store.Root())
	if err != nil {
		return err
	}

	// Read once, here, and used for both the reference-run choice and the
	// receiver keys. It is deliberately the *original* model rather than the
	// temporary one the raster comparison hands the run: that copy carries
	// thousands of synthetic raster receivers, which have no SoundPLAN
	// identity and are filtered out of the receiver comparison anyway.
	model, err := loadValidatedModel(resolvePath(store.Root(), req.modelPath), importReport.ProjectCRS, req.modelPath)
	if err != nil {
		return err
	}

	inputs, err := loadCompareInputs(store.Root(), req.soundPlanRun, importReport, model)
	if err != nil {
		return err
	}

	receiverKeys := soundPlanReceiverKeysFromModel(model)

	rasterPrep, hasRasterPrep, err := prepareSoundPlanRasterCompare(store.Root(), importReport, req.modelPath, req.soundPlanGridRun)
	if err != nil {
		return err
	}

	if hasRasterPrep {
		defer cleanupRasterComparePreparation(rasterPrep)
	}

	runModelPath := compareRunModelPath(store.Root(), req.modelPath, rasterPrep, hasRasterPrep)

	run, err := executeCompareRun(cmd, runCommandRequest{
		scenarioID:      req.scenarioID,
		standardID:      req.standardID,
		standardVersion: req.standardVersion,
		standardProfile: req.standardProfile,
		modelPath:       runModelPath,
		receiverMode:    receiverModeCustom,
		rawParams:       req.rawParams,
	})
	if err != nil {
		return err
	}

	report, err := buildCompareReport(store.Root(), req, inputs, receiverKeys, rasterPrep, run.ID)
	if err != nil {
		return err
	}

	if err := persistCompareReport(store, report); err != nil {
		return err
	}

	if state.Config.JSONLogs {
		return writeCompareJSONOutput(cmd, run.ID, report)
	}

	printCompareSummary(cmd, run.ID, report)

	return nil
}

// persistCompareReport writes the comparison report and registers the resulting
// artifacts in the project manifest.
func persistCompareReport(store projectfs.Store, report soundPlanCompareReport) error {
	reportPath := filepath.Join(store.Root(), filepath.FromSlash(defaultCompareReportPath))
	if err := writeJSONFile(reportPath, report); err != nil {
		return err
	}

	proj, err := store.Load()
	if err != nil {
		return fmt.Errorf("load project manifest: %w", err)
	}

	proj.Artifacts = upsertArtifact(proj.Artifacts, project.ArtifactRef{
		ID:        "artifact-soundplan-compare",
		Kind:      "comparison.soundplan_receivers",
		Path:      defaultCompareReportPath,
		CreatedAt: nowUTC(),
	})
	if report.Raster != nil && strings.TrimSpace(report.Raster.ArtifactPath) != "" {
		proj.Artifacts = upsertArtifact(proj.Artifacts, project.ArtifactRef{
			ID:        "artifact-soundplan-raster-compare",
			Kind:      "comparison.soundplan_raster",
			Path:      report.Raster.ArtifactPath,
			CreatedAt: nowUTC(),
		})
	}

	if err := store.Save(proj); err != nil {
		return fmt.Errorf("save project manifest: %w", err)
	}

	return nil
}

func executeCompareRun(parent *cobra.Command, req runCommandRequest) (project.Run, error) {
	runCmd := &cobra.Command{}
	runCmd.SetContext(parent.Root().Context())
	runCmd.SetOut(io.Discard)

	if err := executeRunCommand(runCmd, req); err != nil {
		return project.Run{}, err
	}

	state, _ := stateFromCommand(parent)

	store, err := projectfs.New(state.Config.ProjectPath)
	if err != nil {
		return project.Run{}, fmt.Errorf("open project %s: %w", state.Config.ProjectPath, err)
	}

	proj, err := store.Load()
	if err != nil {
		return project.Run{}, fmt.Errorf("load project manifest: %w", err)
	}

	run, ok := latestRun(proj.Runs)
	if !ok {
		return project.Run{}, domainerrors.New(domainerrors.KindInternal, "cli.compare", "run completed but no latest run found", nil)
	}

	return run, nil
}

func loadSoundPlanImportReport(root string) (soundPlanImportReport, error) {
	payload, err := os.ReadFile(filepath.Join(root, ".noise", "model", "soundplan-import-report.json"))
	if err != nil {
		return soundPlanImportReport{}, domainerrors.New(domainerrors.KindUserInput, "cli.compare", "read SoundPLAN import report", err)
	}

	var report soundPlanImportReport
	if err := json.Unmarshal(payload, &report); err != nil {
		return soundPlanImportReport{}, domainerrors.New(domainerrors.KindInternal, "cli.compare", "decode SoundPLAN import report", err)
	}

	if strings.TrimSpace(report.SourcePath) == "" {
		return soundPlanImportReport{}, domainerrors.New(domainerrors.KindValidation, "cli.compare", "SoundPLAN import report is missing source_path", nil)
	}

	return report, nil
}

func buildSoundPlanRasterCompareReport(importReport soundPlanImportReport) *soundPlanRasterCompareReport {
	if len(importReport.GridMaps) == 0 {
		return nil
	}

	status := "metadata_only"
	warning := "RRLK*.GM files are currently decoded as metadata only; raster values, origin, spacing alignment, and active-cell masks are not compared yet."

	for _, item := range importReport.GridMaps {
		if item.DecodedValues && item.ActiveCellCount > 0 {
			status = "parsed_values_unaligned"
			warning = "RRLK*.GM values and row spans are decoded, but raster deltas are still blocked on spatial origin/alignment against the Aconiq run grid."

			break
		}
	}

	report := &soundPlanRasterCompareReport{
		Status:          status,
		GridResolutionM: importReport.GridResolutionM,
		SoundPlanRuns:   append([]soundplanimport.GridMapMetadata(nil), importReport.GridMaps...),
		Warnings:        []string{warning},
	}

	return report
}

func compareRasterStatus(report *soundPlanRasterCompareReport) string {
	if report == nil {
		return ""
	}

	return report.Status
}

func loadSoundPlanReceiverResults(soundPlanRoot string, resultRunDir string) ([]soundplanimport.ReceiverResult, error) {
	suffix := compareExtractRunSuffix(resultRunDir)
	path := filepath.Join(soundPlanRoot, resultRunDir, "RREC"+suffix+".abs")

	rows, err := soundplanimport.ParseReceiverResults(path)
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.compare", "read SoundPLAN receiver results", err)
	}

	return rows, nil
}

func compareExtractRunSuffix(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] < '0' || name[i] > '9' {
			return name[i+1:]
		}
	}

	return name
}

func compareFileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !info.IsDir()
}
