package cli

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/engine"
	"github.com/aconiq/backend/internal/geo"
	"github.com/spf13/cobra"
)

type benchScenarioSpec struct {
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	ReceiverColumns  int     `json:"receiver_columns"`
	ReceiverRows     int     `json:"receiver_rows"`
	ReceiverSpacingM float64 `json:"receiver_spacing_m"`
	SourceColumns    int     `json:"source_columns"`
	SourceRows       int     `json:"source_rows"`
	SourceSpacingM   float64 `json:"source_spacing_m"`
}

type benchRunMetrics struct {
	RunID                string `json:"run_id"`
	Workers              int    `json:"workers"`
	ChunkSize            int    `json:"chunk_size"`
	DisableCache         bool   `json:"disable_cache"`
	DurationMS           int64  `json:"duration_ms"`
	ReceiverCount        int    `json:"receiver_count"`
	SourceCount          int    `json:"source_count"`
	TotalChunks          int    `json:"total_chunks"`
	UsedCachedChunks     int    `json:"used_cached_chunks"`
	OutputHash           string `json:"output_hash"`
	RunDirBytesBefore    int64  `json:"run_dir_bytes_before"`
	RunDirBytesAfter     int64  `json:"run_dir_bytes_after"`
	RunDirBytesDelta     int64  `json:"run_dir_bytes_delta"`
	ChunkCacheFileCount  int    `json:"chunk_cache_file_count"`
	AllocBytesBefore     uint64 `json:"alloc_bytes_before"`
	AllocBytesAfter      uint64 `json:"alloc_bytes_after"`
	TotalAllocDeltaBytes uint64 `json:"total_alloc_delta_bytes"`
	SysBytesAfter        uint64 `json:"sys_bytes_after"`
	NumGCDelta           uint32 `json:"num_gc_delta"`
}

type benchNumericDrift struct {
	HashMatch          bool    `json:"hash_match"`
	ChangedReceivers   int     `json:"changed_receivers"`
	MaxAbsLevelDeltaDB float64 `json:"max_abs_level_delta_db"`
}

type benchScenarioResult struct {
	Scenario         benchScenarioSpec `json:"scenario"`
	SourceIndexCellM float64           `json:"source_index_cell_m"`
	Reference        benchRunMetrics   `json:"reference"`
	ColdCache        benchRunMetrics   `json:"cold_cache"`
	WarmCache        benchRunMetrics   `json:"warm_cache"`
	NumericDrift     benchNumericDrift `json:"numeric_drift"`
}

type benchSummary struct {
	BenchID         string                `json:"bench_id"`
	GeneratedAt     time.Time             `json:"generated_at"`
	CacheRoot       string                `json:"cache_root"`
	ScenarioResults []benchScenarioResult `json:"scenario_results"`
	PrunedSuites    []string              `json:"pruned_suites,omitempty"`
}

var defaultBenchScenarios = []benchScenarioSpec{
	{
		Name:             "micro",
		Description:      "Smoke-scale grid for quick cache and determinism checks.",
		ReceiverColumns:  32,
		ReceiverRows:     32,
		ReceiverSpacingM: 10,
		SourceColumns:    4,
		SourceRows:       2,
		SourceSpacingM:   80,
	},
	{
		Name:             "corridor",
		Description:      "Street-corridor style grid with moderate receiver density.",
		ReceiverColumns:  96,
		ReceiverRows:     48,
		ReceiverSpacingM: 10,
		SourceColumns:    6,
		SourceRows:       4,
		SourceSpacingM:   90,
	},
	{
		Name:             "district",
		Description:      "District-scale synthetic load approximating larger planning runs.",
		ReceiverColumns:  160,
		ReceiverRows:     160,
		ReceiverSpacingM: 10,
		SourceColumns:    8,
		SourceRows:       8,
		SourceSpacingM:   100,
	},
}

func newBenchCommand() *cobra.Command {
	var scenarioNames []string
	var workers int
	var chunkSize int
	var sourceIndexCellM float64
	var keepLast int

	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Run synthetic benchmark scenarios for engine runtime, memory, cache IO, and drift",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchCommand(cmd, scenarioNames, workers, chunkSize, sourceIndexCellM, keepLast)
		},
	}

	cmd.Flags().StringSliceVar(&scenarioNames, "scenario", []string{"micro", "corridor", "district"}, "Benchmark scenario(s) to run")
	cmd.Flags().IntVar(&workers, "workers", 0, "Worker count for cold/warm cache runs (0=auto)")
	cmd.Flags().IntVar(&chunkSize, "chunk-size", 128, "Receiver chunk size")
	cmd.Flags().Float64Var(&sourceIndexCellM, "source-index-cell-m", 100, "Source index cell size in meters")
	cmd.Flags().IntVar(&keepLast, "keep-last", 5, "Keep at most this many benchmark suites under the bench cache root")

	return cmd
}

func runBenchCommand(cmd *cobra.Command, scenarioNames []string, workers int, chunkSize int, sourceIndexCellM float64, keepLast int) error {
	state, ok := stateFromCommand(cmd)
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, "cli.bench", "command state unavailable", nil)
	}

	if chunkSize <= 0 {
		return domainerrors.New(domainerrors.KindUserInput, "cli.bench", "--chunk-size must be > 0", nil)
	}

	if sourceIndexCellM <= 0 || math.IsNaN(sourceIndexCellM) || math.IsInf(sourceIndexCellM, 0) {
		return domainerrors.New(domainerrors.KindUserInput, "cli.bench", "--source-index-cell-m must be finite and > 0", nil)
	}

	if keepLast < 1 {
		return domainerrors.New(domainerrors.KindUserInput, "cli.bench", "--keep-last must be >= 1", nil)
	}

	scenarios, err := resolveBenchScenarios(scenarioNames)
	if err != nil {
		return domainerrors.New(domainerrors.KindUserInput, "cli.bench", err.Error(), nil)
	}

	benchID := time.Now().UTC().Format("20060102T150405.000000000Z")
	benchRoot := filepath.Join(state.Config.CacheDir, "bench")
	suiteDir := filepath.Join(benchRoot, benchID)

	err = os.MkdirAll(suiteDir, 0o755)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.bench", "create bench suite directory "+suiteDir, err)
	}

	summary := benchSummary{
		BenchID:         benchID,
		GeneratedAt:     nowUTC(),
		CacheRoot:       benchRoot,
		ScenarioResults: make([]benchScenarioResult, 0, len(scenarios)),
	}

	for _, scenario := range scenarios {
		result, err := runBenchScenario(suiteDir, scenario, workers, chunkSize, sourceIndexCellM)
		if err != nil {
			return err
		}

		summary.ScenarioResults = append(summary.ScenarioResults, result)
	}

	prunedSuites, err := pruneBenchSuites(benchRoot, keepLast)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.bench", "prune bench suites", err)
	}

	summary.PrunedSuites = prunedSuites

	summaryPath := filepath.Join(suiteDir, "summary.json")

	err = writeJSONFile(summaryPath, summary)
	if err != nil {
		return err
	}

	if state.Config.JSONLogs {
		return writeCommandOutput(cmd.OutOrStdout(), true, map[string]any{
			"command":      "bench",
			"bench_id":     summary.BenchID,
			"summary_path": summaryPath,
			"summary":      summary,
		})
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Benchmark suite %s\n", summary.BenchID)
	for _, scenario := range summary.ScenarioResults {
		_, _ = fmt.Fprintf(
			cmd.OutOrStdout(),
			"%s: receivers=%d sources=%d cold=%dms warm=%dms cached_chunks=%d drift_max_abs_db=%.9f\n",
			scenario.Scenario.Name,
			scenario.ColdCache.ReceiverCount,
			scenario.ColdCache.SourceCount,
			scenario.ColdCache.DurationMS,
			scenario.WarmCache.DurationMS,
			scenario.WarmCache.UsedCachedChunks,
			scenario.NumericDrift.MaxAbsLevelDeltaDB,
		)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Summary: %s\n", summaryPath)
	if len(summary.PrunedSuites) > 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pruned suites: %d\n", len(summary.PrunedSuites))
	}

	return nil
}

func resolveBenchScenarios(names []string) ([]benchScenarioSpec, error) {
	if len(names) == 0 {
		return append([]benchScenarioSpec(nil), defaultBenchScenarios...), nil
	}

	index := make(map[string]benchScenarioSpec, len(defaultBenchScenarios))
	for _, scenario := range defaultBenchScenarios {
		index[scenario.Name] = scenario
	}

	out := make([]benchScenarioSpec, 0, len(names))

	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		scenario, ok := index[name]
		if !ok {
			return nil, fmt.Errorf("unknown benchmark scenario %q", name)
		}

		if _, exists := seen[name]; exists {
			continue
		}

		seen[name] = struct{}{}

		out = append(out, scenario)
	}

	return out, nil
}

func runBenchScenario(
	suiteDir string,
	spec benchScenarioSpec,
	workers int,
	chunkSize int,
	sourceIndexCellM float64,
) (benchScenarioResult, error) {
	receivers, sources := buildBenchScenario(spec)
	scenarioDir := filepath.Join(suiteDir, spec.Name)
	runner := engine.NewRunner(nil)

	reference, referenceOut, err := measureBenchRun(runner, scenarioDir, "reference", engine.RunConfig{
		RunID:            "reference",
		Workers:          1,
		ChunkSize:        chunkSize,
		CacheDir:         scenarioDir,
		Receivers:        receivers,
		Sources:          sources,
		DisableCache:     true,
		SourceIndexCellM: sourceIndexCellM,
		DeterminismTag:   "bench-reference",
	})
	if err != nil {
		return benchScenarioResult{}, domainerrors.New(domainerrors.KindInternal, "cli.bench", "run reference benchmark scenario "+spec.Name, err)
	}

	cold, coldOut, err := measureBenchRun(runner, scenarioDir, "bench", engine.RunConfig{
		RunID:            "bench",
		Workers:          workers,
		ChunkSize:        chunkSize,
		CacheDir:         scenarioDir,
		Receivers:        receivers,
		Sources:          sources,
		DisableCache:     false,
		SourceIndexCellM: sourceIndexCellM,
		DeterminismTag:   "bench-cold",
	})
	if err != nil {
		return benchScenarioResult{}, domainerrors.New(domainerrors.KindInternal, "cli.bench", "run cold benchmark scenario "+spec.Name, err)
	}

	warm, warmOut, err := measureBenchRun(runner, scenarioDir, "bench", engine.RunConfig{
		RunID:            "bench",
		Workers:          workers,
		ChunkSize:        chunkSize,
		CacheDir:         scenarioDir,
		Receivers:        receivers,
		Sources:          sources,
		DisableCache:     false,
		SourceIndexCellM: sourceIndexCellM,
		DeterminismTag:   "bench-warm",
	})
	if err != nil {
		return benchScenarioResult{}, domainerrors.New(domainerrors.KindInternal, "cli.bench", "run warm benchmark scenario "+spec.Name, err)
	}

	drift, err := compareBenchOutputs(referenceOut, coldOut)
	if err != nil {
		return benchScenarioResult{}, domainerrors.New(domainerrors.KindInternal, "cli.bench", "compare benchmark outputs "+spec.Name, err)
	}

	if warmOut.OutputHash != coldOut.OutputHash {
		return benchScenarioResult{}, domainerrors.New(domainerrors.KindInternal, "cli.bench", "warm cache output hash differs from cold cache output", nil)
	}

	return benchScenarioResult{
		Scenario:         spec,
		SourceIndexCellM: sourceIndexCellM,
		Reference:        reference,
		ColdCache:        cold,
		WarmCache:        warm,
		NumericDrift:     drift,
	}, nil
}

func buildBenchScenario(spec benchScenarioSpec) ([]engine.Receiver, []engine.Source) {
	receivers := make([]engine.Receiver, 0, spec.ReceiverColumns*spec.ReceiverRows)
	for row := range spec.ReceiverRows {
		for col := range spec.ReceiverColumns {
			index := row*spec.ReceiverColumns + col
			receivers = append(receivers, engine.Receiver{
				ID:      fmt.Sprintf("rx-%06d", index),
				Point:   geo.Point2D{X: float64(col) * spec.ReceiverSpacingM, Y: float64(row) * spec.ReceiverSpacingM},
				HeightM: 4,
			})
		}
	}

	receiverWidthM := float64(max(spec.ReceiverColumns-1, 0)) * spec.ReceiverSpacingM
	receiverHeightM := float64(max(spec.ReceiverRows-1, 0)) * spec.ReceiverSpacingM
	xOffset := -spec.SourceSpacingM
	yOffset := -spec.SourceSpacingM

	sources := make([]engine.Source, 0, spec.SourceColumns*spec.SourceRows)
	for row := range spec.SourceRows {
		for col := range spec.SourceColumns {
			index := row*spec.SourceColumns + col

			x := xOffset
			if spec.SourceColumns > 1 {
				x += float64(col) * ((receiverWidthM + 2*spec.SourceSpacingM) / float64(spec.SourceColumns-1))
			}

			y := yOffset
			if spec.SourceRows > 1 {
				y += float64(row) * ((receiverHeightM + 2*spec.SourceSpacingM) / float64(spec.SourceRows-1))
			}

			sources = append(sources, engine.Source{
				ID:       fmt.Sprintf("src-%03d", index),
				Point:    geo.Point2D{X: x, Y: y},
				Emission: 88 + float64((row+col)%7),
			})
		}
	}

	return receivers, sources
}

func measureBenchRun(
	runner *engine.Runner,
	scenarioDir string,
	runID string,
	cfg engine.RunConfig,
) (benchRunMetrics, engine.RunOutput, error) {
	runDir := filepath.Join(scenarioDir, runID)

	beforeBytes, err := directorySize(runDir)
	if err != nil {
		return benchRunMetrics{}, engine.RunOutput{}, err
	}

	runtime.GC()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	startedAt := time.Now()
	out, err := runner.Run(context.Background(), cfg)
	duration := time.Since(startedAt)

	if err != nil {
		return benchRunMetrics{}, engine.RunOutput{}, err
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	afterBytes, err := directorySize(runDir)
	if err != nil {
		return benchRunMetrics{}, engine.RunOutput{}, err
	}

	chunkCacheFiles, err := countChunkCacheFiles(filepath.Join(runDir, "chunks"))
	if err != nil {
		return benchRunMetrics{}, engine.RunOutput{}, err
	}

	metrics := benchRunMetrics{
		RunID:                runID,
		Workers:              extractIntMetadata(out.Metadata, "workers", cfg.Workers),
		ChunkSize:            cfg.ChunkSize,
		DisableCache:         cfg.DisableCache,
		DurationMS:           duration.Milliseconds(),
		ReceiverCount:        len(cfg.Receivers),
		SourceCount:          len(cfg.Sources),
		TotalChunks:          out.TotalChunks,
		UsedCachedChunks:     out.UsedCachedChunks,
		OutputHash:           out.OutputHash,
		RunDirBytesBefore:    beforeBytes,
		RunDirBytesAfter:     afterBytes,
		RunDirBytesDelta:     afterBytes - beforeBytes,
		ChunkCacheFileCount:  chunkCacheFiles,
		AllocBytesBefore:     before.Alloc,
		AllocBytesAfter:      after.Alloc,
		TotalAllocDeltaBytes: after.TotalAlloc - before.TotalAlloc,
		SysBytesAfter:        after.Sys,
		NumGCDelta:           after.NumGC - before.NumGC,
	}

	return metrics, out, nil
}

func compareBenchOutputs(reference engine.RunOutput, candidate engine.RunOutput) (benchNumericDrift, error) {
	if len(reference.Results) != len(candidate.Results) {
		return benchNumericDrift{}, fmt.Errorf("result length mismatch: %d vs %d", len(reference.Results), len(candidate.Results))
	}

	drift := benchNumericDrift{
		HashMatch: reference.OutputHash == candidate.OutputHash,
	}

	for idx := range reference.Results {
		if reference.Results[idx].ReceiverID != candidate.Results[idx].ReceiverID {
			return benchNumericDrift{}, fmt.Errorf(
				"receiver order mismatch at index %d: %s vs %s",
				idx,
				reference.Results[idx].ReceiverID,
				candidate.Results[idx].ReceiverID,
			)
		}

		delta := math.Abs(reference.Results[idx].LevelDB - candidate.Results[idx].LevelDB)
		if delta > 0 {
			drift.ChangedReceivers++
		}

		if delta > drift.MaxAbsLevelDeltaDB {
			drift.MaxAbsLevelDeltaDB = delta
		}
	}

	return drift, nil
}

func pruneBenchSuites(root string, keepLast int) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	suites := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		suites = append(suites, entry.Name())
	}

	slices.Sort(suites)

	if len(suites) <= keepLast {
		return nil, nil
	}

	pruned := make([]string, 0, len(suites)-keepLast)
	for _, suite := range suites[:len(suites)-keepLast] {
		path := filepath.Join(root, suite)

		err := os.RemoveAll(path)
		if err != nil {
			return pruned, err
		}

		pruned = append(pruned, path)
	}

	return pruned, nil
}

func directorySize(root string) (int64, error) {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}

		return 0, err
	}

	if !info.IsDir() {
		return info.Size(), nil
	}

	var total int64

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		total += info.Size()

		return nil
	})
	if err != nil {
		return 0, err
	}

	return total, nil
}

func countChunkCacheFiles(root string) (int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}

		return 0, err
	}

	total := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		total++
	}

	return total, nil
}

func extractIntMetadata(metadata map[string]any, key string, fallback int) int {
	value, ok := metadata[key]
	if !ok {
		return fallback
	}

	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return fallback
	}
}
