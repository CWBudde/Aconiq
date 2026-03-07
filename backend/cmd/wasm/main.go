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

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/rls19/road"
)

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

// defaultConfigFunc returns the default PropagationConfig as a JSON string.
func defaultConfigFunc(_ js.Value, _ []js.Value) any {
	cfg := road.DefaultPropagationConfig()
	out, _ := json.Marshal(cfg)
	return js.ValueOf(string(out))
}

func jsReject(msg string) js.Value {
	return js.Global().Get("Promise").Call("reject", js.ValueOf(msg))
}

func main() {
	aconiq := js.Global().Get("Object").New()
	aconiq.Set("rls19Road", js.FuncOf(rls19RoadFunc))
	aconiq.Set("defaultConfig", js.FuncOf(defaultConfigFunc))
	js.Global().Set("aconiq", aconiq)

	// Block forever to keep registered functions alive.
	select {}
}
