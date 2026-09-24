package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/aconiq/backend/internal/acoustics"
	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/engine"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/report/results"
	"github.com/aconiq/backend/internal/standards/framework"
)

// runLog is the run's log as it is being written. Every line carries the time
// it was appended, which is why the pipeline never builds one by hand: the
// order of these lines is the record of what the run did, and a failure path
// has to leave the same trail as a success.
type runLog struct {
	lines []string
}

func newRunLog(lines ...string) *runLog {
	return &runLog{lines: lines}
}

func (l *runLog) addf(format string, args ...any) {
	l.lines = append(l.lines, nowUTC().Format(time.RFC3339)+" "+fmt.Sprintf(format, args...))
}

// addAtf appends a line stamped with a time the caller owns, for events that
// carry their own — the engine's progress stream, above all, whose ordering
// against the wall clock is the point of the timestamp.
func (l *runLog) addAtf(at time.Time, format string, args ...any) {
	l.lines = append(l.lines, at.Format(time.RFC3339)+" "+fmt.Sprintf(format, args...))
}

// addLines appends lines a module produced with their own timestamps already
// applied, such as the Schall 03 chain's.
func (l *runLog) addLines(lines []string) {
	l.lines = append(l.lines, lines...)
}

// addReceiverCount records how the receiver set was arrived at, which differs
// between an explicit set and a generated grid.
func (l *runLog) addReceiverCount(receiverMode string, receiverCount int, gridWidth int, gridHeight int) {
	if receiverMode == receiverModeCustom {
		l.addf("receivers=%d set=%s", receiverCount, explicitReceiverSetID)

		return
	}

	l.addf("receivers=%d grid=%dx%d", receiverCount, gridWidth, gridHeight)
}

// addGridExtent records which extent the automatic receiver grid was built
// over. It is the line a user reads to answer "did it use the area I drew?".
// Custom receiver mode builds no grid, so there is nothing to record.
func (l *runLog) addGridExtent(receiverMode string, calcArea *geo.BBox, layout results.GridLayout) {
	for _, line := range gridExtentLines(receiverMode, calcArea, layout) {
		l.addf("%s", line)
	}
}

// gridExtentLines is the grid's run.log record: the extent, and — only when a
// building masked any cell, so a model without buildings logs what it always
// did — how many cells the raster leaves at nodata.
func gridExtentLines(receiverMode string, calcArea *geo.BBox, layout results.GridLayout) []string {
	if receiverMode == receiverModeCustom {
		return nil
	}

	lines := []string{"grid_extent=" + gridExtentLabel(calcArea)}
	if masked := layout.NoDataCount(); masked > 0 {
		lines = append(lines, fmt.Sprintf("grid_masked_cells=%d (receivers inside a building footprint; computed, written to the raster as nodata)", masked))
	}

	return lines
}

func (l *runLog) all() []string {
	return l.lines
}

// runModuleInput is everything a standard's run needs from the pipeline. It is
// deliberately the same for all of them: a module that needs something not in
// here is a module the pipeline does not yet support, which is a decision to
// take rather than a field to add to one case.
type runModuleInput struct {
	standard     framework.ResolvedProfile
	params       map[string]string
	model        modelgeojson.Model
	terrain      terrain.Model
	receiverMode string
	workers      int
	runDir       string
	runID        string
	cacheDir     string
	log          *runLog

	// projection is the CRS the model was moved into before anything read a
	// coordinate off it, and the CRS every result is therefore expressed in.
	// It reaches the run summary, which is the only artifact a consumer of the
	// receiver table can read the CRS back from.
	projection computeProjection

	// mergeProvenance completes the run manifest with metadata that is only
	// knowable once the module has run — which chain Schall 03 resolved, so
	// far. It is a callback rather than a return value because the point at
	// which it is called is observable: a failure there must leave no results
	// behind.
	mergeProvenance func(metadata map[string]string) error
}

// runModuleResult is what the pipeline needs back in order to finish the run.
type runModuleResult struct {
	persisted  persistedRunOutputs
	outputHash string
	finishedAt time.Time
}

// runModule executes one standard. Implementations log what they did through
// input.log — including the line describing their own failure — and return the
// error; the pipeline owns what happens to the run afterwards.
type runModule func(runModuleInput) (runModuleResult, error)

// beforeRunError marks a failure that happened before the module touched
// anything: option parsing, and the receiver mode a standard refuses. The
// pipeline returns these as they are rather than marking the run failed, which
// is what it did before this dispatch existed — the run is left in `running`.
// That is questionable and is not this refactor's question; PLAN.md Priority 3
// owns the exit-code and run-state taxonomy.
type beforeRunError struct {
	err error
}

func (e beforeRunError) Error() string {
	return e.err.Error()
}

func (e beforeRunError) Unwrap() error {
	return e.err
}

// receiverRunModule is the shape nine of the thirteen standards share: parse
// options, extract typed sources, resolve a receiver set, compute receiver
// outputs, persist. What differs between them is the five functions and the
// three strings below — which is precisely what the 562-line switch used to
// spell out once per standard.
type receiverRunModule[Opt any, Src any, Out any] struct {
	// sourceCountKey names the source count in the run log, e.g. `road_sources`.
	sourceCountKey string

	// extractFailure and computeFailure are the run-log lines for a failure in
	// either step. They are per-standard because the log is a user-facing
	// record of what went wrong.
	extractFailure string
	computeFailure string

	parseOptions   func(map[string]string) (Opt, error)
	extract        func(modelgeojson.Model, Opt, []string) ([]Src, error)
	buildReceivers func([]Src, *geo.BBox, Opt) ([]geo.PointReceiver, results.GridLayout, error)
	compute        func([]geo.PointReceiver, []Src, Opt) ([]Out, error)
	persist        func(runDir string, outputs []Out, layout results.GridLayout, sourceCount int, receiverMode string, tier framework.EvidenceTier, projection computeProjection) (persistedRunOutputs, string, time.Time, error)
}

func (m receiverRunModule[Opt, Src, Out]) run(input runModuleInput) (runModuleResult, error) {
	options, err := m.parseOptions(input.params)
	if err != nil {
		return runModuleResult{}, beforeRunError{err: err}
	}

	sources, err := m.extract(input.model, options, input.standard.SupportedSourceTypes)
	if err != nil {
		input.log.addf("%s: %v", m.extractFailure, err)

		return runModuleResult{}, err
	}

	// The calculation area comes off input.model, which runModuleInput already
	// carries. Adding a field for it would be a decision about all thirteen
	// standards (see the comment on runModuleInput); this does not need one.
	receivers, layout, calcArea, err := resolveGridReceivers(input.model, input.receiverMode, func(calcArea *geo.BBox) ([]geo.PointReceiver, results.GridLayout, error) {
		return m.buildReceivers(sources, calcArea, options)
	})
	if err != nil {
		input.log.addf("failed to build receivers: %v", err)

		return runModuleResult{}, err
	}

	input.log.addf("%s=%d", m.sourceCountKey, len(sources))
	input.log.addReceiverCount(input.receiverMode, len(receivers), layout.Width, layout.Height)
	input.log.addGridExtent(input.receiverMode, calcArea, layout)

	outputs, err := m.compute(receivers, sources, options)
	if err != nil {
		input.log.addf("%s: %v", m.computeFailure, err)

		return runModuleResult{}, err
	}

	persisted, outputHash, finishedAt, err := m.persist(
		input.runDir, outputs, layout, len(sources), input.receiverMode, input.standard.EvidenceTier,
		input.projection,
	)
	if err != nil {
		input.log.addf("failed to persist outputs: %v", err)

		return runModuleResult{}, err
	}

	return runModuleResult{
		persisted:  persisted,
		outputHash: outputHash,
		finishedAt: finishedAt,
	}, nil
}

// endPersist binds the shared END persist path to one standard, so a table
// entry names its standard once. Every END module computes
// acoustics.ReceiverOutput, which is what lets one persist function serve them.
func endPersist(standardID string) func(string, []acoustics.ReceiverOutput, results.GridLayout, int, string, framework.EvidenceTier, computeProjection) (persistedRunOutputs, string, time.Time, error) {
	return func(runDir string, outputs []acoustics.ReceiverOutput, layout results.GridLayout, sourceCount int, receiverMode string, tier framework.EvidenceTier, projection computeProjection) (persistedRunOutputs, string, time.Time, error) {
		return persistENDRunOutputs(standardID, runDir, outputs, layout, sourceCount, receiverMode, tier, projection)
	}
}

// runDummyModule is the one standard that goes through the compute engine.
// dummy-freefield is a test fixture, and it is also the only module the engine
// can currently drive — generalising that is PLAN.md Priority 7's "generalise
// the engine".
func runDummyModule(input runModuleInput) (runModuleResult, error) {
	options, err := parseDummyRunOptions(input.params)
	if err != nil {
		return runModuleResult{}, beforeRunError{err: err}
	}

	sources, err := extractDummySources(input.model, options.SourceEmission, input.standard.SupportedSourceTypes)
	if err != nil {
		input.log.addf("failed to extract sources: %v", err)

		return runModuleResult{}, err
	}

	receivers, layout, calcArea, err := resolveGridReceivers(input.model, input.receiverMode, func(calcArea *geo.BBox) ([]geo.PointReceiver, results.GridLayout, error) {
		return buildDummyReceivers(sources, calcArea, options)
	})
	if err != nil {
		input.log.addf("failed to build receivers: %v", err)

		return runModuleResult{}, err
	}

	input.log.addf("sources=%d", len(sources))
	input.log.addReceiverCount(input.receiverMode, len(receivers), layout.Width, layout.Height)
	input.log.addGridExtent(input.receiverMode, calcArea, layout)

	engineRunner := engine.NewRunner(func(event engine.ProgressEvent) {
		if event.Stage == "compute" && event.Message == "chunk_done" {
			input.log.addAtf(event.Time, "stage=%s chunk=%d %d/%d", event.Stage, event.ChunkIndex, event.CompletedChunks, event.TotalChunks)

			return
		}

		input.log.addAtf(event.Time, "stage=%s message=%s", event.Stage, event.Message)
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
		RunID:          input.runID,
		Workers:        options.Workers,
		ChunkSize:      options.ChunkSize,
		CacheDir:       input.cacheDir,
		Receivers:      receivers,
		Sources:        engineSources,
		DisableCache:   options.DisableCache,
		DeterminismTag: "dummy-freefield",
		StandardKey: engine.StandardKey{
			StandardID: input.standard.StandardID,
			Version:    input.standard.Version,
			Profile:    input.standard.Profile,
		},
	})
	if err != nil {
		input.log.addf("engine failed: %v", err)

		return runModuleResult{}, fmt.Errorf("run compute engine: %w", err)
	}

	persisted, err := persistDummyRunOutputs(
		input.runDir, runOutput, receivers, layout,
		firstIndicator(input.standard.SupportedIndicators), input.standard.EvidenceTier, input.projection,
	)
	if err != nil {
		input.log.addf("failed to persist outputs: %v", err)

		return runModuleResult{}, err
	}

	return runModuleResult{
		persisted:  persisted,
		outputHash: runOutput.OutputHash,
		finishedAt: runOutput.FinishedAt,
	}, nil
}

// runModuleFor returns the module registered for a standard id. A standard the
// registry offers but this table does not carry is a wiring gap, and says so:
// TestEveryRegisteredStandardCompletesARun fails the moment one appears.
func runModuleFor(standardID string) (runModule, error) {
	module, ok := runModuleTable[standardID]
	if !ok {
		return nil, domainerrors.New(
			domainerrors.KindUserInput,
			"cli.run",
			fmt.Sprintf("standard %q is registered but not wired in run pipeline yet", standardID),
			nil,
		)
	}

	return module, nil
}
