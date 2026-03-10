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
package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
	"time"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/standards/rls19/road"
)

// currentTerrain holds the terrain model loaded via loadTerrain().
// It is automatically used by compute functions when non-nil.
var currentTerrain terrain.Model

type computeRequest struct {
	Receivers []geo.PointReceiver    `json:"receivers"`
	Sources   []road.RoadSource      `json:"sources"`
	Barriers  []road.Barrier         `json:"barriers"`
	Config    road.PropagationConfig `json:"config"`
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

		// Apply defaults when config is zero-valued.
		if req.Config.SegmentLengthM == 0 && req.Config.MinDistanceM == 0 && req.Config.ReceiverHeightM == 0 {
			req.Config = road.DefaultPropagationConfig()
		}

		// Apply terrain elevation if a terrain model is loaded.
		if currentTerrain != nil && len(req.Receivers) > 0 {
			req.Config.ReceiverTerrainZ = terrainAtGridCenter(currentTerrain, req.Receivers)
		}

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

// loadTerrainFunc loads a GeoTIFF terrain model from a Uint8Array.
// Returns a JSON string with terrain metadata (bounds, pixelSize, gridSize).
func loadTerrainFunc(_ js.Value, args []js.Value) any {
	if len(args) != 1 {
		return jsReject("loadTerrain: expected exactly 1 Uint8Array argument")
	}

	jsArr := args[0]
	length := jsArr.Get("byteLength").Int()
	buf := make([]byte, length)
	js.CopyBytesToGo(buf, jsArr)

	model, err := terrain.LoadFromBytes(buf)
	if err != nil {
		return jsReject(fmt.Sprintf("loadTerrain: %v", err))
	}

	currentTerrain = model

	info, _ := json.Marshal(model.Info())

	return js.ValueOf(string(info))
}

// clearTerrainFunc removes the currently loaded terrain model.
func clearTerrainFunc(_ js.Value, _ []js.Value) any {
	currentTerrain = nil
	return js.Undefined()
}

// terrainAtGridCenter queries terrain elevation at the centroid of receivers.
func terrainAtGridCenter(tm terrain.Model, receivers []geo.PointReceiver) float64 {
	var sumX, sumY float64

	for _, r := range receivers {
		sumX += r.Point.X
		sumY += r.Point.Y
	}

	n := float64(len(receivers))
	elev, ok := tm.ElevationAt(sumX/n, sumY/n)
	if !ok {
		return 0
	}

	return elev
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
	aconiq.Set("loadTerrain", js.FuncOf(loadTerrainFunc))
	aconiq.Set("clearTerrain", js.FuncOf(clearTerrainFunc))
	aconiq.Set("defaultConfig", js.FuncOf(defaultConfigFunc))
	aconiq.Set("health", js.FuncOf(healthFunc))
	aconiq.Set("projectStatus", js.FuncOf(projectStatusFunc))
	js.Global().Set("aconiq", aconiq)

	// Block forever to keep registered functions alive.
	select {}
}
