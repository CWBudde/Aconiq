package wasmkernel

import (
	"fmt"
	"time"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/rls19/road"
)

// How ComputeRLS19Road sizes its chunks when the caller states no preference.
//
// A fixed count cannot work here, and the first version of this file learned
// that the hard way: it reported every 256 receivers, which is a few
// milliseconds on an open-field grid and *twenty seconds* on a scene with
// twenty buildings in it, because a building is a barrier and a reflector at
// once and RLS-19 Nr. 3.5 gives every Spiegelschallquelle its own diffraction
// search. Browser mode therefore sat on "starting the run" for the whole of a
// real model's first chunk, which is the exact failure the progress channel
// exists to rule out.
//
// So the walk measures instead of assuming: it computes a small first chunk,
// times it, and sizes the next one to land near ProgressInterval. That holds
// the reporting cadence roughly constant however expensive one receiver turns
// out to be — which is not something this package can know in advance, since
// it depends on the building count, the Teilstück length and the reflection
// order of the model in front of it.
const (
	// InitialChunkSize is the first chunk, before there is anything to
	// measure. Small enough that even a very expensive receiver reports
	// within about a second, and the handful of extra boundaries it costs a
	// cheap scene are paid back within the first few chunks — growth is
	// geometric.
	InitialChunkSize = 8

	// MaxChunkSize caps the growth. An open-field grid computes a receiver in
	// tens of microseconds, so the target interval alone would ask for chunks
	// of hundreds of thousands and a run would report twice; this keeps a bar
	// moving on the cheap scenes too.
	MaxChunkSize = 4096

	// ProgressInterval is the cadence the adaptive size aims for. It matches
	// the throttle in `kernel.worker.ts`, so roughly every chunk the walk
	// reports is a message the main thread actually draws.
	ProgressInterval = 100 * time.Millisecond

	// chunkGrowthLimit bounds how fast the size may rise between two chunks.
	// One chunk that happened to be cheap — receivers behind the only
	// building in the scene, a warm cache — must not put the next one
	// minutes away. Shrinking is deliberately not limited: a chunk that
	// overran its budget has already cost the user the wait, and the walk
	// should not spend a second one confirming it.
	chunkGrowthLimit = 4
)

// ComputeRLS19Road computes receivers in deterministic chunks, reporting
// progress between them.
//
// It exists because `cmd/wasm/main.go` builds its answer inside a Promise
// executor, and a Promise executor runs synchronously: the whole RLS-19 walk
// happens before the caller's `await` ever yields. Nothing on the Go side can
// change that — a WebAssembly module has one stack and no way to hand control
// back mid-call — so the kernel runs in a Web Worker and the only thing it owes
// the main thread is a count of how far it has got. This is where that count
// comes from.
//
// progress is called with (receivers done, receivers in total) after each
// chunk, never before the first one completes and always exactly once with
// done == total on a successful walk. A nil progress is allowed and means the
// walk reports nothing; that is the one-argument `aconiq.rls19Road` path.
//
// It says nothing about how long the *first* chunk takes, and a caller drawing
// a bar should not wait for it to say something: `kernel-client.ts` reports
// 0 of n itself the moment it dispatches the call, so the reader sees a
// determinate bar across the scene preparation as well.
//
// **Chunking cannot change a result.** The scene is receiver-independent and
// each receiver is computed from it alone, so the chunk size decides only where
// the walk pauses to report, never what it computes or in what order it appends
// — which is what `docs/policies/determinism.md` requires of anything that
// looks like a partition. TestComputeRLS19RoadMatchesTheUnchunkedWalk pins that
// against road.ComputeReceiverOutputs at every chunk size from one receiver to
// all of them, by full struct equality rather than a tolerance, because "same
// inputs, same outputs" here means the bits.
//
// That is what makes the adaptive size below admissible at all. Where the
// chunks fall depends on how fast the machine turned out to be, so two runs of
// the same model report different *sequences* — but a progress report is not
// an output, and the outputs are identical bit for bit. The chunk size the
// caller passes is honoured exactly; chunkSize <= 0 asks for the adaptive one.
func ComputeRLS19Road(
	receivers []geo.PointReceiver,
	sources []road.RoadSource,
	barriers []road.Barrier,
	cfg road.PropagationConfig,
	chunkSize int,
	progress func(done, total int),
) ([]road.ReceiverOutput, error) {
	total := len(receivers)

	// Delegated rather than restated. The empty-receiver refusal is
	// ComputeReceiverOutputs' sentence to word, and a copy of it here would be
	// a second one to keep in step — which is the whole hazard this function
	// has to avoid, since browser mode and `aconiq run` must refuse the same
	// model in the same words.
	if total == 0 {
		_, err := road.ComputeReceiverOutputs(receivers, sources, barriers, cfg)

		// Bare %w, as in terrain.go: the whole point of delegating is that the
		// sentence crosses unchanged, and cmd/wasm already prefixes the entry
		// point. A second layer of context would name the same failure twice.
		return nil, fmt.Errorf("%w", err)
	}

	scene, err := prepareScene(receivers, sources, barriers, cfg)
	if err != nil {
		return nil, err
	}

	outputs := make([]road.ReceiverOutput, 0, total)

	// A caller that named a size gets it for every chunk; one that did not is
	// measured, starting small.
	adaptive := chunkSize <= 0
	if adaptive {
		chunkSize = InitialChunkSize
	}

	for done := 0; done < total; {
		end := min(done+chunkSize, total)

		startedAt := time.Now()

		chunk, err := scene.ComputeReceivers(receivers[done:end], cfg)
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}

		elapsed := time.Since(startedAt)

		outputs = append(outputs, chunk...)

		if adaptive {
			chunkSize = nextChunkSize(chunkSize, end-done, elapsed)
		}

		done = end

		if progress != nil {
			progress(done, total)
		}
	}

	return outputs, nil
}

// nextChunkSize is how many receivers to compute before the next report, given
// that computing computed of them took elapsed.
//
// Straight-line extrapolation: receivers within one scene cost roughly the
// same, because what makes them expensive — the Teilstück count, the buildings
// standing between them and the road — is a property of the scene rather than
// of the receiver. Where that is wrong it is wrong by a factor, and the growth
// limit and the floor keep a factor from becoming a stall.
func nextChunkSize(current, computed int, elapsed time.Duration) int {
	ceiling := min(current*chunkGrowthLimit, MaxChunkSize)

	// Below the clock's resolution. Not a measurement of "instant" but the
	// absence of one, so grow by the limit and measure again next time rather
	// than dividing by a zero.
	if elapsed <= 0 {
		return ceiling
	}

	wanted := float64(computed) * (float64(ProgressInterval) / float64(elapsed))

	// One receiver is the floor, however far over budget the chunk ran: the
	// walk has no smaller unit to pause at, and a scene where a single
	// receiver takes longer than the interval is one the reader most needs to
	// see moving.
	if wanted < 1 {
		return 1
	}

	if wanted > float64(ceiling) {
		return ceiling
	}

	return int(wanted)
}

// prepareScene derives the receiver-independent half once, keeping
// ComputeReceiverOutputs' order of refusals.
//
// That order is load-bearing and not obvious. ComputeReceiverOutputs prepares
// the scene *lazily*, on the first receiver that has cleared its own checks, so
// a model that is wrong in both ways at once — a receiver with no ID and a
// source with no ID — reports the receiver. Preparing eagerly, which is exactly
// what this function does to get the saving it exists for, would swap the two
// around.
//
// So the eager preparation is speculative: when it succeeds nothing was
// reordered, because a scene that prepares cleanly has no refusal to lose the
// race. When it fails, the refusal the caller hears is not ours to choose, and
// ComputeReceiverOutputs is asked to word it. That costs a second
// PrepareScene, on a path that is returning an error and computing nothing.
func prepareScene(
	receivers []geo.PointReceiver,
	sources []road.RoadSource,
	barriers []road.Barrier,
	cfg road.PropagationConfig,
) (*road.Scene, error) {
	// The first receiver's height, as ComputeReceiverOutputs would have used
	// it: ReceiverHeightM is the one config field a receiver overrides, and
	// PrepareScene reads it only to validate it.
	firstCfg := cfg
	firstCfg.ReceiverHeightM = receivers[0].HeightM

	scene, err := road.PrepareScene(sources, barriers, firstCfg)
	if err == nil {
		return scene, nil
	}

	_, refusal := road.ComputeReceiverOutputs(receivers, sources, barriers, cfg)
	if refusal != nil {
		return nil, fmt.Errorf("%w", refusal)
	}

	// Unreachable as the two are written today — ComputeReceiverOutputs
	// prepares from the same first receiver, so it cannot succeed where
	// PrepareScene failed. Returning the refusal we already have rather than a
	// nil scene keeps that an error instead of a panic if it ever stops being
	// true.
	return nil, fmt.Errorf("%w", err)
}
