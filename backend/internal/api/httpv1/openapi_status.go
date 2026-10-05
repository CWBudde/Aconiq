package httpv1

// openapiStatusPathItems describes the status and discovery endpoints.
func openapiStatusPathItems() map[string]any {
	return map[string]any{
		"/api/v1/health": map[string]any{
			"get": map[string]any{
				"summary":     "Health check",
				"operationId": "getHealth",
				"responses": map[string]any{
					"200": map[string]any{
						"description": "API is healthy",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/HealthResponse",
								},
							},
						},
					},
					"405": methodNotAllowedResponse(),
				},
			},
		},
		"/api/v1/project/status": map[string]any{
			"get": map[string]any{
				"summary":     "Project status",
				"operationId": "getProjectStatus",
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Loaded project status",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/ProjectStatusResponse",
								},
							},
						},
					},
					"404": openapiErrorResponse("Project not initialized"),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Internal server error"),
				},
			},
		},
		"/api/v1/standards": map[string]any{
			"get": map[string]any{
				"summary":     "List available noise standards",
				"operationId": "listStandards",
				"responses": map[string]any{
					"200": map[string]any{
						"description": "List of registered noise standards with their versions and profiles",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "array",
									"items": map[string]any{
										"$ref": "#/components/schemas/StandardDescriptor",
									},
								},
							},
						},
					},
					"405": methodNotAllowedResponse(),
					"503": openapiErrorResponse("Standards registry not configured"),
				},
			},
		},
		"/api/v1/openapi.json": map[string]any{
			"get": map[string]any{
				"summary":     "OpenAPI v1 document",
				"operationId": "getOpenAPI",
				"responses": map[string]any{
					"200": map[string]any{
						"description": "This OpenAPI document",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
								},
							},
						},
					},
					"405": methodNotAllowedResponse(),
				},
			},
		},
	}
}

// openapiProjectSchemas describes the health, project status and artifact schemas.
func openapiProjectSchemas() map[string]any {
	return map[string]any{
		"HealthResponse": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"status", "version", "time"},
			"properties": map[string]any{
				"status":  map[string]any{"type": "string"},
				"version": map[string]any{"type": "string"},
				"time":    map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"LastRunStatus": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"id", "status", "standard_id", "version", "started_at", "finished_at"},
			"properties": map[string]any{
				"id":          map[string]any{"type": "string"},
				"status":      map[string]any{"type": "string"},
				"context":     openapiStandardContextSchema(),
				"standard_id": map[string]any{"type": "string"},
				"version":     map[string]any{"type": "string"},
				"profile":     map[string]any{"type": "string"},
				"started_at":  map[string]any{"type": "string", "format": "date-time"},
				"finished_at": map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"ProjectStatusResponse": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"project_id", "name", "project_path", "manifest_version", "crs", "scenario_count", "run_count"},
			"properties": map[string]any{
				"project_id":       map[string]any{"type": "string"},
				"name":             map[string]any{"type": "string"},
				"project_path":     map[string]any{"type": "string"},
				"manifest_version": map[string]any{"type": "integer"},
				"crs":              map[string]any{"type": "string"},
				"scenario_count":   map[string]any{"type": "integer"},
				"run_count":        map[string]any{"type": "integer"},
				"last_run":         map[string]any{"$ref": "#/components/schemas/LastRunStatus"},
				"model":            map[string]any{"$ref": "#/components/schemas/ProjectModelStatus"},
			},
		},
		"ProjectModelStatus": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"hash", "updated_at"},
			"description": "The saved model's receipt. Absent until a model has been saved. A client that kept " +
				"the hash POST /api/v1/model returned compares the two as strings to learn whether its local draft " +
				"is still the project's model, without fetching anything.",
			"properties": map[string]any{
				"hash":       openapiModelHashSchema(),
				"updated_at": map[string]any{"type": "string", "format": "date-time", "description": "When the model artifact was last written"},
			},
		},
		"ArtifactRef": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"id", "kind", "path", "created_at"},
			"properties": map[string]any{
				"id":         map[string]any{"type": "string"},
				"kind":       map[string]any{"type": "string"},
				"path":       map[string]any{"type": "string"},
				"created_at": map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"TerrainInfo": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"bounds", "pixel_size", "grid_size"},
			"properties": map[string]any{
				"bounds": map[string]any{
					"type":        "array",
					"minItems":    4,
					"maxItems":    4,
					"description": "Bounding box [min_x, min_y, max_x, max_y]",
					"items":       map[string]any{"type": "number"},
				},
				"pixel_size": map[string]any{
					"type":        "array",
					"minItems":    2,
					"maxItems":    2,
					"description": "Pixel size [width, height] in CRS units",
					"items":       map[string]any{"type": "number"},
				},
				"grid_size": map[string]any{
					"type":        "array",
					"minItems":    2,
					"maxItems":    2,
					"description": "Raster dimensions [width, height]",
					"items":       map[string]any{"type": "integer"},
				},
			},
		},
	}
}

// openapiStandardSchemas describes the standards registry schemas.
func openapiStandardSchemas() map[string]any {
	return map[string]any{
		"ParameterDefinition": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"name", "kind", "required"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"kind": map[string]any{"type": "string", "enum": []string{"string", "bool", "int", "float"}},
				"unit": map[string]any{
					"type":        "string",
					"description": "Physical unit of the value as a short symbol (m, km/h, dB, dB/km, 1/h, 1/km, %, °C, °). Absent when the parameter is dimensionless or not numeric.",
				},
				"required":      map[string]any{"type": "boolean"},
				"default_value": map[string]any{"type": "string"},
				"description":   map[string]any{"type": "string"},
				"enum": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"min": map[string]any{"type": "number"},
				"max": map[string]any{"type": "number"},
			},
		},
		"ProfileInfo": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"name", "supported_source_types", "supported_indicators", "parameters"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"supported_source_types": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"supported_indicators": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"parameters": map[string]any{
					"type":  "array",
					"items": map[string]any{"$ref": "#/components/schemas/ParameterDefinition"},
				},
			},
		},
		"VersionInfo": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"name", "default_profile", "profiles"},
			"properties": map[string]any{
				"name":            map[string]any{"type": "string"},
				"default_profile": map[string]any{"type": "string"},
				"profiles": map[string]any{
					"type":  "array",
					"items": map[string]any{"$ref": "#/components/schemas/ProfileInfo"},
				},
			},
		},
		"StandardDescriptor": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			// context has no omitempty on descriptorjson.Standard, so the key is always
			// present and a strict consumer may rely on it.
			"required": []string{"context", "id", "description", evidenceTierField, "default_version", "versions"},
			"properties": map[string]any{
				"context":     openapiStandardContextSchema(),
				"id":          map[string]any{"type": "string"},
				"description": map[string]any{"type": "string"},
				evidenceTierField: map[string]any{
					"type": "string",
					"description": "How far this module's output may be trusted. " +
						"`normative` implements the named standard's tables and equations within the boundary declared under docs/conformance/; " +
						"`preview` has sound logic but consumes preview-grade input levels; " +
						"`scaffold` carries no normative coefficients and its levels are invented; " +
						"`test-fixture` is a non-normative demonstrator. Only `normative` output is usable for assessment.",
					"enum": []string{"normative", "preview", "scaffold", "test-fixture"},
				},
				"default_version": map[string]any{"type": "string"},
				"versions": map[string]any{
					"type":  "array",
					"items": map[string]any{"$ref": "#/components/schemas/VersionInfo"},
				},
			},
		},
	}
}
