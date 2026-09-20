package road

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/partition"
)

// DefaultWorkers is how many goroutines a run uses when the caller does not say.
//
// GOMAXPROCS rather than NumCPU, so a container's CPU limit and a GOMAXPROCS
// set by hand are both honoured. Under js/wasm it is 1, which is how the
// browser build never enters the pool: the work would move behind a scheduler
// without ever running on a second thread, and the only real parallelism a
// browser has is more Workers, each with its own instance.
func DefaultWorkers() int {
	return runtime.GOMAXPROCS(0)
}

// ComputeReceiverOutputsParallel computes exactly what ComputeReceiverOutputs
// computes, over the given number of goroutines.
//
// Same outputs in the same order, same refusal in the same words; only the
// wall clock differs.
//
// # Why this is bit-identical rather than merely close
//
// There is no reduction across receivers anywhere in RLS-19. A receiver's
// level is acoustics.EnergySum over that receiver's own contributions, in the
// scene's source-and-Teilstück order, and no split touches that order. So this
// changes no floating-point operation and no floating-point operand — only
// which goroutine executes an unchanged sequence. That is a stronger claim
// than the usual "the partition is deterministic", and it is the reason
// docs/policies/determinism.md's "stable reduction tree" reduces here to a
// concatenation.
//
// The Scene is shared read-only. It is written once, in prepareScene, and its
// two spatial indexes are immutable after their fill; every mutable cursor
// lives on a pathScratch, and Scene.ComputeReceivers makes one per call. That
// division is not new — pathScratch's doc comment already states that a Scene
// "can be shared between goroutines; this cannot" — this is the first caller
// to rely on it.
//
// # Refusals
//
// Every refusal is decided before a chunk exists, because a pool cannot be
// trusted to order them. PrepareSceneFor reproduces the lazily-prepared
// walk's wording for a bad scene, and ValidateReceivers then makes every
// receiver-shaped refusal in input order.
//
// That up-front pass is load-bearing and not belt-and-braces. Relying on "the
// lowest-indexed failing chunk wins" is not enough on its own: a high-index
// chunk can fail and cancel the pool before a lower-index chunk has been
// dispatched, and the skipped chunk then records nothing, so the refusal a
// user reads would depend on scheduling. What is left reachable inside the
// walk is receiver-independent — appendParkingContributions refuses a parking
// source, which every chunk would refuse identically — so which chunk reports
// it no longer changes the sentence.
//
// workers <= 1 delegates and allocates no goroutine, no channel and no chunk
// list. That is the js/wasm path, and it is a runtime branch rather than a
// build tag so that the pool is compiled — and therefore type-checked and
// vetted — on every target rather than only where it runs.
func ComputeReceiverOutputsParallel(
	ctx context.Context,
	receivers []geo.PointReceiver,
	sources []RoadSource,
	barriers []Barrier,
	cfg PropagationConfig,
	workers int,
) ([]ReceiverOutput, error) {
	if workers <= 1 || len(receivers) == 0 {
		return ComputeReceiverOutputs(receivers, sources, barriers, cfg)
	}

	chunks := partition.Over(len(receivers), partition.Size(len(receivers), workers))
	if len(chunks) <= 1 {
		return ComputeReceiverOutputs(receivers, sources, barriers, cfg)
	}

	// Order matters between these two, and it is the sequential walk's order.
	// computeReceivers checks receiver 0, then prepares the scene, then walks
	// — so a scene that cannot be prepared outranks a bad receiver further
	// down the list, and PrepareSceneFor is what reproduces that wording.
	// Only once the scene is good are the remaining refusals purely
	// receiver-shaped, and only then can ValidateReceivers report them in the
	// order the sequential walk would have.
	scene, err := PrepareSceneFor(receivers, sources, barriers, cfg)
	if err != nil {
		return nil, err
	}

	if err := ValidateReceivers(receivers, cfg); err != nil {
		return nil, err
	}

	return scene.computeChunks(ctx, receivers, cfg, chunks, workers)
}

// chunkResult is one chunk's answer, kept beside its error so the collector
// can pick the lowest-indexed failure rather than the first one to arrive.
//
// done separates "this chunk produced no receivers" from "nobody ever ran
// this chunk". Without it a cancelled run assembles the chunks that did finish
// into a short, error-free result — a smaller raster that looks like a
// successful run of a smaller model, which is the worst way for this to fail.
type chunkResult struct {
	outputs []ReceiverOutput
	err     error
	done    bool
}

// computeChunks runs the chunks over a pool and reassembles them in index
// order.
//
// Workers pull from a channel rather than taking a fixed block each. A
// receiver grid is not uniformly expensive — a receiver beside the road walks
// more Teilstücke, crosses more barriers and admits more Spiegelschallquellen
// than one at the far corner — so a static split hands whoever drew the near
// rows several times the work and the run waits for it. Pulling cannot affect
// the output, because the merge is by chunk index and not by arrival.
func (s *Scene) computeChunks(
	ctx context.Context,
	receivers []geo.PointReceiver,
	cfg PropagationConfig,
	chunks []partition.Chunk,
	workers int,
) ([]ReceiverOutput, error) {
	results := make([]chunkResult, len(chunks))
	jobs := make(chan partition.Chunk)

	// Derived so the first refusal stops the others: the run is over, and the
	// remaining chunks would each spend a full walk discovering it.
	computeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	for range min(workers, len(chunks)) {
		wg.Go(func() {
			for chunk := range jobs {
				if computeCtx.Err() != nil {
					continue
				}

				outputs, err := s.ComputeReceivers(receivers[chunk.Start:chunk.End], cfg)
				results[chunk.Index] = chunkResult{outputs: outputs, err: err, done: true}

				if err != nil {
					cancel()
				}
			}
		})
	}

	for _, chunk := range chunks {
		select {
		case jobs <- chunk:
		case <-computeCtx.Done():
		}
	}

	close(jobs)
	wg.Wait()

	return collectChunks(results, len(receivers), ctx.Err())
}

// collectChunks concatenates the chunks in index order, or returns the
// lowest-indexed failure.
//
// Index order, not arrival order: that is the whole determinism argument, and
// it is what docs/policies/determinism.md means by refusing a "first finished
// worker wins" accumulation. Because chunks partition the receiver list in
// input order and a chunk is the sequential walk over its own slice, the
// lowest-indexed failure is the failure the sequential walk would have
// reported.
//
// cancelled is the *caller's* context error. It is checked first, because a
// caller who gave up wants to hear that and not whichever chunk happened to
// notice the cancellation on its way past.
func collectChunks(results []chunkResult, total int, cancelled error) ([]ReceiverOutput, error) {
	if cancelled != nil {
		return nil, fmt.Errorf("%w", cancelled)
	}

	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
	}

	outputs := make([]ReceiverOutput, 0, total)

	for i, r := range results {
		// A chunk nobody ran, with no error anywhere and no caller
		// cancellation, means the pool lost work. Refusing is the only safe
		// answer: the alternative is a short receiver table that reads as a
		// complete run of a smaller model.
		if !r.done {
			return nil, fmt.Errorf("compute chunk %d was never run", i)
		}

		outputs = append(outputs, r.outputs...)
	}

	if len(outputs) != total {
		return nil, fmt.Errorf("computed %d receivers, expected %d", len(outputs), total)
	}

	return outputs, nil
}
