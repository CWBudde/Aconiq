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
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/io/soundplanimport"
	"github.com/aconiq/backend/internal/report/results"
	"github.com/aconiq/backend/internal/standards/schall03"
	"github.com/spf13/cobra"
)

const (
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
	Status                 string                             `json:"status"`
	Alignment              string                             `json:"alignment,omitempty"`
	GridResolutionM        float64                            `json:"grid_resolution_m,omitempty"`
	ReceiverHeightM        float64                            `json:"receiver_height_m,omitempty"`
	SyntheticReceiverCount int                                `json:"synthetic_receiver_count,omitempty"`
	ArtifactPath           string                             `json:"artifact_path,omitempty"`
	SoundPlanRuns          []soundplanimport.GridMapMetadata  `json:"soundplan_runs,omitempty"`
	Runs                   []soundPlanRasterRunCompareSummary `json:"runs,omitempty"`
	Warnings               []string                           `json:"warnings,omitempty"`
}

type soundPlanCompareReport struct {
	Command              string                              `json:"command"`
	StandardID           string                              `json:"standard_id"`
	StandardVersion      string                              `json:"standard_version,omitempty"`
	StandardProfile      string                              `json:"standard_profile,omitempty"`
	RunID                string                              `json:"run_id"`
	SoundPlanSource      string                              `json:"soundplan_source"`
	SoundPlanResultRun   string                              `json:"soundplan_result_run"`
	ReceiverMatchTolM    float64                             `json:"receiver_match_tolerance_m"`
	ToleranceDB          float64                             `json:"tolerance_db"`
	MatchedReceiverCount int                                 `json:"matched_receiver_count"`
	UnmatchedAconiqCount int                                 `json:"unmatched_aconiq_count"`
	UnmatchedSPCount     int                                 `json:"unmatched_soundplan_count"`
	Raster               *soundPlanRasterCompareReport       `json:"raster,omitempty"`
	Stats                map[string]compareIndicatorStats    `json:"stats"`
	Records              []soundPlanReceiverComparisonRecord `json:"records"`
}

func newCompareCommand() *cobra.Command {
	var standardID string
	var standardVersion string
	var standardProfile string
	var modelPath string
	var scenarioID string
	var toleranceDB float64

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Run a comparison against imported SoundPLAN receiver results",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCompare(cmd, standardID, standardVersion, standardProfile, modelPath, scenarioID, toleranceDB)
		},
	}

	cmd.Flags().StringVar(&scenarioID, "scenario", "default", "Scenario ID")
	cmd.Flags().StringVar(&standardID, "standard", schall03.StandardID, "Standard identifier")
	cmd.Flags().StringVar(&standardVersion, "standard-version", "", "Standard version (defaults to standard default)")
	cmd.Flags().StringVar(&standardProfile, "standard-profile", "", "Standard profile (defaults to version profile default)")
	cmd.Flags().StringVar(&modelPath, "model", defaultModelPath, "Path to normalized GeoJSON model")
	cmd.Flags().Float64Var(&toleranceDB, "tolerance-db", 0.5, "Absolute delta threshold for tolerance exceedance counting")

	return cmd
}

func runCompare(cmd *cobra.Command, standardID, standardVersion, standardProfile, modelPath, scenarioID string, toleranceDB float64) error {
	if strings.TrimSpace(standardID) != schall03.StandardID {
		return domainerrors.New(domainerrors.KindUserInput, "cli.compare", "compare currently supports only schall03", nil)
	}

	if toleranceDB < 0 || math.IsNaN(toleranceDB) || math.IsInf(toleranceDB, 0) {
		return domainerrors.New(domainerrors.KindUserInput, "cli.compare", "--tolerance-db must be finite and >= 0", nil)
	}

	state, ok := stateFromCommand(cmd)
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, "cli.compare", "command state unavailable", nil)
	}

	store, err := projectfs.New(state.Config.ProjectPath)
	if err != nil {
		return err
	}

	importReport, err := loadSoundPlanImportReport(store.Root())
	if err != nil {
		return err
	}

	soundPlanRoot := resolvePath(store.Root(), importReport.SourcePath)
	resultRunDirs, err := selectSoundPlanReceiverResultDirs(soundPlanRoot)
	if err != nil {
		return err
	}

	soundPlanReceivers, err := loadSoundPlanReceiverResults(soundPlanRoot, resultRunDirs)
	if err != nil {
		return err
	}

	rasterPrep, err := prepareSoundPlanRasterCompare(store.Root(), importReport, modelPath)
	if err != nil {
		return err
	}
	if rasterPrep != nil {
		defer cleanupRasterComparePreparation(rasterPrep)
	}

	runModelPath := modelPath
	if rasterPrep != nil && strings.TrimSpace(rasterPrep.tempModelPath) != "" {
		runModelPath = relativePath(store.Root(), rasterPrep.tempModelPath)
	}

	run, err := executeCompareRun(cmd, runCommandRequest{
		scenarioID:      scenarioID,
		standardID:      standardID,
		standardVersion: standardVersion,
		standardProfile: standardProfile,
		modelPath:       runModelPath,
		receiverMode:    receiverModeCustom,
	})
	if err != nil {
		return err
	}

	receiverTable, err := results.LoadReceiverTableJSON(filepath.Join(store.Root(), ".noise", "runs", run.ID, "results", "receivers.json"))
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.compare", "load run receiver outputs", err)
	}

	report, err := compareSoundPlanReceiverTables(filterOutSyntheticRasterReceivers(receiverTable), soundPlanReceivers, toleranceDB, importReport.SourcePath, strings.Join(resultRunDirs, ","), run.ID, standardID, standardVersion, standardProfile)
	if err != nil {
		return err
	}

	report.Raster, _, err = finalizeSoundPlanRasterCompare(store.Root(), rasterPrep, receiverTable, toleranceDB)
	if err != nil {
		return err
	}
	if report.Raster == nil {
		report.Raster = buildSoundPlanRasterCompareReport(importReport)
	}

	reportPath := filepath.Join(store.Root(), filepath.FromSlash(defaultCompareReportPath))
	if err := writeJSONFile(reportPath, report); err != nil {
		return err
	}

	proj, err := store.Load()
	if err != nil {
		return err
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
		return err
	}

	if state.Config.JSONLogs {
		return writeCommandOutput(cmd.OutOrStdout(), true, map[string]any{
			"command":                   "compare",
			"report_path":               defaultCompareReportPath,
			"run_id":                    run.ID,
			"matched_receiver_count":    report.MatchedReceiverCount,
			"unmatched_aconiq_count":    report.UnmatchedAconiqCount,
			"unmatched_soundplan_count": report.UnmatchedSPCount,
			"soundplan_result_run":      report.SoundPlanResultRun,
			"raster_status":             compareRasterStatus(report.Raster),
			"raster_artifact_path": func() string {
				if report.Raster == nil {
					return ""
				}

				return report.Raster.ArtifactPath
			}(),
			"soundplan_raster_run_count": func() int {
				if report.Raster == nil {
					return 0
				}

				return len(report.Raster.SoundPlanRuns)
			}(),
		})
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Compared run %s against SoundPLAN %s\n", run.ID, report.SoundPlanResultRun)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Matched receivers: %d\n", report.MatchedReceiverCount)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Unmatched Aconiq receivers: %d\n", report.UnmatchedAconiqCount)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Unmatched SoundPLAN receivers: %d\n", report.UnmatchedSPCount)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Report: %s\n", defaultCompareReportPath)
	if report.Raster != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Raster coverage: %s (%d SoundPLAN grid-map runs)\n", report.Raster.Status, len(report.Raster.SoundPlanRuns))
	}

	for _, indicator := range []string{schall03.IndicatorLrDay, schall03.IndicatorLrNight} {
		stats, ok := report.Stats[indicator]
		if !ok {
			continue
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s mean_abs=%.3f max_abs=%.3f p95_abs=%.3f exceedances=%d\n",
			indicator, stats.MeanAbsDeltaDB, stats.MaxAbsDeltaDB, stats.P95AbsDeltaDB, stats.ToleranceExceeding)
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
		return project.Run{}, err
	}

	proj, err := store.Load()
	if err != nil {
		return project.Run{}, err
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

func selectSoundPlanReceiverResultDirs(soundPlanRoot string) ([]string, error) {
	resultDirs := make([]string, 0, 4)
	candidates := []string{"RSPS0011", "RSPS0021", "RSPS0000"}
	for _, candidate := range candidates {
		dir := filepath.Join(soundPlanRoot, candidate)
		suffix := compareExtractRunSuffix(candidate)
		if compareFileExists(filepath.Join(dir, "RREC"+suffix+".abs")) {
			resultDirs = append(resultDirs, candidate)
		}
	}

	if len(resultDirs) > 0 {
		return resultDirs, nil
	}

	matches, err := filepath.Glob(filepath.Join(soundPlanRoot, "RSPS*"))
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.compare", "discover SoundPLAN receiver result directories", err)
	}

	slices.Sort(matches)
	for _, match := range matches {
		name := filepath.Base(match)
		suffix := compareExtractRunSuffix(name)
		if compareFileExists(filepath.Join(match, "RREC"+suffix+".abs")) {
			resultDirs = append(resultDirs, name)
		}
	}

	if len(resultDirs) == 0 {
		return nil, domainerrors.New(domainerrors.KindUserInput, "cli.compare", "no SoundPLAN RSPS receiver result directory found", nil)
	}

	return resultDirs, nil
}

func loadSoundPlanReceiverResults(soundPlanRoot string, resultRunDirs []string) ([]soundplanimport.ReceiverResult, error) {
	all := make([]soundplanimport.ReceiverResult, 0, 128)
	for _, resultRunDir := range resultRunDirs {
		suffix := compareExtractRunSuffix(resultRunDir)
		path := filepath.Join(soundPlanRoot, resultRunDir, "RREC"+suffix+".abs")

		results, err := soundplanimport.ParseReceiverResults(path)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindInternal, "cli.compare", "read SoundPLAN receiver results", err)
		}

		all = append(all, results...)
	}

	return all, nil
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

func compareSoundPlanReceiverTables(
	table results.ReceiverTable,
	soundPlan []soundplanimport.ReceiverResult,
	toleranceDB float64,
	soundPlanSource string,
	resultRunDir string,
	runID string,
	standardID string,
	standardVersion string,
	standardProfile string,
) (soundPlanCompareReport, error) {
	used := make([]bool, len(soundPlan))
	records := make([]soundPlanReceiverComparisonRecord, 0, len(table.Records))
	dayAbs := make([]float64, 0, len(table.Records))
	nightAbs := make([]float64, 0, len(table.Records))
	unmatchedAconiq := 0

	for _, record := range table.Records {
		dayValue, ok := record.Values[schall03.IndicatorLrDay]
		if !ok {
			return soundPlanCompareReport{}, domainerrors.New(domainerrors.KindValidation, "cli.compare", "receiver table missing LrDay", nil)
		}

		nightValue, ok := record.Values[schall03.IndicatorLrNight]
		if !ok {
			return soundPlanCompareReport{}, domainerrors.New(domainerrors.KindValidation, "cli.compare", "receiver table missing LrNight", nil)
		}

		bestIndex := -1
		bestDistance := math.Inf(1)
		matchStrategy := ""
		for i, candidate := range soundPlan {
			if used[i] || !candidate.HasCoords {
				continue
			}

			dx := record.X - candidate.X
			dy := record.Y - candidate.Y
			distance := math.Hypot(dx, dy)
			if distance > defaultReceiverMatchTolM {
				continue
			}

			if distance < bestDistance || (distance == bestDistance && candidate.RecNo < soundPlan[bestIndex].RecNo) {
				bestIndex = i
				bestDistance = distance
				matchStrategy = "coordinates"
			}
		}

		if bestIndex < 0 {
			for i := range soundPlan {
				if used[i] {
					continue
				}

				bestIndex = i
				bestDistance = -1
				matchStrategy = "ordinal"
				break
			}
		}

		if bestIndex < 0 {
			unmatchedAconiq++
			continue
		}

		used[bestIndex] = true
		matched := soundPlan[bestIndex]
		deltaDay := dayValue - matched.ZB1
		deltaNight := nightValue - matched.ZB2
		dayAbs = append(dayAbs, math.Abs(deltaDay))
		nightAbs = append(nightAbs, math.Abs(deltaNight))

		records = append(records, soundPlanReceiverComparisonRecord{
			AconiqID:       record.ID,
			SoundPlanRecNo: matched.RecNo,
			SoundPlanName:  matched.Name,
			MatchStrategy:  matchStrategy,
			X:              record.X,
			Y:              record.Y,
			DistanceM:      bestDistance,
			AconiqLrDay:    dayValue,
			SoundPlanZB1:   matched.ZB1,
			DeltaDayDB:     deltaDay,
			AconiqLrNight:  nightValue,
			SoundPlanZB2:   matched.ZB2,
			DeltaNightDB:   deltaNight,
		})
	}

	stats := map[string]compareIndicatorStats{
		schall03.IndicatorLrDay:   buildCompareIndicatorStats(dayAbs, toleranceDB),
		schall03.IndicatorLrNight: buildCompareIndicatorStats(nightAbs, toleranceDB),
	}

	unmatchedSoundPlan := 0
	for i := range soundPlan {
		if !used[i] {
			unmatchedSoundPlan++
		}
	}

	return soundPlanCompareReport{
		Command:              "compare",
		StandardID:           standardID,
		StandardVersion:      standardVersion,
		StandardProfile:      standardProfile,
		RunID:                runID,
		SoundPlanSource:      soundPlanSource,
		SoundPlanResultRun:   resultRunDir,
		ReceiverMatchTolM:    defaultReceiverMatchTolM,
		ToleranceDB:          toleranceDB,
		MatchedReceiverCount: len(records),
		UnmatchedAconiqCount: unmatchedAconiq,
		UnmatchedSPCount:     unmatchedSoundPlan,
		Stats:                stats,
		Records:              records,
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

	p95Index := int(math.Ceil(0.95*float64(len(sorted)))) - 1
	if p95Index < 0 {
		p95Index = 0
	}
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
