package engine

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
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
)

// Runner executes chunked compute runs.
type Runner struct {
	progress ProgressSink
}

func NewRunner(progress ProgressSink) *Runner {
	return &Runner{progress: progress}
}

func (r *Runner) Run(ctx context.Context, cfg RunConfig) (RunOutput, error) {
	cfg = normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return RunOutput{}, err
	}

	startedAt := time.Now().UTC()
	runDir := filepath.Join(cfg.CacheDir, cfg.RunID)
	chunksDir := filepath.Join(runDir, "chunks")
	outputPath := filepath.Join(runDir, "run-output.json")
	statePath := filepath.Join(runDir, "run-state.json")

	if err := os.MkdirAll(chunksDir, 0o755); err != nil {
		return RunOutput{}, fmt.Errorf("create run cache directory: %w", err)
	}

	r.emit(cfg.RunID, "load", "start", -1, 0, 0)

	if err := writeRunState(statePath, RunState{
		RunID:           cfg.RunID,
		Status:          RunStateRunning,
		UpdatedAt:       time.Now().UTC(),
		TotalChunks:     0,
		CompletedChunks: 0,
		Message:         "load",
	}); err != nil {
		return RunOutput{}, err
	}

	r.emit(cfg.RunID, "load", "done", -1, 0, 0)

	r.emit(cfg.RunID, "prepare", "start", -1, 0, 0)

	if _, err := buildSourceIndex(cfg); err != nil {
		return RunOutput{}, err
	}

	r.emit(cfg.RunID, "prepare", "done", -1, 0, 0)

	r.emit(cfg.RunID, "chunk", "start", -1, 0, 0)
	chunks := chunkReceivers(cfg.Receivers, cfg.ChunkSize)

	totalChunks := len(chunks)
	if err := writeRunState(statePath, RunState{
		RunID:           cfg.RunID,
		Status:          RunStateRunning,
		UpdatedAt:       time.Now().UTC(),
		TotalChunks:     totalChunks,
		CompletedChunks: 0,
		Message:         "chunk",
	}); err != nil {
		return RunOutput{}, err
	}

	r.emit(cfg.RunID, "chunk", "done", -1, 0, totalChunks)

	r.emit(cfg.RunID, "compute", "start", -1, 0, totalChunks)
	ffSources := convertSourcesToFreefield(cfg.Sources)

	received, usedCached, err := r.computeChunks(ctx, cfg, chunks, ffSources, chunksDir, statePath)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			_ = cleanupTmpFiles(chunksDir)
			_ = writeRunState(statePath, RunState{
				RunID:           cfg.RunID,
				Status:          RunStateCanceled,
				UpdatedAt:       time.Now().UTC(),
				TotalChunks:     totalChunks,
				CompletedChunks: len(received),
				Message:         "canceled",
			})
			r.emit(cfg.RunID, "compute", "canceled", -1, len(received), totalChunks)

			return RunOutput{}, context.Canceled
		}

		_ = cleanupTmpFiles(chunksDir)
		_ = writeRunState(statePath, RunState{
			RunID:           cfg.RunID,
			Status:          RunStateFailed,
			UpdatedAt:       time.Now().UTC(),
			TotalChunks:     totalChunks,
			CompletedChunks: len(received),
			Message:         err.Error(),
		})

		return RunOutput{}, err
	}

	r.emit(cfg.RunID, "compute", "done", -1, len(received), totalChunks)

	r.emit(cfg.RunID, "reduce", "start", -1, len(received), totalChunks)
	results := reduceDeterministic(received)

	hash, err := hashResults(results)
	if err != nil {
		return RunOutput{}, err
	}

	r.emit(cfg.RunID, "reduce", "done", -1, len(received), totalChunks)

	finishedAt := time.Now().UTC()
	output := RunOutput{
		RunID:            cfg.RunID,
		Status:           RunStateCompleted,
		StartedAt:        startedAt,
		FinishedAt:       finishedAt,
		Results:          results,
		OutputHash:       hash,
		TotalChunks:      totalChunks,
		UsedCachedChunks: usedCached,
		Metadata: map[string]any{
			"workers":      cfg.Workers,
			"chunk_size":   cfg.ChunkSize,
			"determinism":  cfg.DeterminismTag,
			"source_count": len(cfg.Sources),
		},
	}

	r.emit(cfg.RunID, "persist", "start", -1, len(received), totalChunks)

	if err := writeJSONFile(outputPath, output); err != nil {
		return RunOutput{}, err
	}

	if err := writeRunState(statePath, RunState{
		RunID:           cfg.RunID,
		Status:          RunStateCompleted,
		UpdatedAt:       finishedAt,
		TotalChunks:     totalChunks,
		CompletedChunks: totalChunks,
		Message:         "persisted",
	}); err != nil {
		return RunOutput{}, err
	}

	r.emit(cfg.RunID, "persist", "done", -1, len(received), totalChunks)

	return output, nil
}

func normalizeConfig(cfg RunConfig) RunConfig {
	if cfg.Workers <= 0 {
		cfg.Workers = max(runtime.NumCPU(), 1)
	}

	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 128
	}

	if cfg.SourceIndexCellM <= 0 {
		cfg.SourceIndexCellM = 100
	}

	return cfg
}

func validateConfig(cfg RunConfig) error {
	if cfg.RunID == "" {
		return errors.New("engine run_id is required")
	}

	if cfg.CacheDir == "" {
		return errors.New("engine cache_dir is required")
	}

	if len(cfg.Receivers) == 0 {
		return errors.New("engine requires at least one receiver")
	}

	if len(cfg.Sources) == 0 {
		return errors.New("engine requires at least one source")
	}

	for _, receiver := range cfg.Receivers {
		if receiver.ID == "" || !receiver.Point.IsFinite() {
			return errors.New("invalid receiver in input")
		}
	}

	for _, source := range cfg.Sources {
		if source.ID == "" || !source.Point.IsFinite() || math.IsNaN(source.Emission) || math.IsInf(source.Emission, 0) {
			return errors.New("invalid source in input")
		}
	}

	return nil
}

func buildSourceIndex(cfg RunConfig) (geo.SpatialIndex, error) {
	index, err := geo.NewGridSpatialIndex(cfg.SourceIndexCellM)
	if err != nil {
		return nil, err
	}

	for _, source := range cfg.Sources {
		err := index.Insert(geo.IndexedItem{
			ID: source.ID,
			BBox: geo.BBox{
				MinX: source.Point.X,
				MinY: source.Point.Y,
				MaxX: source.Point.X,
				MaxY: source.Point.Y,
			},
		})
		if err != nil {
			return nil, err
		}
	}

	return index, nil
}

type receiverChunk struct {
	Index     int
	Receivers []Receiver
}

func chunkReceivers(receivers []Receiver, chunkSize int) []receiverChunk {
	chunks := make([]receiverChunk, 0, (len(receivers)+chunkSize-1)/chunkSize)
	for start, index := 0, 0; start < len(receivers); start, index = start+chunkSize, index+1 {
		end := min(start+chunkSize, len(receivers))

		receiverCopy := append([]Receiver(nil), receivers[start:end]...)
		chunks = append(chunks, receiverChunk{Index: index, Receivers: receiverCopy})
	}

	return chunks
}

type chunkComputeResult struct {
	chunkIndex int
	results    []ReceiverResult
	fromCache  bool
	err        error
}

func (r *Runner) computeChunks(
	ctx context.Context,
	cfg RunConfig,
	chunks []receiverChunk,
	ffSources []freefield.Source,
	chunksDir string,
	statePath string,
) (map[int][]ReceiverResult, int, error) {
	jobs := make(chan receiverChunk)
	resultsCh := make(chan chunkComputeResult, len(chunks))

	workerCount := max(min(cfg.Workers, len(chunks)), 1)

	var wg sync.WaitGroup
	for range workerCount {
		wg.Go(func() {
			for chunk := range jobs {
				res, fromCache, err := computeOrLoadChunk(ctx, cfg, chunk, ffSources, chunksDir)
				resultsCh <- chunkComputeResult{chunkIndex: chunk.Index, results: res, fromCache: fromCache, err: err}

				if err != nil {
					return
				}
			}
		})
	}

	go func() {
		defer close(jobs)

		for _, chunk := range chunks {
			select {
			case <-ctx.Done():
				return
			case jobs <- chunk:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	received := make(map[int][]ReceiverResult, len(chunks))
	usedCached := 0
	completed := 0

	for result := range resultsCh {
		if result.err != nil {
			if errors.Is(result.err, context.Canceled) {
				return received, usedCached, context.Canceled
			}

			return received, usedCached, result.err
		}

		received[result.chunkIndex] = result.results
		if result.fromCache {
			usedCached++
		}

		completed++
		r.emit(cfg.RunID, "compute", "chunk_done", result.chunkIndex, completed, len(chunks))
		_ = writeRunState(statePath, RunState{
			RunID:           cfg.RunID,
			Status:          RunStateRunning,
			UpdatedAt:       time.Now().UTC(),
			TotalChunks:     len(chunks),
			CompletedChunks: completed,
			Message:         "compute",
		})
	}

	if ctx.Err() != nil {
		return received, usedCached, context.Canceled
	}

	return received, usedCached, nil
}

func computeOrLoadChunk(
	ctx context.Context,
	cfg RunConfig,
	chunk receiverChunk,
	ffSources []freefield.Source,
	chunksDir string,
) ([]ReceiverResult, bool, error) {
	cachePath := filepath.Join(chunksDir, fmt.Sprintf("chunk-%06d.json", chunk.Index))
	if !cfg.DisableCache {
		if cached, ok, err := readChunk(cachePath); err != nil {
			return nil, false, err
		} else if ok {
			return cached, true, nil
		}
	}

	results := make([]ReceiverResult, 0, len(chunk.Receivers))
	for _, receiver := range chunk.Receivers {
		select {
		case <-ctx.Done():
			return nil, false, context.Canceled
		default:
		}

		if cfg.ComputeDelay > 0 {
			select {
			case <-ctx.Done():
				return nil, false, context.Canceled
			case <-time.After(cfg.ComputeDelay):
			}
		}

		level := freefield.ComputeReceiverLevelDB(receiver.Point, ffSources)
		results = append(results, ReceiverResult{ReceiverID: receiver.ID, LevelDB: level})
	}

	if !cfg.DisableCache {
		err := writeChunk(cachePath, results)
		if err != nil {
			return nil, false, err
		}
	}

	return results, false, nil
}

func convertSourcesToFreefield(sources []Source) []freefield.Source {
	out := make([]freefield.Source, 0, len(sources))
	for _, source := range sources {
		out = append(out, freefield.Source{
			ID:         source.ID,
			Point:      source.Point,
			EmissionDB: source.Emission,
		})
	}

	return out
}

func reduceDeterministic(chunks map[int][]ReceiverResult) []ReceiverResult {
	indices := make([]int, 0, len(chunks))
	for idx := range chunks {
		indices = append(indices, idx)
	}

	sort.Ints(indices)

	merged := make([]ReceiverResult, 0)
	for _, idx := range indices {
		merged = append(merged, chunks[idx]...)
	}

	return merged
}

func hashResults(results []ReceiverResult) (string, error) {
	payload, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("marshal results for hash: %w", err)
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func readChunk(path string) ([]ReceiverResult, bool, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("read chunk cache %s: %w", path, err)
	}

	var results []ReceiverResult
	if err := json.Unmarshal(payload, &results); err != nil {
		return nil, false, fmt.Errorf("decode chunk cache %s: %w", path, err)
	}

	return results, true, nil
}

func writeChunk(path string, results []ReceiverResult) error {
	tmpPath := path + ".tmp"

	err := writeJSONFile(tmpPath, results)
	if err != nil {
		return err
	}

	err = os.Rename(tmpPath, path)
	if err != nil {
		return fmt.Errorf("persist chunk cache %s: %w", path, err)
	}

	return nil
}

func writeRunState(path string, state RunState) error {
	return writeJSONFile(path, state)
}

func writeJSONFile(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json %s: %w", path, err)
	}

	encoded = append(encoded, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}

	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("write json %s: %w", path, err)
	}

	return nil
}

func cleanupTmpFiles(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) == ".tmp" {
			err := os.Remove(filepath.Join(root, name))
			if err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}

	return nil
}

func (r *Runner) emit(runID string, stage string, message string, chunkIndex int, completedChunks int, totalChunks int) {
	if r == nil || r.progress == nil {
		return
	}

	r.progress(ProgressEvent{
		Time:            time.Now().UTC(),
		RunID:           runID,
		Stage:           stage,
		Message:         message,
		ChunkIndex:      chunkIndex,
		CompletedChunks: completedChunks,
		TotalChunks:     totalChunks,
	})
}
