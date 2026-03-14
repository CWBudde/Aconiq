package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/geo"
)

func TestDeterministicHashAcrossWorkerCounts(t *testing.T) {
	t.Parallel()

	receivers := make([]Receiver, 0, 180)
	for i := range 180 {
		receivers = append(receivers, Receiver{
			ID:      "rx-" + padInt(i),
			Point:   geo.Point2D{X: float64(i % 30), Y: float64(i / 30)},
			HeightM: 4,
		})
	}

	sources := []Source{
		{ID: "s1", Point: geo.Point2D{X: 2, Y: 3}, Emission: 88.2},
		{ID: "s2", Point: geo.Point2D{X: 15, Y: 9}, Emission: 90.1},
		{ID: "s3", Point: geo.Point2D{X: 25, Y: 1}, Emission: 85.7},
	}

	cacheDir := t.TempDir()
	runner := NewRunner(nil)

	out1, err := runner.Run(context.Background(), RunConfig{
		RunID:          "det-w1",
		Workers:        1,
		ChunkSize:      17,
		CacheDir:       cacheDir,
		Receivers:      receivers,
		Sources:        sources,
		DisableCache:   true,
		DeterminismTag: "w1",
	})
	if err != nil {
		t.Fatalf("run with 1 worker: %v", err)
	}

	outN, err := runner.Run(context.Background(), RunConfig{
		RunID:          "det-w4",
		Workers:        4,
		ChunkSize:      17,
		CacheDir:       cacheDir,
		Receivers:      receivers,
		Sources:        sources,
		DisableCache:   true,
		DeterminismTag: "w4",
	})
	if err != nil {
		t.Fatalf("run with 4 workers: %v", err)
	}

	if out1.OutputHash != outN.OutputHash {
		t.Fatalf("expected identical output hash, got %s vs %s", out1.OutputHash, outN.OutputHash)
	}

	if len(out1.Results) != len(outN.Results) {
		t.Fatalf("expected same result length")
	}
}

func TestCancellationLeavesConsistentState(t *testing.T) {
	t.Parallel()

	receivers := make([]Receiver, 0, 600)
	for i := range 600 {
		receivers = append(receivers, Receiver{
			ID:      "rx-" + padInt(i),
			Point:   geo.Point2D{X: float64(i % 50), Y: float64(i / 50)},
			HeightM: 4,
		})
	}

	sources := []Source{{ID: "s1", Point: geo.Point2D{X: 0, Y: 0}, Emission: 90}}

	cacheDir := t.TempDir()
	runner := NewRunner(nil)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(40 * time.Millisecond)
		cancel()
	}()

	_, err := runner.Run(ctx, RunConfig{
		RunID:        "cancel-test",
		Workers:      4,
		ChunkSize:    20,
		CacheDir:     cacheDir,
		Receivers:    receivers,
		Sources:      sources,
		DisableCache: false,
		ComputeDelay: 2 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected cancellation error")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	statePath := filepath.Join(cacheDir, "cancel-test", "run-state.json")

	payload, readErr := os.ReadFile(statePath)
	if readErr != nil {
		t.Fatalf("read run state: %v", readErr)
	}

	var state RunState

	decodeErr := json.Unmarshal(payload, &state)
	if decodeErr != nil {
		t.Fatalf("decode run state: %v", decodeErr)
	}

	if state.Status != RunStateCanceled {
		t.Fatalf("expected canceled state, got %s", state.Status)
	}

	chunksDir := filepath.Join(cacheDir, "cancel-test", "chunks")

	entries, readDirErr := os.ReadDir(chunksDir)
	if readDirErr != nil {
		t.Fatalf("read chunks dir: %v", readDirErr)
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("unexpected tmp file left behind: %s", entry.Name())
		}
	}
}

func TestChunkCacheReuse(t *testing.T) {
	t.Parallel()

	receivers := make([]Receiver, 0, 90)
	for i := range 90 {
		receivers = append(receivers, Receiver{
			ID:      "rx-" + padInt(i),
			Point:   geo.Point2D{X: float64(i), Y: float64(i % 10)},
			HeightM: 4,
		})
	}

	sources := []Source{{ID: "s1", Point: geo.Point2D{X: 10, Y: 10}, Emission: 91.2}}
	cacheDir := t.TempDir()
	runner := NewRunner(nil)

	first, err := runner.Run(context.Background(), RunConfig{
		RunID:        "cache-run",
		Workers:      3,
		ChunkSize:    15,
		CacheDir:     cacheDir,
		Receivers:    receivers,
		Sources:      sources,
		DisableCache: false,
	})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}

	second, err := runner.Run(context.Background(), RunConfig{
		RunID:        "cache-run-repeat",
		Workers:      4,
		ChunkSize:    15,
		CacheDir:     cacheDir,
		Receivers:    receivers,
		Sources:      sources,
		DisableCache: false,
	})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	if first.OutputHash != second.OutputHash {
		t.Fatalf("expected equal output hashes for cache reuse")
	}

	if second.UsedCachedChunks == 0 {
		t.Fatalf("expected cached chunks to be reused")
	}
}

func TestChunkCacheInvalidatesWhenSourcesChange(t *testing.T) {
	t.Parallel()

	receivers := make([]Receiver, 0, 40)
	for i := range 40 {
		receivers = append(receivers, Receiver{
			ID:      "rx-" + padInt(i),
			Point:   geo.Point2D{X: float64(i), Y: float64(i % 8)},
			HeightM: 4,
		})
	}

	cacheDir := t.TempDir()
	runner := NewRunner(nil)

	first, err := runner.Run(context.Background(), RunConfig{
		RunID:     "invalidate-a",
		Workers:   2,
		ChunkSize: 10,
		CacheDir:  cacheDir,
		Receivers: receivers,
		Sources:   []Source{{ID: "s1", Point: geo.Point2D{X: 10, Y: 10}, Emission: 91.2}},
	})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}

	second, err := runner.Run(context.Background(), RunConfig{
		RunID:     "invalidate-b",
		Workers:   2,
		ChunkSize: 10,
		CacheDir:  cacheDir,
		Receivers: receivers,
		Sources:   []Source{{ID: "s1", Point: geo.Point2D{X: 10, Y: 10}, Emission: 95.2}},
	})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	if second.UsedCachedChunks != 0 {
		t.Fatalf("expected changed sources to invalidate chunk cache, got %d cached chunks", second.UsedCachedChunks)
	}

	if first.OutputHash == second.OutputHash {
		t.Fatalf("expected changed sources to produce a different output hash")
	}
}

func TestRunCacheRetentionPrunesOlderRunDirectories(t *testing.T) {
	t.Parallel()

	receivers := []Receiver{
		{ID: "rx-0001", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4},
		{ID: "rx-0002", Point: geo.Point2D{X: 10, Y: 0}, HeightM: 4},
	}
	sources := []Source{{ID: "s1", Point: geo.Point2D{X: 5, Y: 5}, Emission: 90}}

	cacheDir := t.TempDir()
	runner := NewRunner(nil)

	err := os.MkdirAll(filepath.Join(cacheDir, "bench", "keep"), 0o755)
	if err != nil {
		t.Fatalf("create bench cache dir: %v", err)
	}

	for _, runID := range []string{"run-001", "run-002", "run-003"} {
		_, err := runner.Run(context.Background(), RunConfig{
			RunID:            runID,
			Workers:          1,
			ChunkSize:        1,
			CacheDir:         cacheDir,
			RunCacheKeepLast: 2,
			Receivers:        receivers,
			Sources:          sources,
			DisableCache:     false,
		})
		if err != nil {
			t.Fatalf("run %s: %v", runID, err)
		}
	}

	_, err = os.Stat(filepath.Join(cacheDir, "run-001"))
	if !os.IsNotExist(err) {
		t.Fatalf("expected oldest run cache to be pruned, got err=%v", err)
	}

	for _, runID := range []string{"run-002", "run-003"} {
		_, err = os.Stat(filepath.Join(cacheDir, runID))
		if err != nil {
			t.Fatalf("expected run cache %s to remain: %v", runID, err)
		}
	}

	_, err = os.Stat(filepath.Join(cacheDir, "shared-chunks"))
	if err != nil {
		t.Fatalf("expected shared chunk cache to remain: %v", err)
	}

	_, err = os.Stat(filepath.Join(cacheDir, "bench"))
	if err != nil {
		t.Fatalf("expected bench cache to remain: %v", err)
	}
}

func TestProgressEventsIncludePipelineStages(t *testing.T) {
	t.Parallel()

	receivers := []Receiver{{ID: "r1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4}}
	sources := []Source{{ID: "s1", Point: geo.Point2D{X: 1, Y: 1}, Emission: 88}}

	var mu sync.Mutex
	events := make([]ProgressEvent, 0)
	runner := NewRunner(func(event ProgressEvent) {
		mu.Lock()
		defer mu.Unlock()

		events = append(events, event)
	})

	_, err := runner.Run(context.Background(), RunConfig{
		RunID:        "events",
		Workers:      1,
		ChunkSize:    1,
		CacheDir:     t.TempDir(),
		Receivers:    receivers,
		Sources:      sources,
		DisableCache: true,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	hasStage := func(stage string) bool {
		for _, event := range events {
			if event.Stage == stage {
				return true
			}
		}

		return false
	}

	for _, stage := range []string{"load", "prepare", "chunk", "compute", "reduce", "persist"} {
		if !hasStage(stage) {
			t.Fatalf("expected stage event for %s", stage)
		}
	}
}

func padInt(v int) string {
	return fmt.Sprintf("%04d", v)
}
