package cli

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	domainerrors "github.com/soundplan/soundplan/backend/internal/domain/errors"
	"github.com/soundplan/soundplan/backend/internal/domain/project"
	"github.com/soundplan/soundplan/backend/internal/engine"
	"github.com/soundplan/soundplan/backend/internal/geo"
	"github.com/soundplan/soundplan/backend/internal/geo/modelgeojson"
	"github.com/soundplan/soundplan/backend/internal/io/projectfs"
	"github.com/soundplan/soundplan/backend/internal/report/results"
	"github.com/soundplan/soundplan/backend/internal/standards/dummy/freefield"
	"github.com/spf13/cobra"
)

const (
	dummyIndicatorName     = "Ldummy"
	dummyResultUnit        = "dB"
	defaultModelPath       = ".noise/model/model.normalized.geojson"
	defaultGridResolutionM = 10.0
	defaultGridPaddingM    = 20.0
	defaultReceiverHeightM = 4.0
	defaultSourceEmission  = 90.0
	defaultChunkSize       = 128
	maxDummyReceivers      = 250000
)

type dummyRunOptions struct {
	GridResolutionM float64
	GridPaddingM    float64
	ReceiverHeightM float64
	SourceEmission  float64
	Workers         int
	ChunkSize       int
	DisableCache    bool
}

type persistedRunOutputs struct {
	ReceiverJSONPath   string
	ReceiverCSVPath    string
	RasterMetadataPath string
	RasterDataPath     string
	SummaryPath        string
}

func newRunCommand() *cobra.Command {
	var scenarioID string
	var standardID string
	var standardVersion string
	var standardProfile string
	var modelPath string
	var rawParams []string
	var inputPaths []string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute a run and persist result artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, ok := stateFromCommand(cmd)
			if !ok {
				return domainerrors.New(domainerrors.KindInternal, "cli.run", "command state unavailable", nil)
			}

			params, err := parseKeyValueFlags(rawParams)
			if err != nil {
				return err
			}

			store, err := projectfs.New(state.Config.ProjectPath)
			if err != nil {
				return err
			}

			proj, err := store.Load()
			if err != nil {
				return err
			}

			if standardID != freefield.StandardID {
				return domainerrors.New(
					domainerrors.KindUserInput,
					"cli.run",
					fmt.Sprintf("unsupported --standard %q (only %q is available in Phase 8)", standardID, freefield.StandardID),
					nil,
				)
			}

			options, err := parseDummyRunOptions(params)
			if err != nil {
				return err
			}

			resolvedModelPath := resolvePath(store.Root(), modelPath)
			relModelPath := relativePath(store.Root(), resolvedModelPath)
			combinedInputs := mergeInputPaths(append([]string{relModelPath}, inputPaths...))

			run, provenance, err := store.CreateRun(projectfs.CreateRunSpec{
				ScenarioID: scenarioID,
				Standard: project.StandardRef{
					ID:      standardID,
					Version: standardVersion,
					Profile: standardProfile,
				},
				Parameters: params,
				InputPaths: combinedInputs,
				Status:     project.RunStatusRunning,
				LogLines: []string{
					fmt.Sprintf("%s run started", nowUTC().Format(time.RFC3339)),
				},
			})
			if err != nil {
				return err
			}

			logLines := []string{
				fmt.Sprintf("%s run started", run.StartedAt.Format(time.RFC3339)),
				fmt.Sprintf("%s standard=%s version=%s profile=%s", run.StartedAt.Format(time.RFC3339), run.Standard.ID, run.Standard.Version, run.Standard.Profile),
				fmt.Sprintf("%s model=%s", run.StartedAt.Format(time.RFC3339), relModelPath),
			}

			model, err := loadValidatedModel(resolvedModelPath, proj.CRS, relModelPath)
			if err != nil {
				logLines = append(logLines, fmt.Sprintf("%s failed to load model: %v", nowUTC().Format(time.RFC3339), err))
				return finalizeRunFailure(store, run, logLines, err)
			}

			sources, err := extractDummySources(model, options.SourceEmission)
			if err != nil {
				logLines = append(logLines, fmt.Sprintf("%s failed to extract sources: %v", nowUTC().Format(time.RFC3339), err))
				return finalizeRunFailure(store, run, logLines, err)
			}

			receivers, gridWidth, gridHeight, err := buildDummyReceivers(sources, options)
			if err != nil {
				logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), err))
				return finalizeRunFailure(store, run, logLines, err)
			}

			logLines = append(
				logLines,
				fmt.Sprintf("%s sources=%d", nowUTC().Format(time.RFC3339), len(sources)),
				fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), len(receivers), gridWidth, gridHeight),
			)

			engineRunner := engine.NewRunner(func(event engine.ProgressEvent) {
				if event.Stage == "compute" && event.Message == "chunk_done" {
					logLines = append(logLines, fmt.Sprintf("%s stage=%s chunk=%d %d/%d", event.Time.Format(time.RFC3339), event.Stage, event.ChunkIndex, event.CompletedChunks, event.TotalChunks))
					return
				}
				logLines = append(logLines, fmt.Sprintf("%s stage=%s message=%s", event.Time.Format(time.RFC3339), event.Stage, event.Message))
			})

			engineSources := make([]engine.Source, 0, len(sources))
			for _, source := range sources {
				engineSources = append(engineSources, engine.Source{
					ID:       source.ID,
					Point:    source.Point,
					Emission: source.EmissionDB,
				})
			}

			runOutput, err := engineRunner.Run(context.Background(), engine.RunConfig{
				RunID:          run.ID,
				Workers:        options.Workers,
				ChunkSize:      options.ChunkSize,
				CacheDir:       state.Config.CacheDir,
				Receivers:      receivers,
				Sources:        engineSources,
				DisableCache:   options.DisableCache,
				DeterminismTag: "phase8-dummy-freefield",
			})
			if err != nil {
				logLines = append(logLines, fmt.Sprintf("%s engine failed: %v", nowUTC().Format(time.RFC3339), err))
				return finalizeRunFailure(store, run, logLines, err)
			}

			runDir := filepath.Join(store.Root(), ".noise", "runs", run.ID)
			persisted, err := persistDummyRunOutputs(runDir, runOutput, receivers, gridWidth, gridHeight)
			if err != nil {
				logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
				return finalizeRunFailure(store, run, logLines, err)
			}

			now := nowUTC()
			artifacts := []project.ArtifactRef{
				{
					ID:        fmt.Sprintf("artifact-run-%s-receivers-json", run.ID),
					RunID:     run.ID,
					Kind:      "run.result.receiver_table_json",
					Path:      relativePath(store.Root(), persisted.ReceiverJSONPath),
					CreatedAt: now,
				},
				{
					ID:        fmt.Sprintf("artifact-run-%s-receivers-csv", run.ID),
					RunID:     run.ID,
					Kind:      "run.result.receiver_table_csv",
					Path:      relativePath(store.Root(), persisted.ReceiverCSVPath),
					CreatedAt: now,
				},
				{
					ID:        fmt.Sprintf("artifact-run-%s-raster-meta", run.ID),
					RunID:     run.ID,
					Kind:      "run.result.raster_metadata",
					Path:      relativePath(store.Root(), persisted.RasterMetadataPath),
					CreatedAt: now,
				},
				{
					ID:        fmt.Sprintf("artifact-run-%s-raster-data", run.ID),
					RunID:     run.ID,
					Kind:      "run.result.raster_binary",
					Path:      relativePath(store.Root(), persisted.RasterDataPath),
					CreatedAt: now,
				},
				{
					ID:        fmt.Sprintf("artifact-run-%s-summary", run.ID),
					RunID:     run.ID,
					Kind:      "run.result.summary",
					Path:      relativePath(store.Root(), persisted.SummaryPath),
					CreatedAt: now,
				},
			}

			logLines = append(
				logLines,
				fmt.Sprintf("%s output_hash=%s", nowUTC().Format(time.RFC3339), runOutput.OutputHash),
				fmt.Sprintf("%s persisted=%s", nowUTC().Format(time.RFC3339), relativePath(store.Root(), persisted.SummaryPath)),
				fmt.Sprintf("%s run completed", nowUTC().Format(time.RFC3339)),
			)

			if err := finalizeRun(store, run, project.RunStatusCompleted, runOutput.FinishedAt, logLines, artifacts); err != nil {
				return err
			}

			state.Logger.Info(
				"run completed",
				"run_id", run.ID,
				"status", project.RunStatusCompleted,
				"standard_id", run.Standard.ID,
				"provenance", provenance.ManifestPath,
				"output_hash", runOutput.OutputHash,
			)

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Completed run %s (%s)\n", run.ID, project.RunStatusCompleted)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Provenance: %s\n", provenance.ManifestPath)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Results: %s\n", relativePath(store.Root(), filepath.Join(runDir, "results")))
			return nil
		},
	}

	cmd.Flags().StringVar(&scenarioID, "scenario", "default", "Scenario ID")
	cmd.Flags().StringVar(&standardID, "standard", freefield.StandardID, "Standard identifier")
	cmd.Flags().StringVar(&standardID, "standard-id", freefield.StandardID, "Deprecated alias for --standard")
	cmd.Flags().StringVar(&standardVersion, "standard-version", "v0", "Standard version")
	cmd.Flags().StringVar(&standardProfile, "standard-profile", "default", "Standard profile")
	cmd.Flags().StringVar(&modelPath, "model", defaultModelPath, "Path to normalized GeoJSON model")
	cmd.Flags().StringArrayVar(&rawParams, "param", nil, "Run parameter key=value (repeatable)")
	cmd.Flags().StringArrayVar(&inputPaths, "input", nil, "Input path to hash into provenance (repeatable)")

	return cmd
}

func parseKeyValueFlags(values []string) (map[string]string, error) {
	params := make(map[string]string, len(values))
	for _, item := range values {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			return nil, domainerrors.New(domainerrors.KindUserInput, "cli.parseKeyValueFlags", fmt.Sprintf("invalid --param %q (expected key=value)", item), nil)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, domainerrors.New(domainerrors.KindUserInput, "cli.parseKeyValueFlags", fmt.Sprintf("invalid --param %q (empty key)", item), nil)
		}

		params[key] = value
	}

	return params, nil
}

func parseDummyRunOptions(params map[string]string) (dummyRunOptions, error) {
	options := dummyRunOptions{
		GridResolutionM: defaultGridResolutionM,
		GridPaddingM:    defaultGridPaddingM,
		ReceiverHeightM: defaultReceiverHeightM,
		SourceEmission:  defaultSourceEmission,
		ChunkSize:       defaultChunkSize,
	}

	parseFloat := func(key string, target *float64, min float64) error {
		value, ok := params[key]
		if !ok {
			return nil
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseDummyRunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}
		if math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < min {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseDummyRunOptions", fmt.Sprintf("%s must be >= %g", key, min), nil)
		}
		*target = parsed
		return nil
	}

	parseInt := func(key string, target *int, min int) error {
		value, ok := params[key]
		if !ok {
			return nil
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseDummyRunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}
		if parsed < min {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseDummyRunOptions", fmt.Sprintf("%s must be >= %d", key, min), nil)
		}
		*target = parsed
		return nil
	}

	if err := parseFloat("grid_resolution_m", &options.GridResolutionM, 0.001); err != nil {
		return dummyRunOptions{}, err
	}
	if err := parseFloat("grid_padding_m", &options.GridPaddingM, 0); err != nil {
		return dummyRunOptions{}, err
	}
	if err := parseFloat("receiver_height_m", &options.ReceiverHeightM, 0); err != nil {
		return dummyRunOptions{}, err
	}
	if err := parseFloat("source_emission_db", &options.SourceEmission, 0); err != nil {
		return dummyRunOptions{}, err
	}
	if err := parseInt("workers", &options.Workers, 0); err != nil {
		return dummyRunOptions{}, err
	}
	if err := parseInt("chunk_size", &options.ChunkSize, 1); err != nil {
		return dummyRunOptions{}, err
	}

	if rawDisable, ok := params["disable_cache"]; ok {
		parsed, err := strconv.ParseBool(strings.TrimSpace(rawDisable))
		if err != nil {
			return dummyRunOptions{}, domainerrors.New(domainerrors.KindUserInput, "cli.parseDummyRunOptions", fmt.Sprintf("invalid disable_cache=%q", rawDisable), err)
		}
		options.DisableCache = parsed
	}

	return options, nil
}

func mergeInputPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))

	for _, rawPath := range paths {
		trimmed := strings.TrimSpace(rawPath)
		if trimmed == "" {
			continue
		}
		normalized := filepath.ToSlash(trimmed)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	return out
}

func loadValidatedModel(modelPath string, projectCRS string, sourcePath string) (modelgeojson.Model, error) {
	payload, err := os.ReadFile(modelPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return modelgeojson.Model{}, domainerrors.New(domainerrors.KindNotFound, "cli.loadValidatedModel", fmt.Sprintf("model file not found: %s", modelPath), err)
		}
		return modelgeojson.Model{}, domainerrors.New(domainerrors.KindInternal, "cli.loadValidatedModel", fmt.Sprintf("read model file %s", modelPath), err)
	}

	model, err := modelgeojson.Normalize(payload, projectCRS, sourcePath)
	if err != nil {
		return modelgeojson.Model{}, domainerrors.New(domainerrors.KindValidation, "cli.loadValidatedModel", "normalize model file", err)
	}

	report := modelgeojson.Validate(model)
	if report.ErrorCount() > 0 {
		messages := make([]string, 0, len(report.Errors))
		for _, issue := range report.Errors {
			messages = append(messages, fmt.Sprintf("%s: %s", issue.Code, issue.Message))
		}
		return modelgeojson.Model{}, domainerrors.New(domainerrors.KindValidation, "cli.loadValidatedModel", summarizeValidationErrors(messages, 5), nil)
	}

	return model, nil
}

func extractDummySources(model modelgeojson.Model, emissionDB float64) ([]freefield.Source, error) {
	sources := make([]freefield.Source, 0)
	for featureIndex, feature := range model.Features {
		if feature.Kind != "source" {
			continue
		}

		points, err := sourcePointsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractDummySources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("source-%03d", featureIndex)
		}

		for pointIndex, point := range points {
			sourceID := baseID
			if len(points) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, pointIndex+1)
			}
			sources = append(sources, freefield.Source{
				ID:         sourceID,
				Point:      point,
				EmissionDB: emissionDB,
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractDummySources", "model does not contain any supported source features", nil)
	}

	return sources, nil
}

func sourcePointsFromFeature(feature modelgeojson.Feature) ([]geo.Point2D, error) {
	switch feature.GeometryType {
	case "Point":
		point, err := parsePointCoordinate(feature.Coordinates)
		if err != nil {
			return nil, err
		}
		return []geo.Point2D{point}, nil
	case "MultiPoint":
		rawPoints, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, fmt.Errorf("geometry MultiPoint coordinates must be an array")
		}
		points := make([]geo.Point2D, 0, len(rawPoints))
		for _, raw := range rawPoints {
			point, err := parsePointCoordinate(raw)
			if err != nil {
				return nil, err
			}
			points = append(points, point)
		}
		return points, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (dummy-freefield supports Point/MultiPoint only)", feature.GeometryType)
	}
}

func parsePointCoordinate(value any) (geo.Point2D, error) {
	raw, ok := value.([]any)
	if !ok {
		return geo.Point2D{}, fmt.Errorf("point coordinates must be [x,y]")
	}
	if len(raw) < 2 {
		return geo.Point2D{}, fmt.Errorf("point coordinates must have at least 2 values")
	}

	x, err := parseCoordinateNumber(raw[0])
	if err != nil {
		return geo.Point2D{}, err
	}
	y, err := parseCoordinateNumber(raw[1])
	if err != nil {
		return geo.Point2D{}, err
	}

	point := geo.Point2D{X: x, Y: y}
	if !point.IsFinite() {
		return geo.Point2D{}, fmt.Errorf("point coordinates must be finite")
	}

	return point, nil
}

func parseCoordinateNumber(value any) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid numeric coordinate %q: %w", typed, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported coordinate type %T", value)
	}
}

func buildDummyReceivers(sources []freefield.Source, options dummyRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0, len(sources))
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.Point)
	}
	bbox, ok := geo.BBoxFromPoints(sourcePoints)
	if !ok {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildDummyReceivers", "failed to derive source extent", nil)
	}

	grid := geo.GridReceiverSet{
		ID: "grid",
		Extent: geo.BBox{
			MinX: bbox.MinX - options.GridPaddingM,
			MinY: bbox.MinY - options.GridPaddingM,
			MaxX: bbox.MaxX + options.GridPaddingM,
			MaxY: bbox.MaxY + options.GridPaddingM,
		},
		Resolution: options.GridResolutionM,
		HeightM:    options.ReceiverHeightM,
	}

	receivers, err := grid.Generate()
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildDummyReceivers", "generate receiver grid", err)
	}
	if len(receivers) == 0 {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildDummyReceivers", "receiver grid is empty", nil)
	}
	if len(receivers) > maxDummyReceivers {
		return nil, 0, 0, domainerrors.New(domainerrors.KindUserInput, "cli.buildDummyReceivers", fmt.Sprintf("receiver grid too large (%d > %d)", len(receivers), maxDummyReceivers), nil)
	}

	width, height, err := inferGridShape(receivers)
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindInternal, "cli.buildDummyReceivers", "infer receiver grid dimensions", err)
	}

	return receivers, width, height, nil
}

func inferGridShape(receivers []geo.PointReceiver) (int, int, error) {
	if len(receivers) == 0 {
		return 0, 0, fmt.Errorf("receivers are empty")
	}

	firstY := receivers[0].Point.Y
	width := 0
	for _, receiver := range receivers {
		if math.Abs(receiver.Point.Y-firstY) > 1e-9 {
			break
		}
		width++
	}
	if width <= 0 {
		return 0, 0, fmt.Errorf("invalid grid width")
	}
	if len(receivers)%width != 0 {
		return 0, 0, fmt.Errorf("receiver count %d is not divisible by inferred width %d", len(receivers), width)
	}
	return width, len(receivers) / width, nil
}

func persistDummyRunOutputs(
	runDir string,
	runOutput engine.RunOutput,
	receivers []geo.PointReceiver,
	gridWidth int,
	gridHeight int,
) (persistedRunOutputs, error) {
	resultsDir := filepath.Join(runDir, "results")
	if err := os.MkdirAll(resultsDir, 0o755); err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", fmt.Sprintf("create results directory %s", resultsDir), err)
	}

	levelByReceiver := make(map[string]float64, len(runOutput.Results))
	for _, receiverResult := range runOutput.Results {
		levelByReceiver[receiverResult.ReceiverID] = receiverResult.LevelDB
	}

	table := results.ReceiverTable{
		IndicatorOrder: []string{dummyIndicatorName},
		Unit:           dummyResultUnit,
		Records:        make([]results.ReceiverRecord, 0, len(receivers)),
	}
	for _, receiver := range receivers {
		level, ok := levelByReceiver[receiver.ID]
		if !ok {
			return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", fmt.Sprintf("missing result for receiver %s", receiver.ID), nil)
		}
		table.Records = append(table.Records, results.ReceiverRecord{
			ID:      receiver.ID,
			X:       receiver.Point.X,
			Y:       receiver.Point.Y,
			HeightM: receiver.HeightM,
			Values: map[string]float64{
				dummyIndicatorName: level,
			},
		})
	}

	receiverJSONPath := filepath.Join(resultsDir, "receivers.json")
	receiverCSVPath := filepath.Join(resultsDir, "receivers.csv")
	if err := results.SaveReceiverTableJSON(receiverJSONPath, table); err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "save receiver table json", err)
	}
	if err := results.SaveReceiverTableCSV(receiverCSVPath, table); err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "save receiver table csv", err)
	}

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     gridWidth,
		Height:    gridHeight,
		Bands:     1,
		NoData:    -9999,
		Unit:      dummyResultUnit,
		BandNames: []string{dummyIndicatorName},
	})
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "build raster", err)
	}

	for receiverIndex, receiver := range receivers {
		level := levelByReceiver[receiver.ID]
		x := receiverIndex % gridWidth
		y := receiverIndex / gridWidth
		if err := raster.Set(x, y, 0, level); err != nil {
			return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "set raster value", err)
		}
	}

	rasterBasePath := filepath.Join(resultsDir, "ldummy")
	rasterPersistence, err := results.SaveRaster(rasterBasePath, raster)
	if err != nil {
		return persistedRunOutputs{}, domainerrors.New(domainerrors.KindInternal, "cli.persistDummyRunOutputs", "save raster", err)
	}

	summary := map[string]any{
		"run_id":             runOutput.RunID,
		"status":             runOutput.Status,
		"output_hash":        runOutput.OutputHash,
		"total_chunks":       runOutput.TotalChunks,
		"used_cached_chunks": runOutput.UsedCachedChunks,
		"grid_width":         gridWidth,
		"grid_height":        gridHeight,
		"source_count":       runOutput.Metadata["source_count"],
		"receiver_count":     len(receivers),
	}
	summaryPath := filepath.Join(resultsDir, "run-summary.json")
	if err := writeJSONFile(summaryPath, summary); err != nil {
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

func finalizeRunFailure(store projectfs.Store, run project.Run, logLines []string, runErr error) error {
	finishedAt := nowUTC()
	logLines = append(logLines, fmt.Sprintf("%s run failed", finishedAt.Format(time.RFC3339)))
	if err := finalizeRun(store, run, project.RunStatusFailed, finishedAt, logLines, nil); err != nil {
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
		return err
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
	if err := store.Save(proj); err != nil {
		return err
	}

	if len(logLines) == 0 {
		logLines = []string{fmt.Sprintf("%s run finalized with status=%s", finishedAt.Format(time.RFC3339), status)}
	}
	logContent := strings.Join(logLines, "\n") + "\n"

	logPath := filepath.Join(store.Root(), filepath.FromSlash(run.LogPath))
	if err := os.WriteFile(logPath, []byte(logContent), 0o644); err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.finalizeRun", fmt.Sprintf("write run log %s", logPath), err)
	}

	return nil
}
