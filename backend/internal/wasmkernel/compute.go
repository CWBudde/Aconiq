package wasmkernel

import (
	"fmt"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/rls19/road"
)

// DefaultChunkSize is how many receivers ComputeRLS19Road computes between two
// progress reports when the caller states no preference.
//
// It is a compromise between two costs that pull in opposite directions. Too
// small and the walk pays a JavaScript call per handful of receivers — under
// js/wasm every `cb.Invoke` crosses the wasm boundary and resumes the Go
// scheduler, which is not free. Too large and a long run reports nothing for
// seconds at a time, which is the defect the progress channel exists to fix.
// 256 receivers is a few milliseconds of propagation work on the grids browser
// mode actually runs, so a bar moves smoothly without the reporting showing up
// in a profile.
//
// It is deliberately not a limit on anything numeric: see ComputeRLS19Road on
// why the chunk size cannot move a single decibel.
const DefaultChunkSize = 256

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
// **Chunking cannot change a result.** The scene is receiver-independent and
// each receiver is computed from it alone, so the chunk size decides only where
// the walk pauses to report, never what it computes or in what order it appends
// — which is what `docs/policies/determinism.md` requires of anything that
// looks like a partition. TestComputeRLS19RoadMatchesTheUnchunkedWalk pins that
// against road.ComputeReceiverOutputs at every chunk size from one receiver to
// all of them, by full struct equality rather than a tolerance, because "same
// inputs, same outputs" here means the bits.
//
// chunkSize <= 0 means DefaultChunkSize.
func ComputeRLS19Road(
	receivers []geo.PointReceiver,
	sources []road.RoadSource,
	barriers []road.Barrier,
	cfg road.PropagationConfig,
	chunkSize int,
	progress func(done, total int),
) ([]road.ReceiverOutput, error) {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}

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

	for done := 0; done < total; {
		end := min(done+chunkSize, total)

		chunk, err := scene.ComputeReceivers(receivers[done:end], cfg)
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}

		outputs = append(outputs, chunk...)
		done = end

		if progress != nil {
			progress(done, total)
		}
	}

	return outputs, nil
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
