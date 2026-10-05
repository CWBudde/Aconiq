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
			"post": map[string]any{
				"summary":     "Create and execute a run",
				"operationId": "createRun",
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
					"201": map[string]any{
						"description": "Created run summary",
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
					"500": openapiErrorResponse("Run execution failed"),
				},
			},
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

	maps.Copy(items, openapiRunResourcePathItems())
	maps.Copy(items, openapiRunContourPathItems())
	maps.Copy(items, openapiArtifactPathItems())

	return items
}

// openapiRunResourcePathItems describes the single-run resource. Go 1.22 routing
// gives the more specific /api/v1/runs/{id}/log precedence over this pattern.
func openapiRunResourcePathItems() map[string]any {
	return map[string]any{
		"/api/v1/runs/{id}": map[string]any{
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
		"CreateRunRequest": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"scenario_id":      map[string]any{"type": "string"},
				"standard_id":      map[string]any{"type": "string"},
				"standard_version": map[string]any{"type": "string"},
				"standard_profile": map[string]any{"type": "string"},
				"model_path":       map[string]any{"type": "string"},
				"receiver_mode":    map[string]any{"type": "string", "enum": []string{"auto-grid", "custom"}},
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
		},
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
