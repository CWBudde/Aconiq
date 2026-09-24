//go:build js && wasm

// Package main is the WebAssembly entry point for the Aconiq computation kernel.
// It exposes the noise calculation functions to JavaScript via window.aconiq.
//
// Usage from JavaScript:
//
//	const result = await window.aconiq.rls19Road(JSON.stringify({
//	  receivers: [{ id: "R1", point: { x: 0, y: 100 }, height_m: 4 }],
//	  sources:   [...],
//	  barriers:  [...],
//	  config:    { SegmentLengthM: 10, MinDistanceM: 1, ReceiverHeightM: 4 }
//	}), (done, total) => console.log(done, "of", total));
//	const outputs = JSON.parse(result); // []ReceiverOutput
//
// The progress callback is optional; see rls19RoadFunc.
//
// Coordinates must be in a metric CRS before they reach rls19Road — see
// aconiq.transform, and internal/wasmkernel for why.
package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
	"time"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/wasmkernel"
)

type computeRequest struct {
	Receivers []geo.PointReceiver    `json:"receivers"`
	Sources   []road.RoadSource      `json:"sources"`
	Barriers  []road.Barrier         `json:"barriers"`
	Config    road.PropagationConfig `json:"config"`

	// Projection is the CRS pair the caller resolved before it built this
	// scene. It is optional — a request that loads no terrain needs nothing
	// from it — and required the moment a terrain model is loaded, because the
	// coordinates in this request name no CRS of their own and a DTM has to be
	// queried in the one it was written in. See wasmkernel.ComputeProjection.
	Projection *wasmkernel.ComputeProjection `json:"projection,omitempty"`

	// Shard names which member of a Worker pool is asking, when one is.
	// Absent means the whole run, which is what `rls19Road` always is —
	// only `rls19RoadShard` reads it.
	//
	// The rest of this struct is the *whole run's* even when a shard is set:
	// Receivers is every receiver, not the shard's slice. See
	// wasmkernel.ComputeRLS19RoadShard for why that is load-bearing rather
	// than wasteful.
	Shard *wasmkernel.Shard `json:"shard,omitempty"`
}

// prepareComputeRequest parses a request and resolves the ground under it.
//
// Shared by rls19Road and rls19RoadShard so the two cannot come to disagree
// about what a request means — which matters more than usual here, because
// the whole point of the shard export is that N Workers reach the identical
// configuration.
func prepareComputeRequest(input string) (computeRequest, error) {
	var req computeRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return computeRequest{}, fmt.Errorf("invalid input JSON: %w", err)
	}

	req.Config = withPropagationDefaults(req.Config)

	// The terrain is the ground h_m is measured above as well as the one
	// elevation the receiver heights stack on, so both travel into the
	// config — and both in the CRS this request computes in. Browser mode
	// resolves the ground the way `aconiq run` does or it is answering a
	// different question.
	cfg, err := wasmkernel.ApplyTerrain(req.Config, req.Receivers, req.Projection)
	if err != nil {
		return computeRequest{}, fmt.Errorf("%w", err)
	}

	req.Config = cfg

	return req, nil
}

// rls19RoadFunc computes RLS-19 road traffic noise levels.
// Signature: (request: string, onProgress?: (done, total) => void)
// => Promise<string> (JSON).
//
// The second argument is how a run reports how far it has got. It is optional
// because the callers that do not draw a bar should not pay for one, and
// because the export took exactly one argument until the kernel moved off the
// main thread: `kernel.worker.ts` still makes a one-argument call unless its
// caller asked for progress.
//
// The computation and every progress report it makes run on the worker's only
// thread, synchronously, inside the Promise executor — an executor runs
// synchronously, which is the whole reason the kernel needed a thread of its
// own. There is therefore nothing to gain from a goroutine: GOMAXPROCS is 1
// under js/wasm, so one would move the same work behind a scheduler and hand
// the caller a longer stack to read. The worker's own handler is a throttled
// postMessage, which queues a message rather than waiting on an event loop, so
// reporting synchronously into it does not stall the walk.
func rls19RoadFunc(_ js.Value, args []js.Value) any {
	if len(args) != 1 && len(args) != 2 {
		return jsReject("rls19Road: expected 1 or 2 arguments (string request JSON, optional progress callback)")
	}

	onProgress := js.Undefined()

	if len(args) == 2 {
		if args[1].Type() != js.TypeFunction {
			return jsReject("rls19Road: expected the second argument to be a progress callback function")
		}

		onProgress = args[1]
	}

	input := args[0].String()

	return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, promArgs []js.Value) any {
		resolve, reject := promArgs[0], promArgs[1]

		req, err := prepareComputeRequest(input)
		if err != nil {
			reject.Invoke(js.ValueOf(fmt.Sprintf("rls19Road: %v", err)))

			return nil
		}

		// Chunked so the walk can report between chunks. The chunk size is
		// wasmkernel's default: it decides where the walk pauses and nothing
		// else, because the scene is receiver-independent and every receiver
		// is computed from it alone.
		outputs, err := wasmkernel.ComputeRLS19Road(
			req.Receivers, req.Sources, req.Barriers, req.Config,
			0, progressReporter(onProgress),
		)
		if err != nil {
			reject.Invoke(js.ValueOf(fmt.Sprintf("rls19Road: computation error: %v", err)))
			return nil
		}

		out, err := json.Marshal(outputs)
		if err != nil {
			reject.Invoke(js.ValueOf(fmt.Sprintf("rls19Road: marshal error: %v", err)))
			return nil
		}

		resolve.Invoke(js.ValueOf(string(out)))
		return nil
	}))
}

// rls19RoadShardFunc computes one Worker's share of an RLS-19 road run.
//
// Same request shape as rls19Road, plus a `shard` of {index, count}, and it
// resolves to a JSON array of {chunk, start, outputs} rather than a flat
// receiver list. The client sorts those by `chunk` and concatenates: the
// order four Workers reply in is the order the machine happened to schedule
// them, and docs/policies/determinism.md requires the merge to be
// independent of that.
//
// A separate export rather than a field on rls19Road, although the request
// already carries the shard: the two resolve to different shapes, and an
// export whose return type depends on whether a field was set is a contract
// nobody can read off the call site.
//
// progress reports this shard's own receivers. Summing the shards is the
// client's job, because only it knows the run's total.
func rls19RoadShardFunc(_ js.Value, args []js.Value) any {
	if len(args) != 1 && len(args) != 2 {
		return jsReject("rls19RoadShard: expected 1 or 2 arguments (string request JSON, optional progress callback)")
	}

	onProgress := js.Undefined()

	if len(args) == 2 {
		if args[1].Type() != js.TypeFunction {
			return jsReject("rls19RoadShard: expected the second argument to be a progress callback function")
		}

		onProgress = args[1]
	}

	input := args[0].String()

	return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, promArgs []js.Value) any {
		resolve, reject := promArgs[0], promArgs[1]

		req, err := prepareComputeRequest(input)
		if err != nil {
			reject.Invoke(js.ValueOf(fmt.Sprintf("rls19RoadShard: %v", err)))

			return nil
		}

		if req.Shard == nil {
			reject.Invoke(js.ValueOf("rls19RoadShard: the request carries no shard; use rls19Road for a whole run"))

			return nil
		}

		chunks, err := wasmkernel.ComputeRLS19RoadShard(
			req.Receivers, req.Sources, req.Barriers, req.Config,
			*req.Shard, 0, progressReporter(onProgress),
		)
		if err != nil {
			reject.Invoke(js.ValueOf(fmt.Sprintf("rls19RoadShard: computation error: %v", err)))

			return nil
		}

		out, err := json.Marshal(chunks)
		if err != nil {
			reject.Invoke(js.ValueOf(fmt.Sprintf("rls19RoadShard: marshal error: %v", err)))

			return nil
		}

		resolve.Invoke(js.ValueOf(string(out)))

		return nil
	}))
}

// progressReporter turns the caller's JavaScript callback into the function
// wasmkernel.ComputeRLS19Road reports through, or nil when there was none.
//
// nil rather than a callback that does nothing: the compute loop checks for it
// once per chunk, and a one-argument call should cost nothing at all.
func progressReporter(callback js.Value) func(done, total int) {
	if callback.Type() != js.TypeFunction {
		return nil
	}

	// A listener that threw once will throw again on every chunk for the rest
	// of the run, so it is called until it does and then not again.
	live := true

	return func(done, total int) {
		if !live {
			return
		}

		live = invokeQuietly(callback, done, total)
	}
}

// invokeQuietly calls a JavaScript function and contains an exception, saying
// whether it got through.
//
// Containing it is not defensiveness for its own sake. js.Value.Invoke turns a
// JavaScript exception into a Go panic, and a panic out of a js.Func is caught
// by nothing: wasm_exec.js resumes the Go scheduler and reads `event.result`
// afterwards, with no throw hook in between, so the panic ends the program and
// every later call into the kernel answers "Go program has already exited".
// Progress is cosmetic, and a progress bar must not be able to kill the
// computation it is drawing.
func invokeQuietly(callback js.Value, args ...any) (delivered bool) {
	defer func() {
		if recover() != nil {
			delivered = false
		}
	}()

	callback.Invoke(args...)

	return true
}

// transformFunc projects a batch of coordinates between two CRS.
// Takes a single JSON string argument, returns a Promise<string> (JSON),
// mirroring rls19Road so a caller has one calling convention to learn.
//
// This is what keeps browser mode out of degrees: `internal/geo` is already
// linked into this binary, so exposing the transform costs a registration line
// rather than a second transverse-Mercator implementation in TypeScript — and
// the browser therefore projects through the same series the CLI does, by
// construction.
func transformFunc(_ js.Value, args []js.Value) any {
	if len(args) != 1 {
		return jsReject("transform: expected exactly 1 JSON string argument")
	}

	input := args[0].String()

	return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, promArgs []js.Value) any {
		resolve, reject := promArgs[0], promArgs[1]

		out, err := wasmkernel.Transform([]byte(input))
		if err != nil {
			// Verbatim: geo.ComputeCRSForGeographic's refusal names the zone and
			// says what to do about it, and browser mode must refuse a site in
			// the same words `aconiq run` does.
			reject.Invoke(js.ValueOf(err.Error()))

			return nil
		}

		resolve.Invoke(js.ValueOf(string(out)))

		return nil
	}))
}

// maskFootprintsFunc answers which receivers stand inside a building.
// Signature: (json: string) => Promise<string> (JSON)
//
// A Promise like transform, for the same reason: a city-sized grid against a
// few thousand footprints is not work to hand back on the caller's stack.
func maskFootprintsFunc(_ js.Value, args []js.Value) any {
	if len(args) != 1 {
		return jsReject("maskFootprints: expected exactly 1 JSON string argument")
	}

	input := args[0].String()

	return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, promArgs []js.Value) any {
		resolve, reject := promArgs[0], promArgs[1]

		out, err := wasmkernel.MaskFootprints([]byte(input))
		if err != nil {
			reject.Invoke(js.ValueOf(err.Error()))

			return nil
		}

		resolve.Invoke(js.ValueOf(string(out)))

		return nil
	}))
}

// contoursFunc traces ISO-band contour lines over a run's raster.
// Signature: (payload: Uint8Array, request: string) => Promise<string> (JSON)
//
// The raster values arrive as bytes rather than inside the JSON because a
// 500x500 two-band grid is four megabytes of float64: rendering those as JSON
// numbers and parsing them back would cost more than the tracing does. The
// sidecar metadata that says how to read them travels in the request, which is
// why this entry point takes two arguments where transform takes one.
//
// It answers with a Promise, like rls19Road and transform and unlike
// standards: marching squares over that grid is not work to do inside a
// synchronous call. jsReject is therefore the right refusal here and the wrong
// one for a synchronous export, which has an error channel of its own — see
// syncThrowSource.
func contoursFunc(_ js.Value, args []js.Value) any {
	if len(args) != 2 {
		return jsReject("contours: expected 2 arguments (Uint8Array payload, string request JSON)")
	}

	jsArr := args[0]
	length := jsArr.Get("byteLength").Int()
	payload := make([]byte, length)
	js.CopyBytesToGo(payload, jsArr)

	input := args[1].String()

	return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, promArgs []js.Value) any {
		resolve, reject := promArgs[0], promArgs[1]

		out, err := wasmkernel.Contours(payload, []byte(input))
		if err != nil {
			// Verbatim, as in transformFunc: contour.FromRaster's refusals are
			// the ones `aconiq export` and the API print, and a reader
			// comparing browser mode against a bundle has to read the same
			// sentence in both.
			reject.Invoke(js.ValueOf(err.Error()))

			return nil
		}

		resolve.Invoke(js.ValueOf(string(out)))

		return nil
	}))
}

// standardsFunc returns the standards this kernel can run, in the same JSON
// shape `GET /api/v1/standards` answers with.
// Signature: () => string (JSON), throws on failure
//
// Unlike defaultConfig/health/projectStatus below, the marshal error is not
// swallowed: an empty standards list renders the run page unusable, and a
// silent one would look like a kernel that supports nothing. It reaches the
// caller as a thrown Error, which is what `kernel.worker.ts` catches around
// the handshake — see syncThrowSource for how a Go function manages to throw.
func standardsFunc(_ js.Value, _ []js.Value) any {
	out, err := wasmkernel.StandardsJSON()
	if err != nil {
		return jsError(fmt.Sprintf("standards: marshal error: %v", err))
	}

	return js.ValueOf(string(out))
}

// loadTerrainFunc loads a GeoTIFF terrain model from a Uint8Array.
// Signature: (data: Uint8Array, crs: string) => string (terrain.Info JSON),
// throws on failure — see syncThrowSource.
//
// The CRS is a second argument rather than something the kernel works out: the
// GeoTIFF loader reads the tie point and the pixel scale and no
// GeoKeyDirectory, so the file does not say what it is in, and the request that
// later queries it carries coordinates that name no CRS either. The caller is
// the only one who knows, so the caller states it.
func loadTerrainFunc(_ js.Value, args []js.Value) any {
	if len(args) != 2 {
		return jsError("loadTerrain: expected 2 arguments (Uint8Array data, string crs)")
	}

	jsArr := args[0]
	length := jsArr.Get("byteLength").Int()
	buf := make([]byte, length)
	js.CopyBytesToGo(buf, jsArr)

	info, err := wasmkernel.LoadTerrain(buf, args[1].String())
	if err != nil {
		return jsError(fmt.Sprintf("loadTerrain: %v", err))
	}

	return js.ValueOf(string(info))
}

// clearTerrainFunc removes the currently loaded terrain model.
func clearTerrainFunc(_ js.Value, _ []js.Value) any {
	wasmkernel.ClearTerrain()

	return js.Undefined()
}

// withPropagationDefaults fills the three scalars a caller may leave unset,
// one at a time.
//
// Replacing the whole struct would discard everything else the config carries
// — Buildings, Reflectors, ParkingSources, Terrain — for a caller that simply
// did not state a segment length. The browser sets all three today, so that
// was latent rather than live, but the scene fields are exactly what a request
// building a Parkplatz sends.
func withPropagationDefaults(cfg road.PropagationConfig) road.PropagationConfig {
	defaults := road.DefaultPropagationConfig()

	if cfg.SegmentLengthM == 0 {
		cfg.SegmentLengthM = defaults.SegmentLengthM
	}

	if cfg.MinDistanceM == 0 {
		cfg.MinDistanceM = defaults.MinDistanceM
	}

	if cfg.ReceiverHeightM == 0 {
		cfg.ReceiverHeightM = defaults.ReceiverHeightM
	}

	return cfg
}

// defaultConfigFunc returns the default PropagationConfig as a JSON string.
func defaultConfigFunc(_ js.Value, _ []js.Value) any {
	cfg := road.DefaultPropagationConfig()
	out, _ := json.Marshal(cfg)
	return js.ValueOf(string(out))
}

// healthFunc returns a static health response for the WASM demo environment.
// Signature: () => string (JSON)
func healthFunc(_ js.Value, _ []js.Value) any {
	type healthResp struct {
		Status  string `json:"status"`
		Version string `json:"version"`
		Time    string `json:"time"`
	}
	resp := healthResp{
		Status:  "ok",
		Version: "wasm",
		Time:    time.Now().UTC().Format(time.RFC3339),
	}
	out, _ := json.Marshal(resp)
	return js.ValueOf(string(out))
}

// projectStatusFunc returns a stub project status for the WASM demo environment.
// Signature: () => string (JSON)
func projectStatusFunc(_ js.Value, _ []js.Value) any {
	type projectResp struct {
		ProjectID       string `json:"project_id"`
		Name            string `json:"name"`
		ProjectPath     string `json:"project_path"`
		ManifestVersion int    `json:"manifest_version"`
		CRS             string `json:"crs"`
		ScenarioCount   int    `json:"scenario_count"`
		RunCount        int    `json:"run_count"`
	}
	resp := projectResp{
		Name:            "(browser demo)",
		ManifestVersion: 1,
		CRS:             "—",
	}
	out, _ := json.Marshal(resp)
	return js.ValueOf(string(out))
}

func jsReject(msg string) js.Value {
	return js.Global().Get("Promise").Call("reject", js.ValueOf(msg))
}

// jsError is what a *synchronous* export returns in place of its JSON when it
// fails. throwingSyncExport turns it into a throw.
//
// An Error rather than a bare string, unlike jsReject: the string is what the
// asynchronous exports have always rejected with and `kernel-error.ts`
// normalises either, but a value that has to survive a `result instanceof
// Error` test on the way out has to be one.
func jsError(msg string) js.Value {
	return js.Global().Get("Error").New(msg)
}

// syncThrowSource is the JavaScript half of the synchronous exports' error
// channel: given a Go function, it returns one that rethrows what the Go
// function returned, if that turns out to be an Error.
//
// It is in JavaScript because a Go function cannot throw. syscall/js turns a
// JavaScript exception into a Go panic, and a panic out of a js.Func is caught
// by nothing — wasm_exec.js resumes the Go scheduler and then reads
// `event.result`, with no throw hook in between — so the panic ends the
// program, the call evaluates to `undefined`, and every later call into the
// kernel answers "Go program has already exited". One JavaScript frame above
// the Go function is the nearest place a throw can actually happen.
//
// What it replaces is a genuine defect: standards and loadTerrain used to
// report failure with jsReject, a rejected *Promise*. A synchronous caller
// that JSON.parses the result got "[object Promise]" — `kernel-node.ts` does
// exactly that — and the rejection nobody awaited surfaced as an unhandled
// one. Both callers already expect a throw: `kernel.worker.ts` wraps the
// handshake's standards() in a try/catch, and reaches loadTerrain() through an
// async function.
//
// The sentence crosses verbatim. serializeKernelError reads `.message` off an
// Error, and these messages are the ones `aconiq run` prints.
const syncThrowSource = `
	return function () {
		const result = call.apply(null, arguments);
		if (result instanceof Error) {
			throw result;
		}
		return result;
	};
`

// throwingSyncExport wraps a synchronous Go export in syncThrowSource.
//
// Building the wrapper needs the Function constructor, which a Content
// Security Policy without 'unsafe-eval' refuses. Nothing this kernel is served
// from sets one — neither the Vite dev server, the gh-pages build nor `aconiq
// serve` emits a CSP header — but a kernel that failed to start would be a far
// worse outcome than a worse error message, so a refusal is contained and the
// bare Go function is registered instead. A failure then reaches the caller as
// an Error object where JSON was expected: still visible, just less readable,
// which is the side of the old defect to fail on.
func throwingSyncExport(fn js.Func) (wrapped js.Value) {
	defer func() {
		if recover() != nil {
			wrapped = fn.Value
		}
	}()

	rethrow := js.Global().Get("Function").New("call", syncThrowSource)

	return rethrow.Invoke(fn)
}

func main() {
	aconiq := js.Global().Get("Object").New()
	aconiq.Set("rls19Road", js.FuncOf(rls19RoadFunc))
	aconiq.Set("rls19RoadShard", js.FuncOf(rls19RoadShardFunc))
	aconiq.Set("transform", js.FuncOf(transformFunc))
	aconiq.Set("contours", js.FuncOf(contoursFunc))
	aconiq.Set("maskFootprints", js.FuncOf(maskFootprintsFunc))
	// The two synchronous exports that can fail go through the rethrowing
	// wrapper; the ones below cannot fail, so they are registered bare.
	aconiq.Set("standards", throwingSyncExport(js.FuncOf(standardsFunc)))
	aconiq.Set("loadTerrain", throwingSyncExport(js.FuncOf(loadTerrainFunc)))
	aconiq.Set("clearTerrain", js.FuncOf(clearTerrainFunc))
	aconiq.Set("defaultConfig", js.FuncOf(defaultConfigFunc))
	aconiq.Set("health", js.FuncOf(healthFunc))
	aconiq.Set("projectStatus", js.FuncOf(projectStatusFunc))
	js.Global().Set("aconiq", aconiq)

	// Block forever to keep registered functions alive.
	select {}
}
