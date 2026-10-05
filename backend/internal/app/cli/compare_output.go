package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aconiq/backend/internal/standards/schall03"
	"github.com/spf13/cobra"
)

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
