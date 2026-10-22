package httpv1

import (
	"maps"
)

// openapiRunPathItems describes the run lifecycle endpoints.
func openapiRunPathItems() map[string]any {
	items := map[string]any{
		"/api/v1/runs": map[string]any{
			"get": map[string]any{
				"summary":     "List runs (most recent first)",
				"operationId": "listRuns",
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Run summaries ordered newest first",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "array",
									"items": map[string]any{
										"$ref": "#/components/schemas/RunSummary",
									},
								},
							},
						},
					},
					"404": openapiErrorResponse("Project not initialized"),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Internal server error"),
				},
			},
			"post": openapiCreateRunOperation(),
		},
		"/api/v1/runs/{id}/log": map[string]any{
			"get": map[string]any{
				"summary":     "Run log lines",
				"operationId": "getRunLog",
				"parameters": []map[string]any{
					{
						"name":        "id",
						"in":          "path",
						"required":    true,
						"description": "Run ID",
						"schema":      map[string]any{"type": "string"},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Log lines for the requested run",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/RunLog",
								},
							},
						},
					},
					"400": openapiErrorResponse("Missing run ID"),
					"404": openapiErrorResponse("Run not found"),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Failed to read log"),
				},
			},
		},
	}

	items["/api/v1/runs/{id}/provenance"] = openapiRunProvenancePathItem()

	maps.Copy(items, openapiRunResourcePathItems())
	maps.Copy(items, openapiRunContourPathItems())
	maps.Copy(items, openapiArtifactPathItems())

	return items
}

// openapiCreateRunOperation describes POST /api/v1/runs.
func openapiCreateRunOperation() map[string]any {
	return map[string]any{
		"summary":     "Start a run",
		"operationId": "createRun",
		"description": "Starts `aconiq run` under a run id the server mints, and answers `202` as soon " +
			"as the run's row is in the manifest — normally within milliseconds, long before it has " +
			"computed anything. The run is the server's, not the request's: closing the connection " +
			"does not stop it. Poll `GET /api/v1/runs/{id}` (named in `Location`) for its status and " +
			"progress, and stop it with `POST /api/v1/runs/{id}/cancel`. A run the command refuses " +
			"before it creates a row — an unknown standard, a bad parameter — is still answered " +
			"synchronously, with `400`.",
		"requestBody": map[string]any{
			"required": true,
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{
						"$ref": "#/components/schemas/CreateRunRequest",
					},
				},
			},
		},
		"responses": map[string]any{
			"202": map[string]any{
				"description": "The run has started; its summary, normally with status `running`",
				"headers": map[string]any{
					"Location": map[string]any{
						"description": "The run resource to poll",
						"schema":      map[string]any{"type": "string"},
					},
				},
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{
							"$ref": "#/components/schemas/RunSummary",
						},
					},
				},
			},
			"400": openapiErrorResponse("Invalid run request"),
			"404": openapiErrorResponse("Project not initialized"),
			"405": methodNotAllowedResponse(),
			"500": openapiErrorResponse("The run could not be started (`" + errorCodeRunFailed + "`)"),
			"503": openapiErrorResponse("The server is shutting down (`" + errorCodeServerStopping + "`)"),
		},
	}
}

// openapiCancelRunOperation describes POST /api/v1/runs/{id}/cancel.
func openapiCancelRunOperation(runIDParameter []map[string]any, runSummaryContent map[string]any) map[string]any {
	return map[string]any{
		"summary":     "Stop a running run",
		"operationId": "cancelRun",
		"description": "Interrupts the `aconiq run` process computing the run. It stops at its next " +
			"check — before every Teilstück for `rls19-road`, between receivers for the other " +
			"standards — and records the run as `failed`, with `run cancelled` in its log; a process " +
			"still busy ten seconds later is killed and the server records the same. The answer comes " +
			"before either, so the run usually still reads `running` in it — poll " +
			"`GET /api/v1/runs/{id}` for the end. Only a run this server started can be stopped here.",
		"parameters": runIDParameter,
		"responses": map[string]any{
			"202": map[string]any{
				"description": "The run has been told to stop; its summary as it stands",
				"content":     runSummaryContent,
			},
			"404": openapiErrorResponse("Project not initialized, or no run with this ID (`not_found`)"),
			"405": methodNotAllowedResponse(),
			"409": openapiErrorResponse(
				"The run has already finished (`" + errorCodeRunNotRunning + "`), or it is running in a " +
					"process this server did not start (`" + errorCodeRunNotOwned + "`)",
			),
			"500": openapiErrorResponse("The project manifest could not be read; nothing was signalled"),
		},
	}
}

// openapiRunResourcePathItems describes the single-run resource. Go 1.22 routing
// gives the more specific /api/v1/runs/{id}/log precedence over this pattern.
func openapiRunResourcePathItems() map[string]any {
	runIDParameter := []map[string]any{
		{
			"name":        "id",
			"in":          "path",
			"required":    true,
			"description": "Run ID",
			"schema":      map[string]any{"type": "string"},
		},
	}

	runSummaryContent := map[string]any{
		"application/json": map[string]any{
			"schema": map[string]any{
				"$ref": "#/components/schemas/RunSummary",
			},
		},
	}

	return map[string]any{
		"/api/v1/runs/{id}/cancel": map[string]any{
			"post": openapiCancelRunOperation(runIDParameter, runSummaryContent),
		},
		"/api/v1/runs/{id}": map[string]any{
			"get": map[string]any{
				"summary":     "One run, with its progress while it computes",
				"operationId": "getRun",
				"parameters":  runIDParameter,
				"responses": map[string]any{
					"200": map[string]any{
						"description": "The run's summary",
						"content":     runSummaryContent,
					},
					"404": openapiErrorResponse("Project not initialized, or no run with this ID (`not_found`)"),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Internal server error"),
				},
			},
			"delete": map[string]any{
				"summary":     "Delete a run",
				"operationId": "deleteRun",
				"description": "Removes the run from the manifest, drops every artifact ref belonging to it, and " +
					"deletes `.noise/runs/{id}/`. Export bundles under `.noise/exports/` are deliberately kept on " +
					"disk — a bundle may already have been delivered — and the ones whose refs were dropped are " +
					"listed in `retained_paths`. A run that is still `pending` or `running` is refused: its " +
					"directory is being written.",
				"parameters": []map[string]any{
					{
						"name":        "id",
						"in":          "path",
						"required":    true,
						"description": "Run ID",
						"schema":      map[string]any{"type": "string"},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "The run was deleted; the body says what was removed and what was kept",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/DeleteRunResponse",
								},
							},
						},
					},
					"400": openapiErrorResponse("Missing or malformed run ID"),
					"404": openapiErrorResponse("Project not initialized, or no run with this ID (`not_found`)"),
					"405": methodNotAllowedResponse(),
					"409": openapiErrorResponse(
						"The run is still pending or running (`" + errorCodeRunNotFinished +
							"`), or it holds export bundles inside its own directory (`" + errorCodeExportInsideRun +
							"`). Nothing was removed.",
					),
					"500": openapiErrorResponse("Failed to update the manifest or remove the run directory"),
				},
			},
		},
	}
}

// openapiRunSchemas describes the run request and result schemas.
func openapiRunSchemas() map[string]any {
	return map[string]any{
		"RunSummary": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"id", "scenario_id", "standard_id", "version", "status", "started_at", "finished_at", "log_path", "artifacts"},
			"properties": map[string]any{
				"id":              map[string]any{"type": "string"},
				"scenario_id":     map[string]any{"type": "string"},
				"context":         openapiStandardContextSchema(),
				"standard_id":     map[string]any{"type": "string"},
				"version":         map[string]any{"type": "string"},
				"profile":         map[string]any{"type": "string"},
				"receiver_mode":   map[string]any{"type": "string"},
				"receiver_set_id": map[string]any{"type": "string"},
				"status": map[string]any{
					"type": "string",
					"enum": []string{"pending", "running", "completed", "failed"},
				},
				"started_at":  map[string]any{"type": "string", "format": "date-time"},
				"finished_at": map[string]any{"type": "string", "format": "date-time"},
				"log_path":    map[string]any{"type": "string"},
				"artifacts": map[string]any{
					"type":  "array",
					"items": map[string]any{"$ref": "#/components/schemas/ArtifactRef"},
				},
				"progress": map[string]any{"$ref": "#/components/schemas/RunProgress"},
			},
		},
		"RunLog": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"run_id", "lines"},
			"properties": map[string]any{
				"run_id": map[string]any{"type": "string"},
				"lines": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
			},
		},
		"CreateRunRequest": openapiCreateRunRequestSchema(),
		"ImportOSMRequest": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"south", "west", "north", "east"},
			"properties": map[string]any{
				"south": map[string]any{"type": "number"},
				"west":  map[string]any{"type": "number"},
				"north": map[string]any{"type": "number"},
				"east":  map[string]any{"type": "number"},
				"overpass_endpoint": map[string]any{
					"type":   "string",
					"format": "uri",
					"description": "Optional Overpass server. It arrives in a request body, so it is " +
						"constrained rather than trusted: https only, and only a known Overpass host. " +
						"Anything else is refused with error code `overpass_endpoint_not_allowed`, whose " +
						"details.allowed_hosts lists what is accepted. Omit it to use the default server.",
					"examples": allowedOverpassEndpointURLs(),
				},
			},
		},
	}
}

// openapiRunProgressSchemas describes a running run's progress report, and
// the open run the project status carries it on.
func openapiRunProgressSchemas() map[string]any {
	return map[string]any{
		"ActiveRunStatus": map[string]any{
			"type": "object",
			"description": "One open run in `ProjectStatusResponse.active_runs`: the status stream carries " +
				"every one of them, so a client can follow concurrent runs without polling for them.",
			"additionalProperties": false,
			"required":             []string{"id", "status", "standard_id"},
			"properties": map[string]any{
				"id":          map[string]any{"type": "string"},
				"status":      map[string]any{"type": "string"},
				"standard_id": map[string]any{"type": "string"},
				"progress":    map[string]any{"$ref": "#/components/schemas/RunProgress"},
			},
		},
		"RunProgress": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"done", "total", "started_at", "updated_at"},
			"description": "How far a run's compute has got, as the process computing it last reported — at " +
				"most once a second, and always at the first and the last receiver. Present only while the " +
				"run is `pending` or `running` and its standard reports progress (every standard but " +
				"`dummy-freefield`; `beb-exposure` counts the facade receivers it computes levels for). " +
				"Absent means not known, never nothing done.",
			"properties": map[string]any{
				"done":  map[string]any{"type": "integer", "description": "Receivers computed so far"},
				"total": map[string]any{"type": "integer", "description": "Receivers the run computes in total"},
				"started_at": map[string]any{
					"type": "string", "format": "date-time",
					"description": "When the receiver walk began, after the model was loaded and the scene prepared",
				},
				"updated_at": map[string]any{"type": "string", "format": "date-time", "description": "When this report was written"},
				"eta_seconds": map[string]any{
					"type": "number",
					"description": "Seconds left at the rate so far: elapsed / done * (total - done). Absent " +
						"until a receiver has completed. A straight-line estimate, so it is only as good as " +
						"receivers are alike in cost.",
				},
			},
		},
	}
}

// openapiRunDeleteSchemas describes what a run delete answers with.
func openapiRunDeleteSchemas() map[string]any {
	return map[string]any{
		"DeleteRunResponse": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"run_id", "removed_paths", "retained_paths"},
			"properties": map[string]any{
				"run_id": map[string]any{"type": "string"},
				"removed_paths": map[string]any{
					"type":        "array",
					"description": "Project-relative paths that were deleted. Always present, empty rather than null.",
					"items":       map[string]any{"type": "string"},
				},
				"retained_paths": map[string]any{
					"type": "array",
					"description": "Files whose manifest refs were dropped but whose bytes were deliberately left " +
						"in place — export bundles. Always present, empty rather than null.",
					"items": map[string]any{"type": "string"},
				},
			},
		},
	}
}

// allowedOverpassEndpointURLs renders the Overpass allowlist as the URLs a
// caller may actually send, so the document does not restate the list by hand.
func allowedOverpassEndpointURLs() []string {
	urls := make([]string, 0, len(allowedOverpassHosts))
	for _, host := range allowedOverpassHosts {
		urls = append(urls, "https://"+host+"/api/interpreter")
	}

	return urls
}

// openapiRunProvenancePathItem describes GET /api/v1/runs/{id}/provenance.
func openapiRunProvenancePathItem() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"summary":     "A run's provenance.json",
			"operationId": "getRunProvenance",
			"description": "Returns `.noise/runs/<id>/provenance.json` byte for byte, as the run wrote it: the " +
				"standard it resolved, its parameters, the input hashes, the standard-data digest and the tool " +
				"that produced it. It is never re-encoded, so a field an older or newer build wrote is served too.",
			"parameters": []map[string]any{
				{
					"name":        "id",
					"in":          "path",
					"required":    true,
					"description": "Run ID",
					"schema":      map[string]any{"type": "string"},
				},
			},
			"responses": map[string]any{
				"200": map[string]any{
					"description": "The run's provenance manifest",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"$ref": "#/components/schemas/RunProvenance",
							},
						},
					},
				},
				"403": openapiErrorResponse("The manifest names a provenance path outside the project root (`" + errorCodeForbiddenPath + "`)"),
				"404": openapiErrorResponse("Project not initialized, run not found, or its provenance.json is missing"),
				"405": methodNotAllowedResponse(),
				"500": openapiErrorResponse("Failed to read the provenance file"),
			},
		},
	}
}

// openapiRunProvenanceSchemas describes provenance.json. It stays open
// (additionalProperties) because the route serves the file as written: a run
// recorded by another build may carry fields this one does not, and
// tool_commit and tool_build_date are absent from runs older than them.
func openapiRunProvenanceSchemas() map[string]any {
	stringMap := map[string]any{
		"type":                 "object",
		"additionalProperties": map[string]any{"type": "string"},
	}

	return map[string]any{
		"RunProvenance": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
			"required":             []string{"run_id", "scenario_id", "standard", "input_hashes", "generated_at", "tool_name", "tool_version"},
			"properties": map[string]any{
				"run_id":      map[string]any{"type": "string"},
				"scenario_id": map[string]any{"type": "string"},
				"standard": map[string]any{
					"type":     "object",
					"required": []string{"id", "version"},
					"properties": map[string]any{
						"context": openapiStandardContextSchema(),
						"id":      map[string]any{"type": "string"},
						"version": map[string]any{"type": "string"},
						"profile": map[string]any{"type": "string"},
					},
				},
				"receiver_mode":   map[string]any{"type": "string"},
				"receiver_set_id": map[string]any{"type": "string"},
				"parameters":      stringMap,
				"metadata": map[string]any{
					"type":                 "object",
					"additionalProperties": map[string]any{"type": "string"},
					"description":          "Module metadata, including `evidence_tier`, `project_crs` and `compute_crs`.",
				},
				"input_hashes": map[string]any{
					"type":                 "object",
					"additionalProperties": map[string]any{"type": "string"},
					"description":          "Project-relative input path to its SHA-256, as bare lowercase hex.",
				},
				"standard_data": map[string]any{
					"type":     "object",
					"required": []string{"algorithm", "digest", "evidence_tier"},
					"properties": map[string]any{
						"algorithm":     map[string]any{"type": "string"},
						"digest":        map[string]any{"type": "string"},
						"evidence_tier": map[string]any{"type": "string"},
						"tables": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":     "object",
								"required": []string{"name", "digest"},
								"properties": map[string]any{
									"name":   map[string]any{"type": "string"},
									"digest": map[string]any{"type": "string"},
								},
							},
						},
					},
				},
				"generated_at":    map[string]any{"type": "string", "format": "date-time"},
				"tool_name":       map[string]any{"type": "string"},
				"tool_version":    map[string]any{"type": "string"},
				"tool_commit":     map[string]any{"type": "string", "description": "Short source commit of the build, or `unknown`"},
				"tool_build_date": map[string]any{"type": "string", "description": "RFC 3339 build date, or `unknown`"},
			},
		},
	}
}

// openapiCreateRunRequestSchema is the body of POST /api/v1/runs and of its
// estimate: what a run is asked to compute, and the opt-ins it may need.
func openapiCreateRunRequestSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"scenario_id":      map[string]any{"type": "string"},
			"standard_id":      map[string]any{"type": "string"},
			"standard_version": map[string]any{"type": "string"},
			"standard_profile": map[string]any{"type": "string"},
			"model_path":       map[string]any{"type": "string"},
			"receiver_mode": map[string]any{
				"type":        "string",
				"enum":        []string{"auto-grid", "custom", "facade"},
				"description": "facade generates receivers on the facades of facade_buildings, at the placement rule of the standard (rls19-road and schall03 only).",
			},
			"facade_buildings": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string", "minLength": 1},
				"description": "Buildings that carry facade receivers, each a model building feature id or \"<street> <number>\" as its addr:street and addr:housenumber spell it (compared without case and extra spaces; an address shared by two buildings is refused). Required with receiver_mode facade.",
			},
			"facade_spacing_m": map[string]any{
				"type":             "number",
				"exclusiveMinimum": 0,
				"description":      "Station spacing along each facade, in metres. Required with receiver_mode facade; there is no default.",
			},
			"facade_include_courtyard": map[string]any{
				"type":        "boolean",
				"default":     false,
				"description": "Also place facade receivers on courtyard (inner-ring) facades.",
			},
			"allow_unconfirmed": map[string]any{
				"type":        "boolean",
				"default":     false,
				"description": "Required for a normative run over facade receivers that are still proposed or whose slab height is assumed; their counts are then stamped into provenance, the run summary and the report.",
			},
			"params": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"},
			},
			"input_paths": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"experimental": map[string]any{
				"type":        "boolean",
				"default":     false,
				"description": "Required for scaffold-tier standards: they carry no normative coefficients, their base levels are invented and they have no octave bands, so a run must acknowledge that before it may emit levels. Requests without it are rejected with error code experimental_opt_in_required.",
			},
		},
	}
}
