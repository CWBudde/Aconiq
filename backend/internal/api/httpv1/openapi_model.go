package httpv1

// openapiModelPathItems describes the model endpoint.
func openapiModelPathItems() map[string]any {
	return map[string]any{
		"/api/v1/model": map[string]any{
			"get": map[string]any{
				"summary":     "Read the project model",
				"operationId": "getModel",
				"description": "Returns the normalized model `.noise/model/model.normalized.geojson` holds, together " +
					"with its hash. Without `crs` the stored bytes are returned untouched, so the payload is exactly " +
					"what the hash is a receipt for; with `crs` every coordinate is reprojected into that CRS first and " +
					"`crs` in the response names what the coordinates are actually in. The hash is unchanged by the " +
					"reprojection: it always describes the stored file.",
				"parameters": []map[string]any{
					{
						"name":     "crs",
						"in":       "query",
						"required": false,
						"description": "CRS to return the coordinates in, e.g. `EPSG:4326` to draw the model on a web " +
							"map. Omit it to receive the model in the project CRS, unmodified.",
						"schema": map[string]any{"type": "string"},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "The stored model",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/ModelResponse",
								},
							},
						},
					},
					"400": openapiErrorResponse("`crs` is not a recognised CRS identifier (`bad_request`)"),
					"404": openapiErrorResponse(
						"The project is not initialized (`not_found`), or it is but no model has been saved yet " +
							"(`" + errorCodeModelNotFound + "`). The two are separate codes because a client has to " +
							"be able to tell them apart.",
					),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Failed to read the stored model"),
				},
			},
			"post": map[string]any{
				"summary":     "Replace the project model",
				"operationId": "saveModel",
				"description": "Accepts a GeoJSON FeatureCollection in the v1 input schema — the same document " +
					"`aconiq import --input` takes — normalises it into the project CRS, validates it and " +
					"writes `.noise/model/model.normalized.geojson`, the model dump and the validation report, " +
					"registering the three as project artifacts. The previous model is replaced. " +
					"`aconiq run` and `POST /api/v1/runs` read the normalized model from there by default.",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"$ref": "#/components/schemas/ModelSaveRequest",
							},
						},
					},
				},
				"responses": map[string]any{
					"201": map[string]any{
						"description": "Model written; the paths are project-relative",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/ModelSaveResponse",
								},
							},
						},
					},
					"400": openapiErrorResponse(
						"Malformed body, missing `model`, or a document that is not a FeatureCollection (`bad_request`); " +
							"or a model that parsed but failed schema validation (`model_invalid`, whose details.errors " +
							"lists each finding as a ValidationIssue with its feature_id). Nothing is written in either case.",
					),
					"404": openapiErrorResponse("Project not initialized"),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Failed to persist the model"),
				},
			},
		},
	}
}

// openapiModelSchemas describes the model endpoint's request and response.
func openapiModelSchemas() map[string]any {
	return map[string]any{
		"ModelSaveRequest": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"model"},
			"properties": map[string]any{
				"crs": map[string]any{
					"type": "string",
					"description": "CRS of the model's coordinates, e.g. `EPSG:4326` for coordinates drawn on a " +
						"web map. Like `aconiq import --input-crs`: when it differs from the project CRS every " +
						"coordinate is reprojected; when omitted the coordinates are taken to be in the project CRS already.",
				},
				"model": map[string]any{
					"type":        "object",
					"description": "GeoJSON FeatureCollection in the v1 input schema (docs/geojson-schema-v1.md): features carry `kind` = source | building | barrier | receiver | calc-area | ground-zone.",
				},
			},
		},
		"ModelSaveResponse": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"normalized_path", "dump_path", "validation_report_path", "feature_count", "hash", "warnings"},
			"properties": map[string]any{
				"normalized_path":        map[string]any{"type": "string", "description": "Project-relative path of the normalized GeoJSON"},
				"dump_path":              map[string]any{"type": "string", "description": "Project-relative path of the model dump"},
				"validation_report_path": map[string]any{"type": "string", "description": "Project-relative path of the validation report"},
				"feature_count":          map[string]any{"type": "integer"},
				"hash":                   openapiModelHashSchema(),
				"warnings": map[string]any{
					"type":        "array",
					"description": "Validation warnings. The model was written despite them.",
					"items":       map[string]any{"$ref": "#/components/schemas/ValidationIssue"},
				},
			},
		},
		"ModelResponse": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"crs", "project_crs", "hash", "feature_count", "model"},
			"properties": map[string]any{
				"crs": map[string]any{
					"type": "string",
					"description": "The CRS the returned coordinates are in — the requested one, or the project " +
						"CRS when none was requested. Stated rather than inferred.",
				},
				"project_crs":   map[string]any{"type": "string", "description": "The project's own CRS"},
				"hash":          openapiModelHashSchema(),
				"feature_count": map[string]any{"type": "integer"},
				"model": map[string]any{
					"type":        "object",
					"description": "The normalized GeoJSON FeatureCollection. Returned verbatim from disk unless `crs` asked for a reprojection.",
				},
			},
		},
		"ValidationIssue": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"code", "message"},
			"properties": map[string]any{
				"code":       map[string]any{"type": "string"},
				"feature_id": map[string]any{"type": "string", "description": "The feature the finding is about; absent for model-wide findings"},
				"message":    map[string]any{"type": "string"},
			},
		},
	}
}
