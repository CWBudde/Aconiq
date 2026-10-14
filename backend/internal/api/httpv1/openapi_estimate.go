package httpv1

// openapiRunEstimatePathItems describes POST /api/v1/runs/estimate.
func openapiRunEstimatePathItems() map[string]any {
	return map[string]any{
		"/api/v1/runs/estimate": map[string]any{
			"post": map[string]any{
				"summary":     "Estimate what a run would cost in time",
				"operationId": "estimateRun",
				"description": "Runs `aconiq run --estimate` with the request a `POST /api/v1/runs` would " +
					"take: the run's pipeline up to its prepared scene, then a probe of a few seconds " +
					"that times a stratified sample of at most 16 receivers and, for each, a sample of " +
					"its own work units (for `rls19-road`, its (source, Teilstück) pairs). No run is " +
					"created and the manifest is not written. The request is refused where the run " +
					"would be, in the run's words. A standard that has no probe answers `200` with " +
					"`available: false` and a `reason`. Closing the connection interrupts the probe.",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{"$ref": "#/components/schemas/CreateRunRequest"},
						},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "The estimate",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/RunEstimate"},
							},
						},
					},
					"400": openapiErrorResponse("Invalid run request, or one the run command refuses"),
					"404": openapiErrorResponse("Project not initialized"),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("The estimate could not be taken (`" + errorCodeRunFailed + "`)"),
				},
			},
		},
	}
}

// openapiRunEstimateSchemas describes runestimate.Estimate.
func openapiRunEstimateSchemas() map[string]any {
	band := func(description string) map[string]any {
		return map[string]any{
			"allOf":       []map[string]any{{"$ref": "#/components/schemas/EstimateBand"}},
			"description": description,
		}
	}

	number := func(description string) map[string]any {
		return map[string]any{"type": "number", "description": description}
	}

	integer := func(description string) map[string]any {
		return map[string]any{"type": "integer", "description": description}
	}

	return map[string]any{
		"EstimateBand": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"low", "estimate", "high"},
			"description": "A duration range in seconds: a point estimate and the band the probe's own " +
				"spread and sampling error put around it, widened for what a few seconds of probing " +
				"cannot see (clock frequency, other load, parallel efficiency).",
			"properties": map[string]any{
				"low":      number("Seconds"),
				"estimate": number("Seconds"),
				"high":     number("Seconds"),
			},
		},
		"RunEstimate": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"available"},
			"description": "What a run will cost in time. The same JSON `aconiq run --estimate --json` " +
				"prints under `estimate` and the WASM kernel's `estimate` answers with.",
			"properties": map[string]any{
				"available": map[string]any{
					"type":        "boolean",
					"description": "False for a standard without a probe; every field after `reason` is then absent.",
				},
				"reason":             map[string]any{"type": "string", "description": "Why there is no estimate"},
				"standard_id":        map[string]any{"type": "string"},
				"wall_s":             band("Elapsed time on the stated workers, scene preparation included"),
				"cpu_s":              band("Compute summed over all workers, at single-worker speed"),
				"cpu_per_receiver_s": band("`cpu_s` over the receiver count"),
				"parallelism":        number("The speed-up over one worker the point estimate assumes, after the chunk schedule"),
				"inputs": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"receivers", "workers", "units"},
					"properties": map[string]any{
						"receivers": integer("Receivers the run computes"),
						"workers":   integer("Workers the estimate assumes"),
						"units":     integer("Work units over all receivers — for `rls19-road`, (source, Teilstück) pairs in the run's own segment_length_mode, plus Parkplätze"),
						"counts": map[string]any{
							"type":                 "object",
							"description":          "The standard's own inputs by name, e.g. `buildings`, `reflector_walls`, `teilstuecke`",
							"additionalProperties": map[string]any{"type": "integer"},
						},
					},
				},
				"probe": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required": []string{
						"probe_receivers", "measured_receivers", "units_timed",
						"budget_s", "elapsed_s", "prepare_s", "budget_exhausted",
					},
					"properties": map[string]any{
						"probe_receivers":    integer("Receivers chosen for the probe"),
						"measured_receivers": integer("Probe receivers the budget reached"),
						"units_timed":        integer("Work units the probe ran"),
						"budget_s":           number("The probe's wall-clock budget"),
						"elapsed_s":          number("What the probe took"),
						"prepare_s":          number("Scene preparation, measured"),
						"budget_exhausted":   map[string]any{"type": "boolean"},
					},
				},
				"alternatives": map[string]any{
					"type":        "array",
					"description": "Variants of the run costed by their own probe — today the distance-scaled Teilstücke of a fixed-mode RLS-19 run",
					"items": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"required":             []string{"params", "units", "cpu_s", "wall_s"},
						"properties": map[string]any{
							"params": map[string]any{
								"type":                 "object",
								"additionalProperties": map[string]any{"type": "string"},
							},
							"units":  integer("Work units of the variant"),
							"cpu_s":  map[string]any{"$ref": "#/components/schemas/EstimateBand"},
							"wall_s": map[string]any{"$ref": "#/components/schemas/EstimateBand"},
						},
					},
				},
			},
		},
	}
}
