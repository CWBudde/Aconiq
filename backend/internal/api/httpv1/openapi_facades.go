package httpv1

// openapiFacadePathItems describes the facade elevation endpoint.
func openapiFacadePathItems() map[string]any {
	return map[string]any{
		"/api/v1/facades/elevation": map[string]any{
			"post": map[string]any{
				"summary":     "Unfold a building into its facades, storeys and windows",
				"operationId": "facadeElevation",
				"description": "Answers what the elevation view draws for one building of the model in the body: each " +
					"canonical edge in canonical order, with its key's endpoints in the request's `crs` and its length " +
					"in metres; the storeys' slab heights facade receiver mode places at; and the window features " +
					"keyed to each edge, with what validation finds about them. A window keyed to the building whose key " +
					"no longer names one of its edges as it is (`window.facade.moved`) is listed under `detached` " +
					"instead. A window whose `building_ref` names no building of the model " +
					"(`window.facade.unresolved`) is in no building's elevation: nothing says which building it meant, " +
					"and the validation report lists it. Like `/api/v1/transform` it reads no project and writes nothing, so it answers for the " +
					"model a client holds, saved or not. The browser's WebAssembly kernel answers the same question " +
					"from the same Go code (`aconiq.facadeElevation`).",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{"$ref": "#/components/schemas/FacadeElevationRequest"},
						},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "The building unfolded",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/FacadeElevationResponse"},
							},
						},
					},
					"400": openapiErrorResponse(
						"Malformed body, a missing `crs`, `model` or `building`, a model that is not a valid " +
							"FeatureCollection, a building that is not in the model or has no height, or a geographic " +
							"site with no compute CRS (`" + errorCodeBadRequest + "`).",
					),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Failed to encode the elevation"),
				},
			},
		},
	}
}

// openapiFacadeSchemas describes the facade elevation request and response.
func openapiFacadeSchemas() map[string]any {
	number := map[string]any{"type": "number"}
	pair := map[string]any{"type": "array", "items": number, "minItems": 2, "maxItems": 2}

	return map[string]any{
		"FacadeElevationRequest": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"crs", "model", facadeBuildingField},
			"properties": map[string]any{
				"crs": map[string]any{
					"type": "string",
					"description": "CRS the model's coordinates are in. Send the compute CRS a facade run would use, " +
						"so the facades are the edges its keys are numbered on.",
				},
				"model": map[string]any{
					"type":        "object",
					"description": "A GeoJSON FeatureCollection in the v1 model schema, holding the building and its windows.",
				},
				facadeBuildingField: map[string]any{"type": "string", "description": "The building's feature id."},
			},
		},
		"FacadeElevationResponse": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"building_ref", "height_m", "slab_heights_m", "slab_height_source", "facades", "detached"},
			"properties": map[string]any{
				"building_ref": map[string]any{"type": "string"},
				"height_m":     number,
				"levels": map[string]any{
					"type":        "integer",
					"description": "`building:levels`, absent when the building states no positive integer count.",
				},
				"slab_heights_m": map[string]any{
					"type":        "array",
					"items":       number,
					"description": "Each storey's ceiling slab above the ground, ground floor first.",
				},
				"slab_height_source": map[string]any{"type": "string", "enum": []string{"measured", "levels", "assumed"}},
				"facades": map[string]any{
					"type":  "array",
					"items": map[string]any{"$ref": "#/components/schemas/FacadeElevation"},
				},
				"detached": map[string]any{
					"type":        "array",
					"items":       map[string]any{"$ref": "#/components/schemas/ElevationWindow"},
					"description": "The windows keyed to this building whose key names no edge of it as it is now (`window.facade.moved`).",
				},
			},
		},
		"FacadeElevation": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"part", "ring", "edge", "from", "to", "length_m", "facing_deg", "courtyard", "windows"},
			"properties": map[string]any{
				"part":     map[string]any{"type": "integer"},
				"ring":     map[string]any{"type": "integer"},
				"edge":     map[string]any{"type": "integer"},
				"from":     pair,
				"to":       pair,
				"length_m": number,
				"facing_deg": map[string]any{
					"type":        "number",
					"description": "The bearing the facade faces, its outward normal, in degrees clockwise from grid north in `crs`, in [0, 360).",
				},
				"courtyard": map[string]any{"type": "boolean", "description": "An inner ring's edge."},
				"windows": map[string]any{
					"type":  "array",
					"items": map[string]any{"$ref": "#/components/schemas/ElevationWindow"},
				},
			},
		},
		"ElevationWindow": openapiElevationWindowSchema(number),
	}
}

// openapiElevationWindowSchema describes one window of an elevation, with its
// frame and the validation findings about it.
func openapiElevationWindowSchema(number map[string]any) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"id", "window", "sensitive", "review_status", "findings"},
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
			"window": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"u_m", "sill_z_m", "width_m", "height_m"},
				"description":          "The window's rectangle in its facade's frame, in metres.",
				"properties": map[string]any{
					"u_m": number, "sill_z_m": number, "width_m": number, "height_m": number,
				},
			},
			"sensitive":     map[string]any{"type": "boolean"},
			"review_status": map[string]any{"type": "string", "enum": []string{"proposed", "confirmed", "rejected"}},
			"findings": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"level", "code", "message"},
					"properties": map[string]any{
						"level":      map[string]any{"type": "string", "enum": []string{"error", "warning"}},
						"code":       map[string]any{"type": "string"},
						"feature_id": map[string]any{"type": "string"},
						"message":    map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}
