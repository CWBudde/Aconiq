package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"github.com/soundplan/soundplan/backend/internal/standards"
	cnossosroad "github.com/soundplan/soundplan/backend/internal/standards/cnossos/road"
	"github.com/soundplan/soundplan/backend/internal/standards/dummy/freefield"
	"github.com/spf13/cobra"
)

const (
	dummyResultUnit   = "dB"
	defaultModelPath  = ".noise/model/model.normalized.geojson"
	maxDummyReceivers = 250000
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

type cnossosRoadRunOptions struct {
	GridResolutionM        float64
	GridPaddingM           float64
	ReceiverHeightM        float64
	SurfaceType            string
	SpeedKPH               float64
	GradientPercent        float64
	TrafficDayLightVPH     float64
	TrafficDayHeavyVPH     float64
	TrafficEveningLightVPH float64
	TrafficEveningHeavyVPH float64
	TrafficNightLightVPH   float64
	TrafficNightHeavyVPH   float64
	AirAbsorptionDBPerKM   float64
	GroundAttenuationDB    float64
	BarrierAttenuationDB   float64
	MinDistanceM           float64
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

			registry, err := standards.NewRegistry()
			if err != nil {
				return domainerrors.New(domainerrors.KindInternal, "cli.run", "initialize standards registry", err)
			}

			resolvedStandard, err := registry.Resolve(standardID, standardVersion, standardProfile)
			if err != nil {
				return domainerrors.New(domainerrors.KindUserInput, "cli.run", err.Error(), nil)
			}

			resolvedParams, err := resolvedStandard.RunParameterSchema.NormalizeAndValidate(params)
			if err != nil {
				return domainerrors.New(domainerrors.KindUserInput, "cli.run", err.Error(), nil)
			}

			store, err := projectfs.New(state.Config.ProjectPath)
			if err != nil {
				return err
			}

			proj, err := store.Load()
			if err != nil {
				return err
			}

			resolvedModelPath := resolvePath(store.Root(), modelPath)
			relModelPath := relativePath(store.Root(), resolvedModelPath)
			combinedInputs := mergeInputPaths(append([]string{relModelPath}, inputPaths...))

			run, provenance, err := store.CreateRun(projectfs.CreateRunSpec{
				ScenarioID: scenarioID,
				Standard: project.StandardRef{
					ID:      resolvedStandard.StandardID,
					Version: resolvedStandard.Version,
					Profile: resolvedStandard.Profile,
				},
				Parameters: resolvedParams,
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
				fmt.Sprintf("%s standard=%s version=%s profile=%s", run.StartedAt.Format(time.RFC3339), resolvedStandard.StandardID, resolvedStandard.Version, resolvedStandard.Profile),
				fmt.Sprintf("%s model=%s", run.StartedAt.Format(time.RFC3339), relModelPath),
			}

			model, err := loadValidatedModel(resolvedModelPath, proj.CRS, relModelPath)
			if err != nil {
				logLines = append(logLines, fmt.Sprintf("%s failed to load model: %v", nowUTC().Format(time.RFC3339), err))
				return finalizeRunFailure(store, run, logLines, err)
			}

			runDir := filepath.Join(store.Root(), ".noise", "runs", run.ID)
			var persisted persistedRunOutputs
			var outputHash string
			var finishedAt time.Time
			var sourceCount int
			var receiverCount int

			switch resolvedStandard.StandardID {
			case freefield.StandardID:
				options, parseErr := parseDummyRunOptions(resolvedParams)
				if parseErr != nil {
					return parseErr
				}

				sources, extractErr := extractDummySources(model, options.SourceEmission, resolvedStandard.SupportedSourceTypes)
				if extractErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to extract sources: %v", nowUTC().Format(time.RFC3339), extractErr))
					return finalizeRunFailure(store, run, logLines, extractErr)
				}

				receivers, gridWidth, gridHeight, receiverErr := buildDummyReceivers(sources, options)
				if receiverErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
					return finalizeRunFailure(store, run, logLines, receiverErr)
				}

				sourceCount = len(sources)
				receiverCount = len(receivers)
				logLines = append(
					logLines,
					fmt.Sprintf("%s sources=%d", nowUTC().Format(time.RFC3339), sourceCount),
					fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight),
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

				runOutput, runErr := engineRunner.Run(context.Background(), engine.RunConfig{
					RunID:          run.ID,
					Workers:        options.Workers,
					ChunkSize:      options.ChunkSize,
					CacheDir:       state.Config.CacheDir,
					Receivers:      receivers,
					Sources:        engineSources,
					DisableCache:   options.DisableCache,
					DeterminismTag: "phase8-dummy-freefield",
				})
				if runErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s engine failed: %v", nowUTC().Format(time.RFC3339), runErr))
					return finalizeRunFailure(store, run, logLines, runErr)
				}

				persisted, err = persistDummyRunOutputs(runDir, runOutput, receivers, gridWidth, gridHeight, firstIndicator(resolvedStandard.SupportedIndicators))
				if err != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
					return finalizeRunFailure(store, run, logLines, err)
				}
				outputHash = runOutput.OutputHash
				finishedAt = runOutput.FinishedAt
			case cnossosroad.StandardID:
				options, parseErr := parseCnossosRoadRunOptions(resolvedParams)
				if parseErr != nil {
					return parseErr
				}

				roadSources, extractErr := extractCnossosRoadSources(model, options, resolvedStandard.SupportedSourceTypes)
				if extractErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to extract road sources: %v", nowUTC().Format(time.RFC3339), extractErr))
					return finalizeRunFailure(store, run, logLines, extractErr)
				}

				receivers, gridWidth, gridHeight, receiverErr := buildCnossosRoadReceivers(roadSources, options)
				if receiverErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to build receivers: %v", nowUTC().Format(time.RFC3339), receiverErr))
					return finalizeRunFailure(store, run, logLines, receiverErr)
				}

				sourceCount = len(roadSources)
				receiverCount = len(receivers)
				logLines = append(
					logLines,
					fmt.Sprintf("%s road_sources=%d", nowUTC().Format(time.RFC3339), sourceCount),
					fmt.Sprintf("%s receivers=%d grid=%dx%d", nowUTC().Format(time.RFC3339), receiverCount, gridWidth, gridHeight),
				)

				receiverOutputs, computeErr := cnossosroad.ComputeReceiverOutputs(receivers, roadSources, options.PropagationConfig())
				if computeErr != nil {
					logLines = append(logLines, fmt.Sprintf("%s cnossos compute failed: %v", nowUTC().Format(time.RFC3339), computeErr))
					return finalizeRunFailure(store, run, logLines, computeErr)
				}

				persisted, outputHash, finishedAt, err = persistCnossosRoadRunOutputs(runDir, receiverOutputs, gridWidth, gridHeight, sourceCount)
				if err != nil {
					logLines = append(logLines, fmt.Sprintf("%s failed to persist outputs: %v", nowUTC().Format(time.RFC3339), err))
					return finalizeRunFailure(store, run, logLines, err)
				}
			default:
				return domainerrors.New(
					domainerrors.KindUserInput,
					"cli.run",
					fmt.Sprintf("standard %q is registered but not wired in run pipeline yet", resolvedStandard.StandardID),
					nil,
				)
			}

			artifacts := buildRunArtifacts(store.Root(), run.ID, persisted)

			logLines = append(
				logLines,
				fmt.Sprintf("%s output_hash=%s", nowUTC().Format(time.RFC3339), outputHash),
				fmt.Sprintf("%s persisted=%s", nowUTC().Format(time.RFC3339), relativePath(store.Root(), persisted.SummaryPath)),
				fmt.Sprintf("%s run completed", nowUTC().Format(time.RFC3339)),
			)

			if err := finalizeRun(store, run, project.RunStatusCompleted, finishedAt, logLines, artifacts); err != nil {
				return err
			}

			state.Logger.Info(
				"run completed",
				"run_id", run.ID,
				"status", project.RunStatusCompleted,
				"standard_id", run.Standard.ID,
				"provenance", provenance.ManifestPath,
				"output_hash", outputHash,
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
	cmd.Flags().StringVar(&standardVersion, "standard-version", "", "Standard version (defaults to standard default)")
	cmd.Flags().StringVar(&standardProfile, "standard-profile", "", "Standard profile (defaults to version profile default)")
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
	options := dummyRunOptions{}

	parseFloat := func(key string, target *float64, min float64) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, "cli.parseDummyRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
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
			return domainerrors.New(domainerrors.KindInternal, "cli.parseDummyRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
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

	rawDisable, ok := params["disable_cache"]
	if !ok {
		return dummyRunOptions{}, domainerrors.New(domainerrors.KindInternal, "cli.parseDummyRunOptions", `normalized parameter "disable_cache" missing`, nil)
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(rawDisable))
	if err != nil {
		return dummyRunOptions{}, domainerrors.New(domainerrors.KindUserInput, "cli.parseDummyRunOptions", fmt.Sprintf("invalid disable_cache=%q", rawDisable), err)
	}
	options.DisableCache = parsed

	return options, nil
}

func parseCnossosRoadRunOptions(params map[string]string) (cnossosRoadRunOptions, error) {
	options := cnossosRoadRunOptions{}

	parseFloat := func(key string, target *float64) error {
		value, ok := params[key]
		if !ok {
			return domainerrors.New(domainerrors.KindInternal, "cli.parseCnossosRoadRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return domainerrors.New(domainerrors.KindUserInput, "cli.parseCnossosRoadRunOptions", fmt.Sprintf("invalid %s=%q", key, value), err)
		}
		*target = parsed
		return nil
	}

	getString := func(key string) (string, error) {
		value, ok := params[key]
		if !ok {
			return "", domainerrors.New(domainerrors.KindInternal, "cli.parseCnossosRoadRunOptions", fmt.Sprintf("normalized parameter %q missing", key), nil)
		}
		return strings.TrimSpace(value), nil
	}

	if err := parseFloat("grid_resolution_m", &options.GridResolutionM); err != nil {
		return cnossosRoadRunOptions{}, err
	}
	if err := parseFloat("grid_padding_m", &options.GridPaddingM); err != nil {
		return cnossosRoadRunOptions{}, err
	}
	if err := parseFloat("receiver_height_m", &options.ReceiverHeightM); err != nil {
		return cnossosRoadRunOptions{}, err
	}

	surfaceType, err := getString("road_surface_type")
	if err != nil {
		return cnossosRoadRunOptions{}, err
	}
	options.SurfaceType = surfaceType

	if err := parseFloat("road_speed_kph", &options.SpeedKPH); err != nil {
		return cnossosRoadRunOptions{}, err
	}
	if err := parseFloat("road_gradient_percent", &options.GradientPercent); err != nil {
		return cnossosRoadRunOptions{}, err
	}
	if err := parseFloat("traffic_day_light_vph", &options.TrafficDayLightVPH); err != nil {
		return cnossosRoadRunOptions{}, err
	}
	if err := parseFloat("traffic_day_heavy_vph", &options.TrafficDayHeavyVPH); err != nil {
		return cnossosRoadRunOptions{}, err
	}
	if err := parseFloat("traffic_evening_light_vph", &options.TrafficEveningLightVPH); err != nil {
		return cnossosRoadRunOptions{}, err
	}
	if err := parseFloat("traffic_evening_heavy_vph", &options.TrafficEveningHeavyVPH); err != nil {
		return cnossosRoadRunOptions{}, err
	}
	if err := parseFloat("traffic_night_light_vph", &options.TrafficNightLightVPH); err != nil {
		return cnossosRoadRunOptions{}, err
	}
	if err := parseFloat("traffic_night_heavy_vph", &options.TrafficNightHeavyVPH); err != nil {
		return cnossosRoadRunOptions{}, err
	}
	if err := parseFloat("air_absorption_db_per_km", &options.AirAbsorptionDBPerKM); err != nil {
		return cnossosRoadRunOptions{}, err
	}
	if err := parseFloat("ground_attenuation_db", &options.GroundAttenuationDB); err != nil {
		return cnossosRoadRunOptions{}, err
	}
	if err := parseFloat("barrier_attenuation_db", &options.BarrierAttenuationDB); err != nil {
		return cnossosRoadRunOptions{}, err
	}
	if err := parseFloat("min_distance_m", &options.MinDistanceM); err != nil {
		return cnossosRoadRunOptions{}, err
	}

	return options, nil
}

func (o cnossosRoadRunOptions) PropagationConfig() cnossosroad.PropagationConfig {
	return cnossosroad.PropagationConfig{
		AirAbsorptionDBPerKM: o.AirAbsorptionDBPerKM,
		GroundAttenuationDB:  o.GroundAttenuationDB,
		BarrierAttenuationDB: o.BarrierAttenuationDB,
		MinDistanceM:         o.MinDistanceM,
	}
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

func firstIndicator(indicators []string) string {
	for _, indicator := range indicators {
		trimmed := strings.TrimSpace(indicator)
		if trimmed != "" {
			return trimmed
		}
	}
	return freefield.IndicatorLdummy
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

func extractDummySources(model modelgeojson.Model, emissionDB float64, supportedSourceTypes []string) ([]freefield.Source, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}
		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]freefield.Source, 0)
	for featureIndex, feature := range model.Features {
		if feature.Kind != "source" {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractDummySources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
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

func extractCnossosRoadSources(model modelgeojson.Model, options cnossosRoadRunOptions, supportedSourceTypes []string) ([]cnossosroad.RoadSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}
		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]cnossosroad.RoadSource, 0)
	for featureIndex, feature := range model.Features {
		if feature.Kind != "source" {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractCnossosRoadSources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		lines, err := lineStringsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("road-source-%03d", featureIndex)
		}

		for lineIndex, line := range lines {
			sourceID := baseID
			if len(lines) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}
			sources = append(sources, cnossosroad.RoadSource{
				ID:              sourceID,
				Centerline:      line,
				SurfaceType:     options.SurfaceType,
				SpeedKPH:        options.SpeedKPH,
				GradientPercent: options.GradientPercent,
				TrafficDay: cnossosroad.TrafficPeriod{
					LightVehiclesPerHour: options.TrafficDayLightVPH,
					HeavyVehiclesPerHour: options.TrafficDayHeavyVPH,
				},
				TrafficEvening: cnossosroad.TrafficPeriod{
					LightVehiclesPerHour: options.TrafficEveningLightVPH,
					HeavyVehiclesPerHour: options.TrafficEveningHeavyVPH,
				},
				TrafficNight: cnossosroad.TrafficPeriod{
					LightVehiclesPerHour: options.TrafficNightLightVPH,
					HeavyVehiclesPerHour: options.TrafficNightHeavyVPH,
				},
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", "model does not contain any supported line source features", nil)
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

func lineStringsFromFeature(feature modelgeojson.Feature) ([][]geo.Point2D, error) {
	switch feature.GeometryType {
	case "LineString":
		line, err := parseLineStringCoordinates(feature.Coordinates)
		if err != nil {
			return nil, err
		}
		return [][]geo.Point2D{line}, nil
	case "MultiLineString":
		rawLines, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, fmt.Errorf("geometry MultiLineString coordinates must be an array")
		}
		lines := make([][]geo.Point2D, 0, len(rawLines))
		for _, rawLine := range rawLines {
			line, err := parseLineStringCoordinates(rawLine)
			if err != nil {
				return nil, err
			}
			lines = append(lines, line)
		}
		return lines, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (cnossos-road supports LineString/MultiLineString only)", feature.GeometryType)
	}
}

func parseLineStringCoordinates(value any) ([]geo.Point2D, error) {
	rawPoints, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("line coordinates must be an array")
	}
	if len(rawPoints) < 2 {
		return nil, fmt.Errorf("line coordinates must contain at least 2 points")
	}
	points := make([]geo.Point2D, 0, len(rawPoints))
	for _, rawPoint := range rawPoints {
		point, err := parsePointCoordinate(rawPoint)
		if err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, nil
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

func buildCnossosRoadReceivers(sources []cnossosroad.RoadSource, options cnossosRoadRunOptions) ([]geo.PointReceiver, int, int, error) {
	sourcePoints := make([]geo.Point2D, 0)
	for _, source := range sources {
		sourcePoints = append(sourcePoints, source.Centerline...)
	}
	bbox, ok := geo.BBoxFromPoints(sourcePoints)
	if !ok {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosRoadReceivers", "failed to derive source extent", nil)
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
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosRoadReceivers", "generate receiver grid", err)
	}
	if len(receivers) == 0 {
		return nil, 0, 0, domainerrors.New(domainerrors.KindValidation, "cli.buildCnossosRoadReceivers", "receiver grid is empty", nil)
	}
	if len(receivers) > maxDummyReceivers {
		return nil, 0, 0, domainerrors.New(domainerrors.KindUserInput, "cli.buildCnossosRoadReceivers", fmt.Sprintf("receiver grid too large (%d > %d)", len(receivers), maxDummyReceivers), nil)
	}

	width, height, err := inferGridShape(receivers)
	if err != nil {
		return nil, 0, 0, domainerrors.New(domainerrors.KindInternal, "cli.buildCnossosRoadReceivers", "infer receiver grid dimensions", err)
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
	indicator string,
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
		IndicatorOrder: []string{indicator},
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
				indicator: level,
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
		BandNames: []string{indicator},
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

	rasterBasePath := filepath.Join(resultsDir, strings.ToLower(indicator))
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

func persistCnossosRoadRunOutputs(
	runDir string,
	outputs []cnossosroad.ReceiverOutput,
	gridWidth int,
	gridHeight int,
	sourceCount int,
) (persistedRunOutputs, string, time.Time, error) {
	resultsDir := filepath.Join(runDir, "results")
	exported, err := cnossosroad.ExportResultBundle(resultsDir, outputs, gridWidth, gridHeight)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosRoadRunOutputs", "export cnossos road results", err)
	}

	outputHash, err := hashCnossosRoadOutputs(outputs)
	if err != nil {
		return persistedRunOutputs{}, "", time.Time{}, domainerrors.New(domainerrors.KindInternal, "cli.persistCnossosRoadRunOutputs", "hash cnossos outputs", err)
	}

	summary := map[string]any{
		"run_id":         filepath.Base(runDir),
		"status":         project.RunStatusCompleted,
		"output_hash":    outputHash,
		"grid_width":     gridWidth,
		"grid_height":    gridHeight,
		"source_count":   sourceCount,
		"receiver_count": len(outputs),
	}
	summaryPath := filepath.Join(resultsDir, "run-summary.json")
	if err := writeJSONFile(summaryPath, summary); err != nil {
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

func hashCnossosRoadOutputs(outputs []cnossosroad.ReceiverOutput) (string, error) {
	type record struct {
		ReceiverID string                         `json:"receiver_id"`
		Indicators cnossosroad.ReceiverIndicators `json:"indicators"`
	}
	records := make([]record, 0, len(outputs))
	for _, output := range outputs {
		records = append(records, record{
			ReceiverID: output.Receiver.ID,
			Indicators: output.Indicators,
		})
	}
	payload, err := json.Marshal(records)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func buildRunArtifacts(projectRoot string, runID string, persisted persistedRunOutputs) []project.ArtifactRef {
	now := nowUTC()
	return []project.ArtifactRef{
		{
			ID:        fmt.Sprintf("artifact-run-%s-receivers-json", runID),
			RunID:     runID,
			Kind:      "run.result.receiver_table_json",
			Path:      relativePath(projectRoot, persisted.ReceiverJSONPath),
			CreatedAt: now,
		},
		{
			ID:        fmt.Sprintf("artifact-run-%s-receivers-csv", runID),
			RunID:     runID,
			Kind:      "run.result.receiver_table_csv",
			Path:      relativePath(projectRoot, persisted.ReceiverCSVPath),
			CreatedAt: now,
		},
		{
			ID:        fmt.Sprintf("artifact-run-%s-raster-meta", runID),
			RunID:     runID,
			Kind:      "run.result.raster_metadata",
			Path:      relativePath(projectRoot, persisted.RasterMetadataPath),
			CreatedAt: now,
		},
		{
			ID:        fmt.Sprintf("artifact-run-%s-raster-data", runID),
			RunID:     runID,
			Kind:      "run.result.raster_binary",
			Path:      relativePath(projectRoot, persisted.RasterDataPath),
			CreatedAt: now,
		},
		{
			ID:        fmt.Sprintf("artifact-run-%s-summary", runID),
			RunID:     runID,
			Kind:      "run.result.summary",
			Path:      relativePath(projectRoot, persisted.SummaryPath),
			CreatedAt: now,
		},
	}
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
