package httpv1

// openapiImportPathItems describes the import endpoints.
func openapiImportPathItems() map[string]any {
	return map[string]any{
		"/api/v1/import/osm": map[string]any{
			"post": map[string]any{
				"summary":     "Import OSM data for a WGS84 bounding box",
				"operationId": "importOSM",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"$ref": "#/components/schemas/ImportOSMRequest",
							},
						},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Imported GeoJSON feature collection",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
								},
							},
						},
					},
					"400": openapiErrorResponse("Invalid OSM import request"),
					"405": methodNotAllowedResponse(),
					"502": openapiErrorResponse("Overpass API request failed. When the server answered at all, details.upstream_status carries the status it sent."),
				},
			},
		},
		"/api/v1/import/terrain": map[string]any{
			"post": map[string]any{
				"summary":     "Import a GeoTIFF terrain model",
				"operationId": "importTerrain",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"multipart/form-data": map[string]any{
							"schema": map[string]any{
								"type":     "object",
								"required": []string{"file"},
								"properties": map[string]any{
									"file": map[string]any{
										"type":   "string",
										"format": "binary",
									},
								},
							},
						},
					},
				},
				"responses": map[string]any{
					"201": map[string]any{
						"description": "Imported terrain metadata",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/TerrainInfo",
								},
							},
						},
					},
					"400": openapiErrorResponse("Invalid terrain import request"),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Failed to persist terrain artifact"),
				},
			},
		},
	}
}
