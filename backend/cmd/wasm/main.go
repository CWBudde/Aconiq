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
//	}));
//	const outputs = JSON.parse(result); // []ReceiverOutput
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
}

// rls19RoadFunc computes RLS-19 road traffic noise levels.
// Takes a single JSON string argument, returns a Promise<string> (JSON).
func rls19RoadFunc(_ js.Value, args []js.Value) any {
	if len(args) != 1 {
		return jsReject("rls19Road: expected exactly 1 JSON string argument")
	}

	input := args[0].String()

	return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, promArgs []js.Value) any {
		resolve, reject := promArgs[0], promArgs[1]

		var req computeRequest
		if err := json.Unmarshal([]byte(input), &req); err != nil {
			reject.Invoke(js.ValueOf(fmt.Sprintf("rls19Road: invalid input JSON: %v", err)))
			return nil
		}

		req.Config = withPropagationDefaults(req.Config)

		// The terrain is the ground h_m is measured above as well as the one
		// elevation the receiver heights stack on, so both travel into the
		// config — and both in the CRS this request computes in. Browser mode
		// resolves the ground the way `aconiq run` does or it is answering a
		// different question.
		cfg, err := wasmkernel.ApplyTerrain(req.Config, req.Receivers, req.Projection)
		if err != nil {
			reject.Invoke(js.ValueOf(fmt.Sprintf("rls19Road: %v", err)))

			return nil
		}

		req.Config = cfg

		outputs, err := road.ComputeReceiverOutputs(req.Receivers, req.Sources, req.Barriers, req.Config)
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

// standardsFunc returns the standards this kernel can run, in the same JSON
// shape `GET /api/v1/standards` answers with.
// Signature: () => string (JSON)
//
// Unlike defaultConfig/health/projectStatus below, the marshal error is not
// swallowed: an empty standards list renders the run page unusable, and a
// silent one would look like a kernel that supports nothing.
func standardsFunc(_ js.Value, _ []js.Value) any {
	out, err := wasmkernel.StandardsJSON()
	if err != nil {
		return jsReject(fmt.Sprintf("standards: marshal error: %v", err))
	}

	return js.ValueOf(string(out))
}

// loadTerrainFunc loads a GeoTIFF terrain model from a Uint8Array.
// Signature: (data: Uint8Array, crs: string) => string (terrain.Info JSON)
//
// The CRS is a second argument rather than something the kernel works out: the
// GeoTIFF loader reads the tie point and the pixel scale and no
// GeoKeyDirectory, so the file does not say what it is in, and the request that
// later queries it carries coordinates that name no CRS either. The caller is
// the only one who knows, so the caller states it.
func loadTerrainFunc(_ js.Value, args []js.Value) any {
	if len(args) != 2 {
		return jsReject("loadTerrain: expected 2 arguments (Uint8Array data, string crs)")
	}

	jsArr := args[0]
	length := jsArr.Get("byteLength").Int()
	buf := make([]byte, length)
	js.CopyBytesToGo(buf, jsArr)

	info, err := wasmkernel.LoadTerrain(buf, args[1].String())
	if err != nil {
		return jsReject(fmt.Sprintf("loadTerrain: %v", err))
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

func main() {
	aconiq := js.Global().Get("Object").New()
	aconiq.Set("rls19Road", js.FuncOf(rls19RoadFunc))
	aconiq.Set("transform", js.FuncOf(transformFunc))
	aconiq.Set("standards", js.FuncOf(standardsFunc))
	aconiq.Set("loadTerrain", js.FuncOf(loadTerrainFunc))
	aconiq.Set("clearTerrain", js.FuncOf(clearTerrainFunc))
	aconiq.Set("defaultConfig", js.FuncOf(defaultConfigFunc))
	aconiq.Set("health", js.FuncOf(healthFunc))
	aconiq.Set("projectStatus", js.FuncOf(projectStatusFunc))
	js.Global().Set("aconiq", aconiq)

	// Block forever to keep registered functions alive.
	select {}
}
