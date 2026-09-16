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
	"github.com/aconiq/backend/internal/report/results"
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

// writeCompareJSONOutput emits the machine-readable compare summary.
func writeCompareJSONOutput(cmd *cobra.Command, runID string, report soundPlanCompareReport) error {
	rasterArtifactPath := ""
	rasterRunCount := 0
	rasterRun := ""
	rasterRunSelection := ""

	if report.Raster != nil {
		rasterArtifactPath = report.Raster.ArtifactPath
		// The count of grid maps discovered, which is what this field has always
		// meant. Exactly one of them is compared; which one, and why, is reported
		// beside it rather than by shrinking this number to 1.
		rasterRunCount = len(report.Raster.SoundPlanRuns)
		rasterRun = report.Raster.SoundPlanRasterRun
		rasterRunSelection = report.Raster.SoundPlanRasterRunSelection
	}

	return writeCommandOutput(cmd.OutOrStdout(), true, map[string]any{
		"command":                        commandNameCompare,
		"report_path":                    defaultCompareReportPath,
		"run_id":                         runID,
		"matched_receiver_count":         report.MatchedReceiverCount,
		"match_strategy_counts":          report.MatchStrategyCounts,
		"max_match_distance_m":           report.MaxMatchDistanceM,
		"unmatched_aconiq_count":         report.UnmatchedAconiqCount,
		"unmatched_soundplan_count":      report.UnmatchedSPCount,
		"unmatched_aconiq":               report.UnmatchedAconiq,
		"unmatched_soundplan":            report.UnmatchedSoundPlan,
		"soundplan_result_run":           report.SoundPlanResultRun,
		"soundplan_result_run_selection": report.SoundPlanResultRunSelection,
		"stats_scope":                    report.StatsScope,
		outputFieldWarnings:              report.Warnings,
		"raster_status":                  compareRasterStatus(report.Raster),
		"raster_artifact_path":           rasterArtifactPath,
		"soundplan_raster_run_count":     rasterRunCount,
		"soundplan_raster_run":           rasterRun,
		"soundplan_raster_run_selection": rasterRunSelection,
	})
}

// maxPrintedUnmatchedReceivers caps the unmatched receivers named on stdout.
// The report names every one of them.
const maxPrintedUnmatchedReceivers = 5

// printCompareSummary writes the human-readable compare summary.
//
// It prints how the receivers were matched and how far apart the matched pairs
// are, because that is what the previous version of this command got wrong
// without anyone noticing: all 60 of its pairs were positional, and the only
// place that said so was a field inside the artifact.
func printCompareSummary(cmd *cobra.Command, runID string, report soundPlanCompareReport) {
	out := cmd.OutOrStdout()

	_, _ = fmt.Fprintf(out, "Compared run %s against SoundPLAN %s (%s)\n", runID, report.SoundPlanResultRun, report.SoundPlanResultRunSelection)
	_, _ = fmt.Fprintf(out, "Matched receivers: %d [%s]\n", report.MatchedReceiverCount, formatMatchStrategyCounts(report.MatchStrategyCounts))
	_, _ = fmt.Fprintf(out, "Max match distance: %.4f m (tolerance %.2f m)\n", report.MaxMatchDistanceM, report.ReceiverMatchTolM)

	printUnmatchedReceivers(cmd, "Aconiq", report.UnmatchedAconiqCount, report.UnmatchedAconiq)
	printUnmatchedReceivers(cmd, "SoundPLAN", report.UnmatchedSPCount, report.UnmatchedSoundPlan)

	for _, warning := range report.Warnings {
		_, _ = fmt.Fprintf(out, "Warning: %s\n", warning)
	}

	_, _ = fmt.Fprintf(out, "Report: %s\n", defaultCompareReportPath)

	if report.Raster != nil {
		_, _ = fmt.Fprintf(out, "Raster coverage: %s (%d SoundPLAN grid-map runs discovered)\n", report.Raster.Status, len(report.Raster.SoundPlanRuns))

		if report.Raster.SoundPlanRasterRun != "" {
			_, _ = fmt.Fprintf(out, "Raster compared against SoundPLAN %s (%s)\n", report.Raster.SoundPlanRasterRun, report.Raster.SoundPlanRasterRunSelection)
		}
	}

	for _, indicator := range []string{schall03.IndicatorLrDay, schall03.IndicatorLrNight} {
		stats, ok := report.Stats[indicator]
		if !ok {
			continue
		}

		_, _ = fmt.Fprintf(out, "%s mean_abs=%.3f max_abs=%.3f p95_abs=%.3f exceedances=%d (over %d matched receivers)\n",
			indicator, stats.MeanAbsDeltaDB, stats.MaxAbsDeltaDB, stats.P95AbsDeltaDB, stats.ToleranceExceeding, stats.Count)
	}
}

// formatMatchStrategyCounts renders the strategy histogram in a fixed order so
// the line is reproducible.
func formatMatchStrategyCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "none"
	}

	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}

	slices.Sort(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s=%d", name, counts[name]))
	}

	return strings.Join(parts, " ")
}

func printUnmatchedReceivers(cmd *cobra.Command, side string, count int, names []string) {
	out := cmd.OutOrStdout()

	_, _ = fmt.Fprintf(out, "Unmatched %s receivers: %d\n", side, count)

	for i, name := range names {
		if i >= maxPrintedUnmatchedReceivers {
			_, _ = fmt.Fprintf(out, "  ... and %d more\n", len(names)-maxPrintedUnmatchedReceivers)

			break
		}

		_, _ = fmt.Fprintf(out, "  - %s\n", name)
	}
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

// Receiver match strategies, as recorded per record and counted in
// match_strategy_counts.
//
// There used to be a third, `ordinal`, which paired an Aconiq receiver with
// whatever SoundPLAN row happened to sit at the same position in file order.
// It is deleted rather than demoted: it is not a weaker match, it is a
// fabricated one, and every way of keeping it — behind a flag, excluded from
// the statistics — still ends in an artifact full of pairs that were never
// pairs. A receiver that cannot be matched is reported unmatched, by name.
const (
	matchStrategyKey         = "soundplan_key"
	matchStrategyCoordinates = "coordinates"
)

// soundPlanReceiverKey identifies one SoundPLAN receiver: the immission point
// it belongs to and which of that point's floors it is. It is the key
// RREC*.abs itself is indexed by, and with it the correspondence between the
// two receiver sets is a bijection by construction, so no assignment algorithm
// is needed or wanted.
type soundPlanReceiverKey struct {
	ObjID int64
	Floor int
}

// soundPlanReceiverMatch pairs one Aconiq receiver with one SoundPLAN row.
type soundPlanReceiverMatch struct {
	AconiqIndex    int
	SoundPlanIndex int
	Strategy       string
	DistanceM      float64
}

// soundPlanMatchResult is everything the matcher decided, including what it
// refused to decide.
type soundPlanMatchResult struct {
	Matches            []soundPlanReceiverMatch
	UnmatchedAconiq    []string
	UnmatchedSoundPlan []string
	Warnings           []string
	StrategyCounts     map[string]int
	MaxDistanceM       float64
}

// matchSoundPlanReceivers pairs the Aconiq receiver table with the SoundPLAN
// receiver rows.
//
// Receivers that carry a SoundPLAN key — which every receiver an
// `--from-soundplan` import produced does — are matched on it alone. The
// coordinate distance of such a pair is recorded and asserted against tolM,
// not searched over: it exists to catch a key that points at the wrong place,
// and in the reference project every pair agrees to within 2.3e-6 m.
//
// Receivers with no key at all — a hand-built receiver set compared against a
// SoundPLAN run — fall back to coordinates. That fallback is a *mutual*
// nearest-neighbour within tolM: a pair is formed only when each side is the
// other's nearest, which is what makes the result independent of the order the
// two inputs arrive in. The previous greedy first-come matcher was not.
//
// Two things are refused rather than resolved, because no answer would be the
// right one: a receiver carrying a SoundPLAN identity with floor 0, which the
// import emits for an immission point it could not expand, and two receivers
// carrying the same key.
func matchSoundPlanReceivers(
	table results.ReceiverTable,
	keys map[string]soundPlanReceiverKey,
	soundPlan []soundplanimport.ReceiverResult,
	tolM float64,
) (soundPlanMatchResult, error) {
	byKey, err := indexSoundPlanReceiversByKey(soundPlan)
	if err != nil {
		return soundPlanMatchResult{}, err
	}

	out := soundPlanMatchResult{
		Matches:        make([]soundPlanReceiverMatch, 0, len(table.Records)),
		StrategyCounts: map[string]int{},
	}

	used := make([]bool, len(soundPlan))

	unkeyed, err := matchSoundPlanReceiversByKey(table, keys, soundPlan, byKey, used, &out)
	if err != nil {
		return soundPlanMatchResult{}, err
	}

	coordinateMatches := matchSoundPlanReceiversByCoordinates(table, unkeyed, soundPlan, used, tolM)
	for _, match := range coordinateMatches {
		used[match.SoundPlanIndex] = true

		out.Matches = append(out.Matches, match)
	}

	slices.SortFunc(out.Matches, func(a, b soundPlanReceiverMatch) int {
		return a.AconiqIndex - b.AconiqIndex
	})

	matchedAconiq := make(map[int]struct{}, len(out.Matches))

	for _, match := range out.Matches {
		matchedAconiq[match.AconiqIndex] = struct{}{}
		out.StrategyCounts[match.Strategy]++
		out.MaxDistanceM = math.Max(out.MaxDistanceM, match.DistanceM)

		if match.DistanceM > tolM {
			out.Warnings = append(out.Warnings, fmt.Sprintf(
				"receiver %s is %.3f m from the SoundPLAN row it is keyed to, beyond the %.3f m assertion tolerance",
				table.Records[match.AconiqIndex].ID, match.DistanceM, tolM,
			))
		}
	}

	for _, recordIndex := range unkeyed {
		if _, ok := matchedAconiq[recordIndex]; ok {
			continue
		}

		out.UnmatchedAconiq = append(out.UnmatchedAconiq, describeAconiqReceiver(table.Records[recordIndex], soundPlanReceiverKey{}))
	}

	for i, row := range soundPlan {
		if !used[i] {
			out.UnmatchedSoundPlan = append(out.UnmatchedSoundPlan, describeSoundPlanRow(row))
		}
	}

	return out, nil
}

// indexSoundPlanReceiversByKey indexes the reference rows by (ObjID, Floor).
//
// A duplicate key is a hard error, not something to resolve quietly: it means
// the row set spans more than one scenario — exactly what reading every RSPS*
// directory into one pool used to produce — and no answer the matcher could
// give would be the right one.
func indexSoundPlanReceiversByKey(soundPlan []soundplanimport.ReceiverResult) (map[soundPlanReceiverKey]int, error) {
	byKey := make(map[soundPlanReceiverKey]int, len(soundPlan))

	for i, row := range soundPlan {
		key := soundPlanReceiverKey{ObjID: int64(row.ObjID), Floor: int(row.Floor)}
		if _, exists := byKey[key]; exists {
			return nil, domainerrors.New(
				domainerrors.KindValidation, "cli.compare",
				fmt.Sprintf("SoundPLAN receiver results contain %s twice; the result set spans more than one calculation run", describeSoundPlanRow(row)),
				nil,
			)
		}

		byKey[key] = i
	}

	return byKey, nil
}

// matchSoundPlanReceiversByKey runs the keyed pass and returns the indices of
// the records that carry no SoundPLAN key at all, for the coordinate fallback.
//
// It appends matches and unmatched Aconiq receivers to out and marks the rows
// it consumed in used, both of which the caller owns.
func matchSoundPlanReceiversByKey(
	table results.ReceiverTable,
	keys map[string]soundPlanReceiverKey,
	soundPlan []soundplanimport.ReceiverResult,
	byKey map[soundPlanReceiverKey]int,
	used []bool,
	out *soundPlanMatchResult,
) ([]int, error) {
	unkeyed := make([]int, 0, len(table.Records))
	claimedBy := make(map[soundPlanReceiverKey]string, len(table.Records))

	for recordIndex, record := range table.Records {
		key, hasKey := keys[record.ID]
		if !hasKey {
			unkeyed = append(unkeyed, recordIndex)

			continue
		}

		// A SoundPLAN immission point whose floor attributes could not be
		// decoded becomes one receiver at the project default height carrying
		// floor 0, and docs/geojson-schema-v1.md says it has no usable key and
		// will not match. It is failed here rather than left to the coordinate
		// fallback: every row of that point's column shares the immission
		// point's X/Y, so a nearest-neighbour search would pair a guessed
		// height against whichever floor happens to sort first.
		if key.Floor <= 0 {
			out.UnmatchedAconiq = append(out.UnmatchedAconiq, describeAconiqReceiver(record, key))

			continue
		}

		// Two receivers claiming one reference row is the mirror of the
		// duplicate indexSoundPlanReceiversByKey rejects, and it is worse
		// undetected: both would be appended against the same row, so one
		// reference receiver would be counted twice in every aggregate while
		// the report showed no unmatched SoundPLAN row to say so.
		if first, claimed := claimedBy[key]; claimed {
			return nil, domainerrors.New(
				domainerrors.KindValidation, "cli.compare",
				fmt.Sprintf(
					"receivers %s and %s both carry soundplan_obj_id %d floor %d; one reference receiver cannot stand for two",
					first, record.ID, key.ObjID, key.Floor,
				),
				nil,
			)
		}

		claimedBy[key] = record.ID

		soundPlanIndex, found := byKey[key]
		if !found {
			out.UnmatchedAconiq = append(out.UnmatchedAconiq, describeAconiqReceiver(record, key))

			continue
		}

		used[soundPlanIndex] = true

		out.Matches = append(out.Matches, soundPlanReceiverMatch{
			AconiqIndex:    recordIndex,
			SoundPlanIndex: soundPlanIndex,
			Strategy:       matchStrategyKey,
			DistanceM:      soundPlanMatchDistance(record, soundPlan[soundPlanIndex]),
		})
	}

	return unkeyed, nil
}

// matchSoundPlanReceiversByCoordinates pairs the receivers that carry no
// SoundPLAN key with the rows still unused, by mutual nearest neighbour within
// tolM. O(n·m) and order-independent; ties fall to the lower index on both
// sides, so the result is fully determined by the inputs.
func matchSoundPlanReceiversByCoordinates(
	table results.ReceiverTable,
	unkeyed []int,
	soundPlan []soundplanimport.ReceiverResult,
	used []bool,
	tolM float64,
) []soundPlanReceiverMatch {
	if len(unkeyed) == 0 {
		return nil
	}

	nearestRow := make(map[int]int, len(unkeyed))

	for _, recordIndex := range unkeyed {
		if rowIndex, ok := nearestSoundPlanRow(table.Records[recordIndex], soundPlan, used, tolM); ok {
			nearestRow[recordIndex] = rowIndex
		}
	}

	matches := make([]soundPlanReceiverMatch, 0, len(nearestRow))

	for _, recordIndex := range unkeyed {
		rowIndex, ok := nearestRow[recordIndex]
		if !ok {
			continue
		}

		if nearestUnkeyedRecord(soundPlan[rowIndex], table, unkeyed) != recordIndex {
			continue
		}

		matches = append(matches, soundPlanReceiverMatch{
			AconiqIndex:    recordIndex,
			SoundPlanIndex: rowIndex,
			Strategy:       matchStrategyCoordinates,
			DistanceM:      soundPlanMatchDistance(table.Records[recordIndex], soundPlan[rowIndex]),
		})
	}

	return matches
}

func nearestSoundPlanRow(
	record results.ReceiverRecord,
	soundPlan []soundplanimport.ReceiverResult,
	used []bool,
	tolM float64,
) (int, bool) {
	bestIndex := -1
	bestDistance := math.Inf(1)

	for i, candidate := range soundPlan {
		if used[i] || !candidate.HasCoords {
			continue
		}

		distance := math.Hypot(record.X-candidate.X, record.Y-candidate.Y)
		if distance > tolM || distance >= bestDistance {
			continue
		}

		bestIndex = i
		bestDistance = distance
	}

	return bestIndex, bestIndex >= 0
}

func nearestUnkeyedRecord(
	row soundplanimport.ReceiverResult,
	table results.ReceiverTable,
	unkeyed []int,
) int {
	if !row.HasCoords {
		return -1
	}

	bestIndex := -1
	bestDistance := math.Inf(1)

	for _, recordIndex := range unkeyed {
		record := table.Records[recordIndex]

		distance := math.Hypot(record.X-row.X, record.Y-row.Y)
		if distance >= bestDistance {
			continue
		}

		bestIndex = recordIndex
		bestDistance = distance
	}

	return bestIndex
}

// soundPlanMatchDistance is the plan distance between a matched pair. A row
// without coordinates yields 0: there is nothing to assert, and a negative
// sentinel would sort as the best possible agreement.
func soundPlanMatchDistance(record results.ReceiverRecord, row soundplanimport.ReceiverResult) float64 {
	if !row.HasCoords {
		return 0
	}

	return math.Hypot(record.X-row.X, record.Y-row.Y)
}

func describeAconiqReceiver(record results.ReceiverRecord, key soundPlanReceiverKey) string {
	if key.ObjID > 0 {
		return fmt.Sprintf("%s (obj %d floor %d)", record.ID, key.ObjID, key.Floor)
	}

	return record.ID
}

func describeSoundPlanRow(row soundplanimport.ReceiverResult) string {
	name := strings.TrimSpace(row.Name)
	if name == "" {
		name = fmt.Sprintf("rec %d", row.RecNo)
	}

	return fmt.Sprintf("%s (obj %d floor %d)", name, row.ObjID, row.Floor)
}

// soundPlanReceiverKeysFromModel reads each receiver's SoundPLAN identity out
// of the model the run computed.
//
// The identity is read from the feature's properties rather than parsed back
// out of its ID, so the import is the one place that decides what a receiver
// is. It must be handed the *original* model: the raster comparison hands the
// run a temporary copy with thousands of synthetic receivers appended, which
// carry no SoundPLAN identity and are filtered out of this comparison anyway.
//
// A floor of 0 is kept rather than dropped. It is not a usable key, but it is
// still an identity, and matchSoundPlanReceivers needs to tell such a receiver
// apart from one that carries no SoundPLAN identity at all: the first must not
// match, the second falls back to coordinates.
func soundPlanReceiverKeysFromModel(model modelgeojson.Model) map[string]soundPlanReceiverKey {
	keys := make(map[string]soundPlanReceiverKey, len(model.Features))

	for _, feature := range model.Features {
		if feature.Kind != modelgeojson.FeatureKindReceiver {
			continue
		}

		objID, hasObjID := propertyAsInt64(feature.Properties["soundplan_obj_id"])
		floor, hasFloor := propertyAsInt64(feature.Properties["soundplan_floor"])

		if !hasObjID || !hasFloor || objID <= 0 {
			continue
		}

		keys[feature.ID] = soundPlanReceiverKey{ObjID: objID, Floor: int(floor)}
	}

	return keys
}

// propertyAsInt64 reads a GeoJSON property as an integer. A model that came
// back through JSON carries float64 where the import wrote int, so both have
// to be accepted; a non-integral number is not an id and is rejected.
func propertyAsInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		if typed != math.Trunc(typed) || math.IsInf(typed, 0) {
			return 0, false
		}

		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}

		return parsed, true
	default:
		return 0, false
	}
}

// compareReceiverTablesInput bundles everything one receiver comparison needs.
type compareReceiverTablesInput struct {
	table           results.ReceiverTable
	keys            map[string]soundPlanReceiverKey
	soundPlan       []soundplanimport.ReceiverResult
	toleranceDB     float64
	soundPlanSource string
	resultRun       soundPlanResultRunSelection
	runID           string
	standardID      string
	standardVersion string
	standardProfile string
}

func compareSoundPlanReceiverTables(input compareReceiverTablesInput) (soundPlanCompareReport, error) {
	matched, err := matchSoundPlanReceivers(input.table, input.keys, input.soundPlan, defaultReceiverMatchTolM)
	if err != nil {
		return soundPlanCompareReport{}, err
	}

	records := make([]soundPlanReceiverComparisonRecord, 0, len(matched.Matches))
	dayAbs := make([]float64, 0, len(matched.Matches))
	nightAbs := make([]float64, 0, len(matched.Matches))

	for _, match := range matched.Matches {
		record := input.table.Records[match.AconiqIndex]

		dayValue, ok := record.Values[schall03.IndicatorLrDay]
		if !ok {
			return soundPlanCompareReport{}, domainerrors.New(domainerrors.KindValidation, "cli.compare", "receiver table missing LrDay", nil)
		}

		nightValue, ok := record.Values[schall03.IndicatorLrNight]
		if !ok {
			return soundPlanCompareReport{}, domainerrors.New(domainerrors.KindValidation, "cli.compare", "receiver table missing LrNight", nil)
		}

		row := input.soundPlan[match.SoundPlanIndex]
		deltaDay := dayValue - row.ZB1
		deltaNight := nightValue - row.ZB2

		dayAbs = append(dayAbs, math.Abs(deltaDay))
		nightAbs = append(nightAbs, math.Abs(deltaNight))

		records = append(records, soundPlanReceiverComparisonRecord{
			AconiqID:       record.ID,
			SoundPlanRecNo: row.RecNo,
			SoundPlanObjID: int64(row.ObjID),
			SoundPlanFloor: row.Floor,
			SoundPlanName:  row.Name,
			MatchStrategy:  match.Strategy,
			X:              record.X,
			Y:              record.Y,
			DistanceM:      match.DistanceM,
			AconiqLrDay:    dayValue,
			SoundPlanZB1:   row.ZB1,
			DeltaDayDB:     deltaDay,
			AconiqLrNight:  nightValue,
			SoundPlanZB2:   row.ZB2,
			DeltaNightDB:   deltaNight,
		})
	}

	stats := map[string]compareIndicatorStats{
		schall03.IndicatorLrDay:   buildCompareIndicatorStats(dayAbs, input.toleranceDB),
		schall03.IndicatorLrNight: buildCompareIndicatorStats(nightAbs, input.toleranceDB),
	}

	warnings := append(append([]string(nil), input.resultRun.Warnings...), matched.Warnings...)

	return soundPlanCompareReport{
		Command:                      commandNameCompare,
		StandardID:                   input.standardID,
		StandardVersion:              input.standardVersion,
		StandardProfile:              input.standardProfile,
		RunID:                        input.runID,
		SoundPlanSource:              input.soundPlanSource,
		SoundPlanResultRun:           input.resultRun.Dir,
		SoundPlanResultRunCandidates: append([]string(nil), input.resultRun.Candidates...),
		SoundPlanResultRunSelection:  input.resultRun.Selection,
		ReceiverMatchTolM:            defaultReceiverMatchTolM,
		ToleranceDB:                  input.toleranceDB,
		MatchedReceiverCount:         len(records),
		MatchStrategyCounts:          matched.StrategyCounts,
		MaxMatchDistanceM:            matched.MaxDistanceM,
		UnmatchedAconiqCount:         len(matched.UnmatchedAconiq),
		UnmatchedSPCount:             len(matched.UnmatchedSoundPlan),
		UnmatchedAconiq:              matched.UnmatchedAconiq,
		UnmatchedSoundPlan:           matched.UnmatchedSoundPlan,
		StatsScope:                   statsScopeMatchedOnly,
		Warnings:                     warnings,
		Stats:                        stats,
		Records:                      records,
	}, nil
}

func buildCompareIndicatorStats(absDeltas []float64, toleranceDB float64) compareIndicatorStats {
	if len(absDeltas) == 0 {
		return compareIndicatorStats{}
	}

	sorted := append([]float64(nil), absDeltas...)
	slices.Sort(sorted)

	sum := 0.0
	exceeding := 0

	for _, value := range sorted {
		sum += value
		if value > toleranceDB {
			exceeding++
		}
	}

	p95Index := max(int(math.Ceil(0.95*float64(len(sorted))))-1, 0)
	if p95Index >= len(sorted) {
		p95Index = len(sorted) - 1
	}

	return compareIndicatorStats{
		MeanAbsDeltaDB:     sum / float64(len(sorted)),
		MaxAbsDeltaDB:      sorted[len(sorted)-1],
		P95AbsDeltaDB:      sorted[p95Index],
		ToleranceExceeding: exceeding,
		Count:              len(sorted),
	}
}
