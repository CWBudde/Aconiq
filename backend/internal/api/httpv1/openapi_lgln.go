package httpv1

// openapiLGLNPathItems describes the LGLN LoD2 building import.
func openapiLGLNPathItems() map[string]any {
	return map[string]any{
		"/api/v1/import/lgln": map[string]any{
			"post": map[string]any{
				"summary": "Import LGLN LoD2 buildings for a WGS84 bounding box",
				"description": "Finds the LGLN Niedersachsen LoD2 CityGML tiles intersecting the box, " +
					"downloads them into .noise/cache/lgln (reused on the next request), and answers with " +
					"one building per CityGML Building or BuildingPart whose footprint centroid lies in the " +
					"box, in EPSG:4326. Nothing is saved to the model.",
				"operationId": "importLGLN",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"$ref": "#/components/schemas/ImportLGLNRequest",
							},
						},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "LGLN buildings as a GeoJSON feature collection",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/ImportLGLNResponse",
								},
							},
						},
					},
					"400": openapiErrorResponse("Invalid bounding box, a box outside Lower Saxony, or more tiles than one request may download (`lgln_too_many_tiles`)"),
					"405": methodNotAllowedResponse(),
					"502": openapiErrorResponse("The LGLN service could not be used (`lgln_unavailable`)"),
				},
			},
		},
	}
}

// openapiLGLNSchemas describes what the LGLN import takes and answers with.
func openapiLGLNSchemas() map[string]any {
	return map[string]any{
		"ImportLGLNRequest": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"south", "west", "north", "east"},
			"properties": map[string]any{
				"south": map[string]any{"type": "number"},
				"west":  map[string]any{"type": "number"},
				"north": map[string]any{"type": "number"},
				"east":  map[string]any{"type": "number"},
			},
		},
		"ImportLGLNResponse": map[string]any{
			"type":     "object",
			"required": []string{"type", "crs", "features", "tiles", "attribution", "skipped"},
			"properties": map[string]any{
				"type": map[string]any{"type": "string", "enum": []string{"FeatureCollection"}},
				"crs": map[string]any{
					"type":        "object",
					"description": "Named CRS of the feature coordinates; always `EPSG:4326` (longitude, latitude).",
					"required":    []string{"type", "properties"},
					"properties": map[string]any{
						"type": map[string]any{"type": "string", "enum": []string{"name"}},
						"properties": map[string]any{
							"type":     "object",
							"required": []string{"name"},
							"properties": map[string]any{
								"name": map[string]any{"type": "string", "enum": []string{"EPSG:4326"}},
							},
						},
					},
				},
				"features": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
				"tiles": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"required":             []string{"id", "date"},
						"properties": map[string]any{
							"id":   map[string]any{"type": "string"},
							"date": map[string]any{"type": "string", "format": "date"},
						},
					},
				},
				"attribution": map[string]any{
					"type":        "string",
					"description": "Source note the licence (CC BY 4.0) requires wherever the buildings are shown.",
				},
				"skipped": map[string]any{
					"type":                 "object",
					"additionalProperties": map[string]any{"type": "integer"},
					"description":          "Buildings the parser left out, counted by reason.",
				},
			},
		},
	}
}
